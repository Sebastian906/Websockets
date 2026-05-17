"""
WebSocket hub with per-match subscription rooms and heartbeat.

FastAPI's built-in WebSocket support (via Starlette) is used instead of the
`ws` npm package.  The public interface (broadcast_match_created,
broadcast_commentary, broadcast_score_update) is identical to the JS version
so the routes can call the same helpers.
"""
import asyncio
import json
import logging
from collections import defaultdict
from typing import Any, Dict, Optional, Set

from fastapi import WebSocket, WebSocketDisconnect

logger = logging.getLogger(__name__)

# Internal state  (module-level singletons — same pattern as JS `matchSubscribers`)

# matchId (int) → set of connected WebSocket objects
_match_subscribers: Dict[int, Set[WebSocket]] = defaultdict(set)

# All connected sockets (for global broadcasts like match_created)
_all_clients: Set[WebSocket] = set()

# Per-socket subscription tracking  socket → set[matchId]
_socket_subscriptions: Dict[WebSocket, Set[int]] = defaultdict(set)

# Low-level helpers
async def _send_json(ws: WebSocket, payload: Any) -> None:
    """Send a JSON payload; silently drop if the socket is already gone."""
    try:
        await ws.send_text(json.dumps(payload, default=str))
    except Exception:
        pass  # Socket already closed — harmless

def _subscribe(match_id: int, ws: WebSocket) -> None:
    _match_subscribers[match_id].add(ws)
    _socket_subscriptions[ws].add(match_id)

def _unsubscribe(match_id: int, ws: WebSocket) -> None:
    _match_subscribers[match_id].discard(ws)
    if not _match_subscribers[match_id]:
        del _match_subscribers[match_id]
    _socket_subscriptions[ws].discard(match_id)

def _cleanup(ws: WebSocket) -> None:
    """Remove a socket from every subscription set it belongs to."""
    for match_id in list(_socket_subscriptions.get(ws, [])):
        _match_subscribers[match_id].discard(ws)
        if not _match_subscribers[match_id]:
            del _match_subscribers[match_id]
    _socket_subscriptions.pop(ws, None)
    _all_clients.discard(ws)

# Broadcast helpers (called by route handlers — public API)
async def broadcast_match_created(match: Any) -> None:
    """Send a match_created event to every connected client."""
    payload = {"type": "match_created", "data": match}
    for ws in list(_all_clients):
        await _send_json(ws, payload)

async def broadcast_commentary(match_id: int, comment: Any) -> None:
    """Send a commentary event to clients subscribed to match_id."""
    payload = {"type": "commentary", "data": comment}
    for ws in list(_match_subscribers.get(match_id, [])):
        await _send_json(ws, payload)

async def broadcast_score_update(match_id: int, home_score: int, away_score: int) -> None:
    """Send a score_update event to clients subscribed to match_id."""
    payload = {
        "type": "score_update",
        "matchId": match_id,
        "data": {"homeScore": home_score, "awayScore": away_score},
    }
    for ws in list(_match_subscribers.get(match_id, [])):
        await _send_json(ws, payload)

# Per-message handler  (mirrors handleMessage in server.js)
async def _handle_message(ws: WebSocket, raw: str) -> None:
    try:
        message = json.loads(raw)
    except json.JSONDecodeError:
        await _send_json(ws, {"type": "error", "message": "Invalid JSON"})
        return

    msg_type = message.get("type")
    match_id = message.get("matchId")

    if msg_type == "subscribe" and isinstance(match_id, int):
        _subscribe(match_id, ws)
        await _send_json(ws, {"type": "subscribed", "matchId": match_id})
        return

    if msg_type == "unsubscribe" and isinstance(match_id, int):
        _unsubscribe(match_id, ws)
        await _send_json(ws, {"type": "unsubscribed", "matchId": match_id})
        return

    # Unknown message — silently ignore (same behaviour as JS version)

# WebSocket endpoint handler  (called by the FastAPI router)
PING_INTERVAL = 30  # seconds — mirrors the setInterval(30 000) in server.js

async def websocket_endpoint(ws: WebSocket) -> None:
    """
    Accept a WebSocket connection, register it, and manage its lifecycle.

    Mount this coroutine on a FastAPI WebSocket route:

        @app.websocket("/websocket")
        async def ws_route(ws: WebSocket):
            await websocket_endpoint(ws)
    """
    await ws.accept()
    _all_clients.add(ws)
    await _send_json(ws, {"type": "welcome"})

    # Heartbeat task — mirrors the ping/pong interval in server.js
    async def _heartbeat() -> None:
        while True:
            await asyncio.sleep(PING_INTERVAL)
            try:
                await ws.send_text(json.dumps({"type": "ping"}))
            except Exception:
                break

    heartbeat_task = asyncio.create_task(_heartbeat())

    try:
        while True:
            data = await ws.receive_text()
            await _handle_message(ws, data)
    except WebSocketDisconnect:
        pass
    except Exception as exc:
        logger.warning("WebSocket error: %s", exc)
    finally:
        heartbeat_task.cancel()
        _cleanup(ws)