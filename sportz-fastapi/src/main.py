"""
FastAPI application entry point.

Run with:
    uvicorn src.main:app --reload --host 0.0.0.0 --port 8000
"""
import uvicorn
from fastapi import FastAPI, WebSocket
from fastapi.middleware.cors import CORSMiddleware

from src.config import settings
from src.routes.commentary import router as commentary_router
from src.routes.matches import router as matches_router
from src.websocket.server import websocket_endpoint

# App
app = FastAPI(title="Sportz FastAPI", version="1.0.0")

# CORS  (mirrors `app.use(cors({ origin: VITE_FRONTEND_URL }))` in index.js)
app.add_middleware(
    CORSMiddleware,
    allow_origins=[settings.VITE_FRONTEND_URL],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Routes
app.include_router(matches_router)
app.include_router(commentary_router)

@app.get("/")
async def root():
    """Health-check endpoint — mirrors app.get('/', ...) in index.js."""
    return "Welcome to Sportz FastAPI!"

# WebSocket  (mirrors attachWebSocketServer in websocket/server.js)
@app.websocket("/websocket")
async def ws_route(ws: WebSocket):
    await websocket_endpoint(ws)

# Dev entry point
if __name__ == "__main__":
    uvicorn.run(
        "src.main:app",
        host=settings.HOST,
        port=settings.PORT,
        reload=True,
        log_level="info",
    )