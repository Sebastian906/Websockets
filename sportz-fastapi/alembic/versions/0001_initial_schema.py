"""Initial schema — matches and commentary tables.

Revision ID: 0001
Revises:
Create Date: 2025-05-01 00:00:00.000000
"""
from typing import Sequence, Union

import sqlalchemy as sa
from alembic import op

revision: str = "0001"
down_revision: Union[str, None] = None
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None

def upgrade() -> None:
    # match_status enum (create only if it does not already exist)
    op.execute(
        """
        DO $$
        BEGIN
            IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'match_status') THEN
                CREATE TYPE match_status AS ENUM ('scheduled', 'live', 'finished');
            END IF;
        END
        $$;
        """
    )

    op.create_table(
        "matches",
        sa.Column("id", sa.Integer, primary_key=True, autoincrement=True),
        sa.Column("sport", sa.Text, nullable=False),
        sa.Column("home_team", sa.Text, nullable=False),
        sa.Column("away_team", sa.Text, nullable=False),
        sa.Column(
            "status",
            sa.Enum("scheduled", "live", "finished", name="match_status"),
            nullable=False,
            server_default="scheduled",
        ),
        sa.Column("start_time", sa.DateTime, nullable=True),
        sa.Column("end_time", sa.DateTime, nullable=True),
        sa.Column("home_score", sa.Integer, nullable=False, server_default="0"),
        sa.Column("away_score", sa.Integer, nullable=False, server_default="0"),
        sa.Column(
            "created_at",
            sa.DateTime,
            nullable=False,
            server_default=sa.text("now()"),
        ),
    )

    op.create_table(
        "commentary",
        sa.Column("id", sa.Integer, primary_key=True, autoincrement=True),
        sa.Column("match_id", sa.Integer, sa.ForeignKey("matches.id"), nullable=False),
        sa.Column("minute", sa.Integer, nullable=True),
        sa.Column("sequence", sa.Integer, nullable=True),
        sa.Column("period", sa.Text, nullable=True),
        sa.Column("event_type", sa.Text, nullable=True),
        sa.Column("actor", sa.Text, nullable=True),
        sa.Column("team", sa.Text, nullable=True),
        sa.Column("message", sa.Text, nullable=False),
        sa.Column("metadata", sa.JSON, nullable=True),
        sa.Column("tags", sa.ARRAY(sa.Text), nullable=True),
        sa.Column(
            "created_at",
            sa.DateTime,
            nullable=False,
            server_default=sa.text("now()"),
        ),
    )

def downgrade() -> None:
    op.drop_table("commentary")
    op.drop_table("matches")
    op.execute("DROP TYPE IF EXISTS match_status")