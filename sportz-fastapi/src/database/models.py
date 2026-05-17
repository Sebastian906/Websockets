"""
SQLAlchemy ORM table definitions.

Table layout is identical to the Drizzle migration
(drizzle/0000_parched_the_twelve.sql).
"""
import enum

from sqlalchemy import (
    ARRAY,
    JSON,
    BigInteger,
    Column,
    DateTime,
    Enum,
    ForeignKey,
    Integer,
    Text,
    func,
)
from sqlalchemy.orm import DeclarativeBase, relationship

class Base(DeclarativeBase):
    pass

class MatchStatus(str, enum.Enum):
    scheduled = "scheduled"
    live = "live"
    finished = "finished"

class Match(Base):
    """
    Mirrors the `matches` table in schema.js.
    """
    __tablename__ = "matches"

    id = Column(Integer, primary_key=True, autoincrement=True)
    sport = Column(Text, nullable=False)
    home_team = Column("home_team", Text, nullable=False)
    away_team = Column("away_team", Text, nullable=False)
    status = Column(
        Enum(MatchStatus, name="match_status"),
        nullable=False,
        default=MatchStatus.scheduled,
        server_default="scheduled",
    )
    start_time = Column("start_time", DateTime(timezone=False), nullable=True)
    end_time = Column("end_time", DateTime(timezone=False), nullable=True)
    home_score = Column("home_score", Integer, nullable=False, default=0, server_default="0")
    away_score = Column("away_score", Integer, nullable=False, default=0, server_default="0")
    created_at = Column(
        "created_at",
        DateTime(timezone=False),
        nullable=False,
        server_default=func.now(),
    )

    commentary = relationship("Commentary", back_populates="match", lazy="noload")

class Commentary(Base):
    """
    Mirrors the `commentary` table in schema.js.
    """
    __tablename__ = "commentary"

    id = Column(Integer, primary_key=True, autoincrement=True)
    match_id = Column(
        "match_id", Integer, ForeignKey("matches.id"), nullable=False
    )
    minute = Column(Integer, nullable=True)
    sequence = Column(Integer, nullable=True)
    period = Column(Text, nullable=True)
    event_type = Column("event_type", Text, nullable=True)
    actor = Column(Text, nullable=True)
    team = Column(Text, nullable=True)
    message = Column(Text, nullable=False)
    extra_metadata = Column("metadata", JSON, nullable=True)
    tags = Column(ARRAY(Text), nullable=True)
    created_at = Column(
        "created_at",
        DateTime(timezone=False),
        nullable=False,
        server_default=func.now(),
    )

    match = relationship("Match", back_populates="commentary", lazy="noload")