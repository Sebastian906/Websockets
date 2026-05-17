"""
Pydantic validation schemas for matches.
"""
from datetime import datetime
from enum import Enum
from typing import Optional

from pydantic import BaseModel, Field, field_validator, model_validator

class MatchStatus(str, Enum):
    scheduled = "scheduled"
    live = "live"
    finished = "finished"

# Query params
class ListMatchesQuery(BaseModel):
    limit: Optional[int] = Field(default=None, ge=1, le=100)

class MatchIdParam(BaseModel):
    id: int = Field(ge=1)

# Request body
class CreateMatchBody(BaseModel):
    sport: str = Field(min_length=1)
    home_team: str = Field(min_length=1, alias="homeTeam")
    away_team: str = Field(min_length=1, alias="awayTeam")
    start_time: datetime = Field(alias="startTime")
    end_time: datetime = Field(alias="endTime")
    home_score: Optional[int] = Field(default=0, ge=0, alias="homeScore")
    away_score: Optional[int] = Field(default=0, ge=0, alias="awayScore")

    model_config = {"populate_by_name": True}

    @model_validator(mode="after")
    def end_after_start(self) -> "CreateMatchBody":
        if self.end_time <= self.start_time:
            raise ValueError("endTime must be chronologically after startTime")
        return self

class UpdateScoreBody(BaseModel):
    home_score: int = Field(ge=0, alias="homeScore")
    away_score: int = Field(ge=0, alias="awayScore")

    model_config = {"populate_by_name": True}

# Response body
class MatchOut(BaseModel):
    id: int
    sport: str
    home_team: str = Field(serialization_alias="homeTeam")
    away_team: str = Field(serialization_alias="awayTeam")
    status: str
    start_time: Optional[datetime] = Field(serialization_alias="startTime")
    end_time: Optional[datetime] = Field(serialization_alias="endTime")
    home_score: int = Field(serialization_alias="homeScore")
    away_score: int = Field(serialization_alias="awayScore")
    created_at: Optional[datetime] = Field(serialization_alias="createdAt")

    model_config = {"from_attributes": True, "populate_by_name": True}