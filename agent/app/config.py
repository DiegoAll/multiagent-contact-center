"""Configuración del servicio del agente (variables de entorno / .env)."""

from functools import lru_cache
from typing import Literal

from pydantic import SecretStr
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    # --- LLM (OpenAI o DeepSeek, ambos vía API compatible con OpenAI) ---
    llm_provider: Literal["openai", "deepseek"] = "openai"
    openai_api_key: SecretStr | None = None
    openai_model: str = "gpt-4o-mini"  # ajusta al modelo vigente en tu cuenta
    deepseek_api_key: SecretStr | None = None
    deepseek_model: str = "deepseek-chat"
    deepseek_base_url: str = "https://api.deepseek.com"
    llm_temperature: float | None = 0.0  # None = no enviar (modelos de razonamiento)
    llm_timeout_seconds: float = 30.0

    # --- Embeddings / RAG ---
    # DeepSeek no ofrece embeddings: se usan los de OpenAI o "hash" (offline, solo demo/tests).
    embeddings_provider: Literal["openai", "hash"] = "openai"
    embeddings_model: str = "text-embedding-3-small"
    vector_database_url: str = "postgresql+psycopg://atento:atento@localhost:5432/atento"
    vector_collection: str = "atento_kb"
    knowledge_dir: str = "knowledge"
    ingest_on_startup: bool = True
    rag_top_k: int = 4

    # --- MCP (tools del CRM simulado) ---
    mcp_url: str = "http://localhost:8081/mcp"
    internal_token: SecretStr | None = None  # secreto compartido backend <-> agente <-> MCP

    # --- Límites y guardrails ---
    max_tool_steps: int = 6
    request_timeout_seconds: float = 90.0
    max_message_chars: int = 2000
    canary_token: str = "CANARY-7f3a91c2"  # si aparece en una respuesta, hubo fuga del prompt


@lru_cache
def get_settings() -> Settings:
    return Settings()
