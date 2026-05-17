# sportz-fastapi

Python/FastAPI port of `sportz-express`. Implements the **exact same** REST
and WebSocket API, using:

| JS (sportz-express) | Python (sportz-fastapi) |
|---|---|
| Express 5 | FastAPI 0.115 |
| `ws` library | FastAPI built-in WebSocket (Starlette) |
| Drizzle ORM | SQLAlchemy 2 (async) |
| `drizzle-kit` migrations | Alembic |
| Zod validation | Pydantic v2 |
| `dotenv` | pydantic-settings |
| Node.js seed script | `src/seed/seed.py` |

---

## Project structure

```
sportz-fastapi/
├── alembic/                    # Migration tooling (≈ drizzle/)
│   ├── env.py
│   ├── script.py.mako
│   └── versions/
│       └── 0001_initial_schema.py
├── alembic.ini                 # ≈ drizzle.config.js
├── requirements.txt
├── pyproject.toml
├── .env.example
└── src/
    ├── main.py                 # ≈ src/index.js
    ├── config.py               # centralised settings (pydantic-settings)
    ├── database/
    │   ├── db.py               # ≈ src/database/db.js
    │   └── models.py           # ≈ src/database/schema.js
    ├── schemas/
    │   ├── matches.py          # ≈ src/validation/matches.js
    │   └── commentary.py       # ≈ src/validation/commentary.js
    ├── routes/
    │   ├── matches.py          # ≈ src/routes/matches.js
    │   └── commentary.py       # ≈ src/routes/commentary.js
    ├── websocket/
    │   └── server.py           # ≈ src/websocket/server.js
    ├── utils/
    │   └── match_status.py     # ≈ src/utils/match-status.js
    ├── seed/
    │   └── seed.py             # ≈ src/seed/seed.js
    └── data/
        └── data.json           # same format as sportz-express
```

---

## Quick start

### 1. Create and activate a virtual environment

```bash
python -m venv .venv
source .venv/bin/activate   # Windows: .venv\Scripts\activate
```

### 2. Install dependencies

```bash
pip install -r requirements.txt
```

### 3. Configure environment

```bash
cp .env.example .env
# Edit .env and set DATABASE_URL, e.g.:
# DATABASE_URL=postgresql+asyncpg://postgres:password@localhost:5432/sportz
```

> **Note:** The DATABASE_URL uses the `postgresql+asyncpg://` scheme (not
> `postgresql://`) because the app uses SQLAlchemy's async engine.

### 4. Run migrations

```bash
# Apply the initial schema (creates matches + commentary tables)
alembic upgrade head
```

### 5. Start the server

```bash
# Development (with auto-reload)
uvicorn src.main:app --reload --host 0.0.0.0 --port 8000

# Or via the module entry point
python -m src.main
```

The server starts on `http://localhost:8000`, WebSocket on `ws://localhost:8000/websocket`.

### 6. Seed data (optional)

Copy your `data.json` into `src/data/` (same format as the Express version),
then:

```bash
API_URL=http://localhost:8000 python -m src.seed.seed
```

Additional seed variables:

| Variable | Default | Description |
|---|---|---|
| `DELAY_MS` | `250` | Delay between commentary inserts (ms) |
| `SEED_MATCH_DURATION_MINUTES` | `120` | Duration for auto-generated matches |
| `SEED_FORCE_LIVE` | `1` | Force matches into the live time window |

---

## API reference

Identical to `sportz-express`:

| Method | Path | Description |
|---|---|---|
| `GET` | `/` | Health check |
| `GET` | `/matches?limit=N` | List matches (newest first) |
| `POST` | `/matches` | Create a match |
| `GET` | `/matches/{id}/commentary?limit=N` | List commentary |
| `POST` | `/matches/{id}/commentary` | Add commentary |
| `WS` | `/websocket` | Real-time feed |

### WebSocket messages

**Client → Server**

```json
{ "type": "subscribe",   "matchId": 1 }
{ "type": "unsubscribe", "matchId": 1 }
```

**Server → Client**

```json
{ "type": "welcome" }
{ "type": "subscribed",   "matchId": 1 }
{ "type": "unsubscribed", "matchId": 1 }
{ "type": "match_created", "data": { ... } }
{ "type": "commentary",    "data": { ... } }
{ "type": "score_update",  "matchId": 1, "data": { "homeScore": 2, "awayScore": 1 } }
{ "type": "ping" }
{ "type": "error",  "message": "Invalid JSON" }
```

---

## Running both servers together

The FastAPI server uses port `8000` by default — same as Express.  To run
both simultaneously, change one of the ports:

```bash
# Express (default)
cd sportz-express && npm run dev          # port 8000

# FastAPI on a different port
PORT=8001 uvicorn src.main:app --reload  # port 8001
```

Then update `sportz-frontend/src/constants/constants.ts` to point at
whichever backend you want to test.