"""
Database seeder — populates matches and commentary via the REST API.

Usage:
    API_URL=http://localhost:8000 python -m src.seed.seed

Environment variables (same as the JS version):
    API_URL                      required
    DELAY_MS                     delay between commentary inserts (default 250)
    SEED_MATCH_DURATION_MINUTES  default match duration (default 120)
    SEED_FORCE_LIVE              force matches into the live window (default true)
"""
from __future__ import annotations

import asyncio
import json
import logging
import math
import os
import random
import re
import sys
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

import httpx

logging.basicConfig(level=logging.INFO, format="%(message)s")
log = logging.getLogger(__name__)

# Config 
DELAY_MS = int(os.getenv("DELAY_MS", "250"))
NEW_MATCH_DELAY_MIN_MS = 2_000
NEW_MATCH_DELAY_MAX_MS = 3_000
DEFAULT_MATCH_DURATION_MINUTES = int(os.getenv("SEED_MATCH_DURATION_MINUTES", "120"))
_force_live_raw = os.getenv("SEED_FORCE_LIVE", "1")
FORCE_LIVE = _force_live_raw not in ("0", "false", "False")
API_URL = os.getenv("API_URL", "")

if not API_URL:
    sys.exit("API_URL is required to seed via REST endpoints.")

DATA_FILE = Path(__file__).parent.parent / "data" / "data.json"

# Data loading
def _read_json(path: Path) -> Any:
    with path.open(encoding="utf-8") as fh:
        return json.load(fh)


def load_seed_data(path: Path = DATA_FILE) -> Tuple[List[dict], List[dict]]:
    parsed = _read_json(path)

    if isinstance(parsed, list):
        return parsed, []
    if isinstance(parsed, dict):
        if isinstance(parsed.get("commentary"), list):
            return parsed["commentary"], parsed.get("matches") or []
        if isinstance(parsed.get("feed"), list):
            return parsed["feed"], parsed.get("matches") or []

    raise ValueError("Seed data must be an array or contain a commentary/feed array.")

# HTTP helper with retry/backoff
async def fetch_with_retry(
    client: httpx.AsyncClient,
    method: str,
    url: str,
    attempts: int = 5,
    base_delay: float = 0.2,
    **kwargs: Any,
) -> httpx.Response:
    last_err: Optional[Exception] = None
    for i in range(attempts):
        try:
            resp = await client.request(method, url, **kwargs)
            if resp.is_success:
                return resp
            if resp.status_code >= 500 or resp.status_code == 429:
                last_err = RuntimeError(f"HTTP {resp.status_code} {resp.reason_phrase}")
            else:
                raise RuntimeError(f"HTTP {resp.status_code} {resp.reason_phrase}")
        except httpx.RequestError as exc:
            last_err = exc

        if i < attempts - 1:
            delay = min(5.0, base_delay * (2 ** i)) + random.random() * 0.1
            await asyncio.sleep(delay)

    raise last_err or RuntimeError("fetch_with_retry exhausted all attempts")

# API calls
async def fetch_matches(client: httpx.AsyncClient, limit: int = 100) -> List[dict]:
    resp = await fetch_with_retry(client, "GET", f"{API_URL}/matches?limit={limit}")
    payload = resp.json()
    return payload.get("data", []) if isinstance(payload.get("data"), list) else []

async def create_match(client: httpx.AsyncClient, seed_match: dict) -> dict:
    start_time, end_time = _build_match_times(seed_match)
    resp = await fetch_with_retry(
        client,
        "POST",
        f"{API_URL}/matches",
        json={
            "sport": seed_match["sport"],
            "homeTeam": seed_match["homeTeam"],
            "awayTeam": seed_match["awayTeam"],
            "startTime": start_time,
            "endTime": end_time,
            "homeScore": seed_match.get("homeScore", 0),
            "awayScore": seed_match.get("awayScore", 0),
        },
    )
    return resp.json()["data"]

async def insert_commentary(client: httpx.AsyncClient, match_id: int, entry: dict) -> dict:
    payload: Dict[str, Any] = {"message": entry.get("message", "Update")}
    for field in ("minute", "sequence", "period", "eventType", "actor", "team", "metadata", "tags"):
        val = entry.get(field)
        if val is not None:
            payload[field] = val

    resp = await fetch_with_retry(
        client,
        "POST",
        f"{API_URL}/matches/{match_id}/commentary",
        json=payload,
    )
    if not resp.is_success:
        raise RuntimeError(f"Failed to create commentary: {resp.status_code}")
    return resp.json()["data"]

# Time helpers
from datetime import datetime, timedelta, timezone

def _parse_date(value: Any) -> Optional[datetime]:
    if not value:
        return None
    try:
        dt = datetime.fromisoformat(str(value).replace("Z", "+00:00"))
        if dt.tzinfo is None:
            dt = dt.replace(tzinfo=timezone.utc)
        return dt
    except ValueError:
        return None

def _is_live_match(match: dict) -> bool:
    start = _parse_date(match.get("startTime"))
    end = _parse_date(match.get("endTime"))
    if not start or not end:
        return False
    now = datetime.now(tz=timezone.utc)
    return start <= now < end

def _build_match_times(seed_match: dict) -> Tuple[str, str]:
    now = datetime.now(tz=timezone.utc)
    duration = timedelta(minutes=DEFAULT_MATCH_DURATION_MINUTES)

    start = _parse_date(seed_match.get("startTime"))
    end = _parse_date(seed_match.get("endTime"))

    if not start and not end:
        start = now - timedelta(minutes=5)
        end = start + duration
    else:
        if start and not end:
            end = start + duration
        if not start and end:
            start = end - duration

    if FORCE_LIVE and start and end:
        if not (start <= now < end):
            start = now - timedelta(minutes=5)
            end = start + duration

    if not start or not end:
        raise ValueError("Seed match must include valid startTime and endTime.")

    return start.isoformat(), end.isoformat()

# Cricket ordering helpers  (mirrors the JS equivalents faithfully)
def _innings_rank(period: Any) -> int:
    if not period:
        return 0
    lower = str(period).lower()
    m = re.search(r"(\d+)(st|nd|rd|th)", lower)
    if m:
        return int(m.group(1)) or 0
    if "first" in lower:
        return 1
    if "second" in lower:
        return 2
    if "third" in lower:
        return 3
    if "fourth" in lower:
        return 4
    return 0

def _normalize_cricket_feed(entries: List[dict], match: dict) -> List[dict]:
    sorted_entries = sorted(
        entries,
        key=lambda e: (
            _innings_rank(e.get("period")),
            e.get("sequence") if isinstance(e.get("sequence"), int) else sys.maxsize,
            e.get("minute") if isinstance(e.get("minute"), int) else sys.maxsize,
        ),
    )

    grouped: Dict[int, List[dict]] = {}
    for entry in sorted_entries:
        key = _innings_rank(entry.get("period"))
        grouped.setdefault(key, []).append(entry)

    ordered: List[dict] = []
    for key in sorted(grouped):
        innings_entries = grouped[key]
        primary_team = next(
            (
                e["team"]
                for e in innings_entries
                if e.get("team") in (match.get("homeTeam"), match.get("awayTeam"))
            ),
            None,
        )
        secondary_team = (
            match.get("awayTeam") if primary_team == match.get("homeTeam") else match.get("homeTeam")
        )

        neutral = [e for e in innings_entries if not e.get("team") or e.get("team") == "neutral"]
        primary = [e for e in innings_entries if e.get("team") == primary_team]
        secondary = [e for e in innings_entries if e.get("team") == secondary_team]
        other = [
            e
            for e in innings_entries
            if e.get("team")
            and e.get("team") != "neutral"
            and e.get("team") != primary_team
            and e.get("team") != secondary_team
        ]
        ordered.extend(neutral + primary + secondary + other)

    return ordered

# Feed helpers
def _replace_trailing_team(message: Any, replacements: Dict[str, str]) -> Any:
    if not isinstance(message, str):
        return message
    m = re.search(r"\(([^)]+)\)\s*$", message)
    if not m:
        return message
    next_team = replacements.get(m.group(1))
    if not next_team:
        return message
    return re.sub(r"\([^)]+\)\s*$", f"({next_team})", message)

def _clone_commentary_entries(
    entries: List[dict], template_match: dict, target_match: dict
) -> List[dict]:
    replacements = {
        template_match["homeTeam"]: target_match["homeTeam"],
        template_match["awayTeam"]: target_match["awayTeam"],
    }
    result = []
    for entry in entries:
        cloned = {**entry, "matchId": target_match["id"]}
        if entry.get("team") == template_match["homeTeam"]:
            cloned["team"] = target_match["homeTeam"]
        elif entry.get("team") == template_match["awayTeam"]:
            cloned["team"] = target_match["awayTeam"]
        cloned["message"] = _replace_trailing_team(entry.get("message"), replacements)
        result.append(cloned)
    return result

def expand_feed_for_matches(feed: List[dict], seed_matches: List[dict]) -> List[dict]:
    if not seed_matches:
        return feed

    by_match_id: Dict[int, List[dict]] = {}
    for entry in feed:
        mid = entry.get("matchId")
        if isinstance(mid, int):
            by_match_id.setdefault(mid, []).append(entry)

    match_by_id = {m["id"]: m for m in seed_matches if isinstance(m.get("id"), int)}
    template_by_sport: Dict[str, dict] = {}
    for m in seed_matches:
        sport = m.get("sport")
        if sport not in template_by_sport and isinstance(m.get("id"), int) and m["id"] in by_match_id:
            template_by_sport[sport] = m

    expanded = list(feed)
    for m in seed_matches:
        if not isinstance(m.get("id"), int) or m["id"] in by_match_id:
            continue
        template = template_by_sport.get(m.get("sport"))
        if not template:
            continue
        expanded.extend(_clone_commentary_entries(by_match_id.get(template["id"], []), template, m))

    return expanded

def build_randomized_feed(feed: List[dict], match_map: dict) -> List[dict]:
    buckets: Dict[Any, List[dict]] = {}
    for entry in feed:
        key = entry.get("matchId") if isinstance(entry.get("matchId"), int) else None
        buckets.setdefault(key, []).append(entry)

    for match_id, entries in list(buckets.items()):
        if not isinstance(match_id, int):
            continue
        target = match_map.get(match_id)
        sport = (target or {}).get("match", {}).get("sport", "").lower()
        if sport == "cricket" and target:
            buckets[match_id] = _normalize_cricket_feed(entries, target["match"])

    match_ids = list(buckets.keys())
    randomized: List[dict] = []
    last_match_id = None

    while len(randomized) < len(feed):
        candidates = [mid for mid in match_ids if buckets.get(mid)]
        if not candidates:
            break

        selectable = candidates
        if last_match_id is not None and len(candidates) > 1:
            without_last = [mid for mid in candidates if mid != last_match_id]
            if without_last:
                selectable = without_last

        choice = random.choice(selectable)
        randomized.append(buckets[choice].pop(0))
        last_match_id = choice

    return randomized

# Main seeder
def _random_match_delay() -> float:
    span = NEW_MATCH_DELAY_MAX_MS - NEW_MATCH_DELAY_MIN_MS
    return (NEW_MATCH_DELAY_MIN_MS + random.randint(0, span)) / 1000

async def seed() -> None:
    log.info("Seeding via API: %s", API_URL)

    feed, seed_matches = load_seed_data()

    async with httpx.AsyncClient(timeout=30, follow_redirects=True) as client:
        matches_list = await fetch_matches(client)

        match_map: Dict[int, dict] = {}
        match_key_map: Dict[str, dict] = {}

        for match in matches_list:
            if FORCE_LIVE and not _is_live_match(match):
                continue
            key = f"{match['sport']}|{match['homeTeam']}|{match['awayTeam']}"
            match_key_map.setdefault(key, match)
            match_map[match["id"]] = {
                "match": match,
                "score": {"home": match.get("homeScore", 0), "away": match.get("awayScore", 0)},
                "fakeNext": random.choice(["home", "away"]),
            }

        for seed_match in seed_matches or []:
            key = f"{seed_match['sport']}|{seed_match['homeTeam']}|{seed_match['awayTeam']}"
            match = match_key_map.get(key)
            if not match or (FORCE_LIVE and not _is_live_match(match)):
                match = await create_match(client, seed_match)
                match_key_map[key] = match
                await asyncio.sleep(_random_match_delay())

            if isinstance(seed_match.get("id"), int):
                match_map[seed_match["id"]] = {
                    "match": match,
                    "score": {"home": match.get("homeScore", 0), "away": match.get("awayScore", 0)},
                    "fakeNext": random.choice(["home", "away"]),
                }
            match_map[match["id"]] = {
                "match": match,
                "score": {"home": match.get("homeScore", 0), "away": match.get("awayScore", 0)},
                "fakeNext": random.choice(["home", "away"]),
            }

        if not match_map:
            raise RuntimeError("No matches found or created in the database.")

        expanded_feed = expand_feed_for_matches(feed, seed_matches)
        randomized_feed = build_randomized_feed(expanded_feed, match_map)

        for entry in randomized_feed:
            target = match_map.get(entry.get("matchId"))
            if not target:
                log.warning("Skipping entry — matchId missing or not found: %s", entry.get("message"))
                continue

            match = target["match"]
            row = await insert_commentary(client, match["id"], entry)
            log.info("[Match %s] %s", match["id"], row["message"])

            if DELAY_MS > 0:
                await asyncio.sleep(DELAY_MS / 1000)

if __name__ == "__main__":
    asyncio.run(seed())