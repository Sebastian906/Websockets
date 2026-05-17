"""
Alembic environment configuration.
Uses the same DATABASE_URL as the app and targets the SQLAlchemy metadata
defined in src/database/models.py.
"""
import asyncio
from logging.config import fileConfig

from alembic import context
from sqlalchemy import pool
from sqlalchemy.engine import Connection
from sqlalchemy.ext.asyncio import async_engine_from_config, create_async_engine
import ssl
from urllib.parse import urlparse, parse_qs, urlencode, urlunparse

# Pull settings before importing models so DATABASE_URL is available
from src.config import settings  # noqa: E402
from src.database.models import Base  # noqa: E402

config = context.config

# Inject the real DATABASE_URL (overrides the empty value in alembic.ini)
config.set_main_option("sqlalchemy.url", settings.DATABASE_URL)

if config.config_file_name is not None:
    fileConfig(config.config_file_name)

target_metadata = Base.metadata

# Offline migrations (generate SQL without a live DB)
def run_migrations_offline() -> None:
    url = config.get_main_option("sqlalchemy.url")
    context.configure(
        url=url,
        target_metadata=target_metadata,
        literal_binds=True,
        dialect_opts={"paramstyle": "named"},
    )
    with context.begin_transaction():
        context.run_migrations()

# Online migrations (run against a live DB)
def do_run_migrations(connection: Connection) -> None:
    context.configure(connection=connection, target_metadata=target_metadata)
    with context.begin_transaction():
        context.run_migrations()

async def run_async_migrations() -> None:
    # Sanitize the DATABASE_URL: remove sql-specific query params that
    # would be passed as kwargs to asyncpg.connect (e.g. `sslmode`) and
    # instead construct proper connect_args (SSLContext) when required.
    parsed = urlparse(settings.DATABASE_URL)
    qs = parse_qs(parsed.query)
    # Remove params that asyncpg.connect does not accept directly
    filtered_qs = {k: v for k, v in qs.items() if k not in ("sslmode", "channel_binding")}
    new_query = urlencode(filtered_qs, doseq=True)
    sanitized = urlunparse(parsed._replace(query=new_query))

    connect_args = {}
    sslmode = qs.get("sslmode", [None])[0]
    if sslmode and sslmode.lower() != "disable":
        ctx = ssl.create_default_context()
        ctx.check_hostname = True
        connect_args["ssl"] = ctx

    connectable = create_async_engine(
        sanitized,
        poolclass=pool.NullPool,
        connect_args=connect_args if connect_args else {},
    )
    async with connectable.connect() as connection:
        await connection.run_sync(do_run_migrations)
    await connectable.dispose()

def run_migrations_online() -> None:
    asyncio.run(run_async_migrations())

if context.is_offline_mode():
    run_migrations_offline()
else:
    run_migrations_online()