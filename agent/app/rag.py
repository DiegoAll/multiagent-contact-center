"""RAG: ingesta de la base de conocimiento y construcción del retriever."""

from __future__ import annotations

import asyncio
import logging
from pathlib import Path

from langchain_core.callbacks import (
    AsyncCallbackManagerForRetrieverRun,
    CallbackManagerForRetrieverRun,
)
from langchain_core.documents import Document
from langchain_core.embeddings import Embeddings
from langchain_core.retrievers import BaseRetriever
from langchain_core.vectorstores import InMemoryVectorStore, VectorStore
from langchain_text_splitters import MarkdownHeaderTextSplitter, RecursiveCharacterTextSplitter

from app.config import Settings

log = logging.getLogger(__name__)

_HEADERS = [("#", "title"), ("##", "section")]


def load_and_split(knowledge_dir: str | Path) -> tuple[list[Document], list[str]]:
    """Lee los .md, los divide por encabezados y devuelve (chunks, ids deterministas).

    Los ids estables hacen la ingesta idempotente (upsert).
    """
    header_splitter = MarkdownHeaderTextSplitter(_HEADERS, strip_headers=False)
    char_splitter = RecursiveCharacterTextSplitter(chunk_size=700, chunk_overlap=80)

    chunks: list[Document] = []
    ids: list[str] = []
    for path in sorted(Path(knowledge_dir).glob("*.md")):
        text = path.read_text(encoding="utf-8")
        sections = header_splitter.split_text(text)
        for i, doc in enumerate(char_splitter.split_documents(sections)):
            doc.metadata["source"] = path.name
            chunks.append(doc)
            ids.append(f"{path.stem}:{i}")
    return chunks, ids


def build_vector_store(settings: Settings, embeddings: Embeddings) -> VectorStore:
    """PGVector en producción/compose; en memoria si se usa 'hash' sin base de datos."""
    if settings.embeddings_provider == "hash" and settings.vector_database_url == "memory":
        return InMemoryVectorStore(embeddings)

    from langchain_postgres import PGVector

    return PGVector(
        embeddings=embeddings,
        collection_name=settings.vector_collection,
        connection=settings.vector_database_url,
        use_jsonb=True,
    )


class SyncStoreRetriever(BaseRetriever):
    """Retriever que busca con la API *síncrona* del vector store, también desde async.

    `PGVector` se construye en modo síncrono (la ingesta usa `add_documents`). En ese
    modo su ruta asíncrona (`asimilarity_search`) falla con
    `AssertionError: _async_engine not found`, y `as_retriever().ainvoke` la usa. Aquí la
    búsqueda síncrona se ejecuta en un hilo para no bloquear el bucle de eventos.
    """

    store: VectorStore
    k: int = 4

    model_config = {"arbitrary_types_allowed": True}

    def _get_relevant_documents(
        self, query: str, *, run_manager: CallbackManagerForRetrieverRun
    ) -> list[Document]:
        return self.store.similarity_search(query, k=self.k)

    async def _aget_relevant_documents(
        self, query: str, *, run_manager: AsyncCallbackManagerForRetrieverRun
    ) -> list[Document]:
        return await asyncio.to_thread(self.store.similarity_search, query, k=self.k)


def ingest(store: VectorStore, knowledge_dir: str | Path) -> int:
    """Indexa la base de conocimiento. Devuelve el número de chunks."""
    chunks, ids = load_and_split(knowledge_dir)
    if chunks:
        store.add_documents(chunks, ids=ids)
    log.info("rag_ingested", extra={"chunks": len(chunks)})
    return len(chunks)
