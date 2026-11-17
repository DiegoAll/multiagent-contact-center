"""Fábricas del modelo de chat y de embeddings (OpenAI / DeepSeek / hash offline)."""

import hashlib
import math
import re
import unicodedata

from langchain_core.embeddings import Embeddings
from langchain_core.language_models import BaseChatModel
from langchain_openai import ChatOpenAI, OpenAIEmbeddings

from app.config import Settings


def get_chat_model(s: Settings) -> BaseChatModel:
    """DeepSeek expone una API compatible con OpenAI: solo cambia base_url y modelo."""
    kwargs: dict = {"timeout": s.llm_timeout_seconds, "max_retries": 2}
    if s.llm_temperature is not None:
        kwargs["temperature"] = s.llm_temperature

    if s.llm_provider == "deepseek":
        return ChatOpenAI(
            model=s.deepseek_model,
            api_key=s.deepseek_api_key,
            base_url=s.deepseek_base_url,
            **kwargs,
        )
    return ChatOpenAI(model=s.openai_model, api_key=s.openai_api_key, **kwargs)


class HashingEmbeddings(Embeddings):
    """Embeddings léxicos deterministas (bolsa de palabras con hashing).

    No son semánticos: sirven para correr la demo y los tests sin API key.
    En producción usa embeddings reales (OpenAI, Bedrock, etc.).
    """

    def __init__(self, dim: int = 512) -> None:
        self.dim = dim

    @staticmethod
    def _tokens(text: str) -> list[str]:
        text = unicodedata.normalize("NFKD", text.lower())
        text = "".join(c for c in text if not unicodedata.combining(c))
        return [t for t in re.split(r"[^a-z0-9]+", text) if len(t) >= 3]

    def _embed(self, text: str) -> list[float]:
        vec = [0.0] * self.dim
        for tok in self._tokens(text):
            idx = int(hashlib.md5(tok.encode(), usedforsecurity=False).hexdigest(), 16) % self.dim
            vec[idx] += 1.0
        norm = math.sqrt(sum(v * v for v in vec)) or 1.0
        return [v / norm for v in vec]

    def embed_documents(self, texts: list[str]) -> list[list[float]]:
        return [self._embed(t) for t in texts]

    def embed_query(self, text: str) -> list[float]:
        return self._embed(text)


def get_embeddings(s: Settings) -> Embeddings:
    if s.embeddings_provider == "hash":
        return HashingEmbeddings()
    return OpenAIEmbeddings(model=s.embeddings_model, api_key=s.openai_api_key)
