"""
Match status helpers.
"""
from datetime import datetime, timezone
from typing import Callable, Coroutine, Optional

from src.database.models import MatchStatus

def get_match_status(
    start_time: Optional[datetime],
    end_time: Optional[datetime],
    now: Optional[datetime] = None,
) -> Optional[MatchStatus]:
    """
    Compute the status of a match based on its start/end times.
    """
    if start_time is None or end_time is None:
        return None

    if now is None:
        now = datetime.now(tz=timezone.utc)

    # Make naive datetimes UTC-aware for comparison
    def _aware(dt: datetime) -> datetime:
        if dt.tzinfo is None:
            return dt.replace(tzinfo=timezone.utc)
        return dt

    start = _aware(start_time)
    end = _aware(end_time)
    now = _aware(now)

    if now < start:
        return MatchStatus.scheduled

    if now >= end:
        return MatchStatus.finished

    return MatchStatus.live

async def sync_match_status(
    match,  # ORM Match instance
    update_status: Callable[[MatchStatus], Coroutine],
) -> MatchStatus:
    """
    Check whether the stored status is stale and update it if needed.

    Equivalent to the JS `syncMatchStatus(match, updateStatus)` function.
    """
    next_status = get_match_status(match.start_time, match.end_time)
    if next_status is None:
        return match.status

    if match.status != next_status:
        await update_status(next_status)
        match.status = next_status

    return match.status