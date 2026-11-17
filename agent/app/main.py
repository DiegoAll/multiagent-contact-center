"""API HTTP del agente (FastAPI). Consumida por el backend Go; no se expone al público."""

from __future__ import annotations

import asyncio
import hashlib
import hmac
import logging
import uuid
from contextlib import asynccontextmanager
from typing import Any

from fastapi import FastAPI, Header, HTTPException
from langchain_core.messages import HumanMessage
from langgraph.checkpoint.memory import InMemorySaver

from app.config import Settings, get_settings
from app.graph import Dependencies, build_graph
from app.llm import get_chat_model, get_embeddings
from app.rag import SyncStoreRetriever, build_vector_store, ingest
from app.schemas import ChatRequest, ChatResponse
from app.tools import ToolProvider

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
log = logging.getLogger("agent")


def _build_real_graph(settings: Settings):
    embeddings = get_embeddings(settings)
    store = build_vector_store(settings, embeddings)
    if settings.ingest_on_startup:
        try:
            ingest(store, settings.knowledge_dir)
        except Exception:  # noqa: BLE001
            log.exception("rag_ingest_failed")
    deps = Dependencies(
        llm=get_chat_model(settings),
        retriever=SyncStoreRetriever(store=store, k=settings.rag_top_k),
        get_tools=ToolProvider(settings).get_tools,
    )
    # InMemorySaver: memoria por sesión en un solo proceso. En producción, usar
    # PostgresSaver (langgraph-checkpoint-postgres) para compartirla entre réplicas.
    return build_graph(deps, settings, checkpointer=InMemorySaver())


def create_app(settings: Settings | None = None, graph: Any | None = None) -> FastAPI:
    settings = settings or get_settings()

    @asynccontextmanager
    async def lifespan(app: FastAPI):
        app.state.graph = graph if graph is not None else _build_real_graph(settings)
        yield

    app = FastAPI(title="Atento Agent Service", version="1.0.0", lifespan=lifespan)

    def verify_token(token: str | None) -> None:
        if settings.internal_token is None:
            return
        expected = settings.internal_token.get_secret_value()
        if not token or not hmac.compare_digest(token, expected):
            raise HTTPException(status_code=401, detail="unauthorized")

    @app.get("/healthz")
    async def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.post("/chat", response_model=ChatResponse)
    async def chat(
        req: ChatRequest, x_internal_token: str | None = Header(default=None)
    ) -> ChatResponse:
        verify_token(x_internal_token)

        session_id = req.session_id or uuid.uuid4().hex[:16]
        email = req.customer_email.strip().lower() if req.customer_email else None
        run_input = {
            "messages": [HumanMessage(content=req.message)],
            "customer_email": email,
            "customer_id": None,
            "route": "",
            "drafts": [],
            "agents_used": [],
            "sources": [],
            "tool_calls": [],
            "escalated": False,
            "blocked": False,
            "guard_reason": None,
            "reply": "",
        }
        run_config = {
            "configurable": {"thread_id": session_id},
            "run_name": "customer_support_chat",
            "tags": ["atento-lab"],
            # Metadatos para filtrar en LangSmith. Nunca se envía el email en claro.
            "metadata": {
                "session_id": session_id,
                "customer_hash": hashlib.sha256(email.encode()).hexdigest()[:12] if email else None,
            },
        }

        try:
            state = await asyncio.wait_for(
                app.state.graph.ainvoke(run_input, config=run_config),
                timeout=settings.request_timeout_seconds,
            )
        except TimeoutError as exc:
            raise HTTPException(status_code=504, detail="agent timeout") from exc
        except Exception as exc:  # noqa: BLE001
            log.exception("agent_failed")
            raise HTTPException(status_code=502, detail="agent error") from exc

        return ChatResponse(
            session_id=session_id,
            reply=state["reply"],
            escalated=bool(state.get("escalated")),
            agents_used=state.get("agents_used", []),
            sources=state.get("sources", []),
            tool_calls=state.get("tool_calls", []),
        )

    return app


app = create_app()
