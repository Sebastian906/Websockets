"""
/matches/{id}/commentary REST routes.
"""
from fastapi import APIRouter, Depends, HTTPException, Query
from sqlalchemy import desc, select
from sqlalchemy.ext.asyncio import AsyncSession

from src.database.db import get_db
from src.database.models import Commentary
from src.schemas.commentary import CreateCommentaryBody
from src.websocket.server import broadcast_commentary

router = APIRouter(tags=["commentary"])

MAX_LIMIT = 100

def _commentary_to_dict(c: Commentary) -> dict:
    """Serialise a Commentary ORM row to camelCase for the frontend."""
    return {
        "id": c.id,
        "matchId": c.match_id,
        "minute": c.minute,
        "sequence": c.sequence,
        "period": c.period,
        "eventType": c.event_type,
        "actor": c.actor,
        "team": c.team,
        "message": c.message,
        "metadata": c.extra_metadata,
        "tags": c.tags,
        "createdAt": c.created_at.isoformat() if c.created_at else None,
    }

# GET /matches/{match_id}/commentary
@router.get("/matches/{match_id}/commentary")
async def list_commentary(
    match_id: int,
    limit: int = Query(default=10, ge=1, le=100),
    db: AsyncSession = Depends(get_db),
):
    """
    Return commentary entries for a match (newest first).
    Mirrors the GET / handler in commentary.js.
    """
    safe_limit = min(limit, MAX_LIMIT)
    result = await db.execute(
        select(Commentary)
        .where(Commentary.match_id == match_id)
        .order_by(desc(Commentary.created_at))
        .limit(safe_limit)
    )
    rows = result.scalars().all()
    return {"data": [_commentary_to_dict(c) for c in rows]}

# POST /matches/{match_id}/commentary
@router.post("/matches/{match_id}/commentary", status_code=201)
async def create_commentary(
    match_id: int,
    body: CreateCommentaryBody,
    db: AsyncSession = Depends(get_db),
):
    """
    Insert a new commentary entry and broadcast it to subscribed WS clients.
    Mirrors the POST / handler in commentary.js.
    """
    entry = Commentary(
        match_id=match_id,
        minute=body.minute,
        sequence=body.sequence,
        period=body.period,
        event_type=body.event_type,
        actor=body.actor,
        team=body.team,
        message=body.message,
        extra_metadata=body.metadata,
        tags=body.tags,
    )
    db.add(entry)
    await db.commit()
    await db.refresh(entry)

    entry_dict = _commentary_to_dict(entry)
    await broadcast_commentary(match_id, entry_dict)

    return {"data": entry_dict}