import pytest
from langchain_core.vectorstores import InMemoryVectorStore

from app.llm import HashingEmbeddings
from app.rag import SyncStoreRetriever, ingest, load_and_split
from tests.conftest import KNOWLEDGE_DIR


def test_load_and_split_produces_stable_ids_and_sources():
    chunks, ids = load_and_split(KNOWLEDGE_DIR)
    assert chunks and len(chunks) == len(ids) == len(set(ids))
    assert all("source" in c.metadata for c in chunks)
    assert {c.metadata["source"] for c in chunks} >= {"envios.md", "devoluciones.md", "garantia.md"}
    # idempotencia: mismo contenido -> mismos ids
    assert load_and_split(KNOWLEDGE_DIR)[1] == ids


def test_ingest_is_idempotent():
    store = InMemoryVectorStore(HashingEmbeddings())
    first = ingest(store, KNOWLEDGE_DIR)
    ingest(store, KNOWLEDGE_DIR)
    assert len(store.store) == first


def test_retrieval_returns_the_relevant_source(retriever):
    cases = {
        "¿Cuánto tarda el envío a Bogotá?": "envios.md",
        "¿Cuántos días tengo para devolver un producto y cuándo me reembolsan?": "devoluciones.md",
        "¿Cuántos meses de garantía tienen los productos electrónicos?": "garantia.md",
        "¿Aceptan PSE o pago contra entrega?": "pagos.md",
    }
    for question, expected in cases.items():
        sources = [d.metadata["source"] for d in retriever.invoke(question)]
        assert expected in sources, f"{question!r} -> {sources}"


class SyncOnlyStore(InMemoryVectorStore):
    """Imita un PGVector en modo síncrono: la ruta asíncrona falla como en producción."""

    async def asimilarity_search(self, *args, **kwargs):
        raise AssertionError("_async_engine not found")


async def test_sync_store_retriever_works_when_the_async_path_is_broken():
    store = SyncOnlyStore(HashingEmbeddings())
    ingest(store, KNOWLEDGE_DIR)

    # Reproduce el fallo real: el retriever estándar usa la ruta asíncrona del store.
    with pytest.raises(AssertionError, match="_async_engine"):
        await store.as_retriever(search_kwargs={"k": 3}).ainvoke("¿Cuánto tarda el envío a Bogotá?")

    retriever = SyncStoreRetriever(store=store, k=3)
    docs = await retriever.ainvoke("¿Cuánto tarda el envío a Bogotá?")
    assert len(docs) == 3
    assert "envios.md" in {d.metadata["source"] for d in docs}


def test_sync_store_retriever_sync_path_and_k():
    store = InMemoryVectorStore(HashingEmbeddings())
    ingest(store, KNOWLEDGE_DIR)
    docs = SyncStoreRetriever(store=store, k=2).invoke("¿Cuántos meses de garantía hay?")
    assert len(docs) == 2
