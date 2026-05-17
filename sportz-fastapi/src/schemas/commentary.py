"""
Pydantic validation schemas for commentary.
"""
from datetime import datetime
from typing import Any, Dict, List, Optional

from pydantic import BaseModel, Field

# Query params
class ListCommentaryQuery(BaseModel):
    limit: Optional[int] = Field(default=None, ge=1, le=100)

# Request body
class CreateCommentaryBody(BaseModel):
    minute: Optional[int] = Field(default=None, ge=0)
    sequence: Optional[int] = None
    period: Optional[str] = None
    event_type: Optional[str] = Field(default=None, alias="eventType")
    actor: Optional[str] = None
    team: Optional[str] = None
    message: str = Field(min_length=1)
    metadata: Optional[Dict[str, Any]] = None
    tags: Optional[List[str]] = None

    model_config = {"populate_by_name": True}

# Response body
class CommentaryOut(BaseModel):
    id: int
    match_id: int = Field(serialization_alias="matchId")
    minute: Optional[int] = None
    sequence: Optional[int] = None
    period: Optional[str] = None
    event_type: Optional[str] = Field(default=None, serialization_alias="eventType")
    actor: Optional[str] = None
    team: Optional[str] = None
    message: str
    metadata: Optional[Dict[str, Any]] = None
    tags: Optional[List[str]] = None
    created_at: Optional[datetime] = Field(default=None, serialization_alias="createdAt")

    model_config = {"from_attributes": True, "populate_by_name": True}