from pydantic_settings import BaseSettings, SettingsConfigDict

class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    DATABASE_URL: str
    PORT: int = 8000
    HOST: str = "0.0.0.0"
    VITE_FRONTEND_URL: str = "http://localhost:5173"

settings = Settings()  # type: ignore[call-arg]