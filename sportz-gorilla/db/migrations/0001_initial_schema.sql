-- sportz-go/db/migrations/0001_initial_schema.sql
--
-- Identical schema to:
--   sportz-express/drizzle/0000_parched_the_twelve.sql
--   sportz-fastapi/alembic/versions/0001_initial_schema.py
--
-- Run with:
--   psql "$DATABASE_URL" -f db/migrations/0001_initial_schema.sql

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'match_status') THEN
        CREATE TYPE match_status AS ENUM ('scheduled', 'live', 'finished');
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS matches (
    id         SERIAL PRIMARY KEY,
    sport      TEXT         NOT NULL,
    home_team  TEXT         NOT NULL,
    away_team  TEXT         NOT NULL,
    status     match_status NOT NULL DEFAULT 'scheduled',
    start_time TIMESTAMP,
    end_time   TIMESTAMP,
    home_score INTEGER      NOT NULL DEFAULT 0,
    away_score INTEGER      NOT NULL DEFAULT 0,
    created_at TIMESTAMP    NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS commentary (
    id         SERIAL PRIMARY KEY,
    match_id   INTEGER      NOT NULL REFERENCES matches(id),
    minute     INTEGER,
    sequence   INTEGER,
    period     TEXT,
    event_type TEXT,
    actor      TEXT,
    team       TEXT,
    message    TEXT         NOT NULL,
    metadata   JSONB,
    tags       TEXT[],
    created_at TIMESTAMP    NOT NULL DEFAULT now()
);