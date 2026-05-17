"""
Async SQLAlchemy engine and session factory.
"""
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine
import ssl
from urllib.parse import urlparse, parse_qs, urlencode, urlunparse

from src.config import settings

# Sanitize DATABASE_URL and convert ssl-related query params into
# appropriate connect_args for asyncpg (asyncpg expects an `ssl` arg,
# not `sslmode`).
parsed = urlparse(settings.DATABASE_URL)
qs = parse_qs(parsed.query)
filtered_qs = {k: v for k, v in qs.items() if k not in ("sslmode", "channel_binding")}
new_query = urlencode(filtered_qs, doseq=True)
sanitized_db_url = urlunparse(parsed._replace(query=new_query))

connect_args = {}
sslmode = qs.get("sslmode", [None])[0]
if sslmode and sslmode.lower() != "disable":
    ctx = ssl.create_default_context()
    ctx.check_hostname = True
    connect_args["ssl"] = ctx

engine = create_async_engine(
    sanitized_db_url,
    pool_pre_ping=True,
    pool_size=10,
    max_overflow=20,
    connect_args=connect_args if connect_args else {},
)

AsyncSessionLocal = async_sessionmaker(
    bind=engine,
    class_=AsyncSession,
    expire_on_commit=False,
)

async def get_db() -> AsyncSession:  # type: ignore[misc]
    """FastAPI dependency that yields a database session."""
    async with AsyncSessionLocal() as session:
        yield session