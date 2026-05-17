"""
/matches REST routes.
"""
from typing import List

from fastapi import APIRouter, Depends, HTTPException, Query, Request
from sqlalchemy import desc, select
from sqlalchemy.ext.asyncio import AsyncSession

from src.database.db import get_db
from src.database.models import Match
from src.schemas.matches import CreateMatchBody, ListMatchesQuery, MatchOut
from src.utils.match_status import get_match_status
from src.websocket.server import broadcast_match_created

router = APIRouter(prefix="/matches", tags=["matches"])

MAX_LIMIT = 100

def _match_to_dict(m: Match) -> dict:
    """Serialise an ORM Match to the camelCase shape the frontend expects."""
    return {
        "id": m.id,
        "sport": m.sport,
        "homeTeam": m.home_team,
        "awayTeam": m.away_team,
        "status": m.status.value if hasattr(m.status, "value") else m.status,
        "startTime": m.start_time.isoformat() if m.start_time else None,
        "endTime": m.end_time.isoformat() if m.end_time else None,
        "homeScore": m.home_score,
        "awayScore": m.away_score,
        "createdAt": m.created_at.isoformat() if m.created_at else None,
    }

# GET /matches
@router.get("/")
async def list_matches(
    limit: int = Query(default=50, ge=1, le=100),
    db: AsyncSession = Depends(get_db),
):
    """
    Return the most recently created matches (newest first).
    Mirrors the GET / handler in matches.js.
    """
    safe_limit = min(limit, MAX_LIMIT)
    result = await db.execute(
        select(Match).order_by(desc(Match.created_at)).limit(safe_limit)
    )
    rows = result.scalars().all()
    return {"data": [_match_to_dict(m) for m in rows]}

# POST /matches
@router.post("/", status_code=201)
async def create_match(
    body: CreateMatchBody,
    db: AsyncSession = Depends(get_db),
):
    """
    Create a new match and broadcast it to all WebSocket clients.
    Mirrors the POST / handler in matches.js.
    """
    status = get_match_status(body.start_time, body.end_time)

    match = Match(
        sport=body.sport,
        home_team=body.home_team,
        away_team=body.away_team,
        start_time=body.start_time,
        end_time=body.end_time,
        home_score=body.home_score or 0,
        away_score=body.away_score or 0,
        status=status,
    )
    db.add(match)
    await db.commit()
    await db.refresh(match)

    match_dict = _match_to_dict(match)
    await broadcast_match_created(match_dict)

    return {"data": match_dict}