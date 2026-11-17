"""Fixtures compartidas: LLM falso, tools MCP falsas y RAG en memoria (sin red ni API keys)."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import pytest
from langchain_core.language_models import BaseChatModel
from langchain_core.messages import AIMessage, HumanMessage, SystemMessage, ToolMessage
from langchain_core.outputs import ChatGeneration, ChatResult
from langchain_core.tools import StructuredTool
from langchain_core.vectorstores import InMemoryVectorStore
from pydantic import Field

from app.config import Settings
from app.graph import Dependencies
from app.llm import HashingEmbeddings
from app.rag import ingest

KNOWLEDGE_DIR = Path(__file__).resolve().parent.parent / "knowledge"


class FakeLLM(BaseChatModel):
    """Responde según el rol del prompt de sistema (supervisor / conocimiento / transacciones)."""

    route: str = "KNOWLEDGE"
    kb_answer: str = "Según nuestra política, el envío a Bogotá tarda 1 a 2 días hábiles."
    tool_calls: list[dict[str, Any]] = Field(default_factory=list)
    final_text: str = "Tu pedido va en camino."
    systems: list[str] = Field(default_factory=list)
    kb_inputs: list[str] = Field(default_factory=list)

    @property
    def _llm_type(self) -> str:
        return "fake"

    def bind_tools(self, tools: Any, **kwargs: Any) -> FakeLLM:
        return self

    def _generate(self, messages, stop=None, run_manager=None, **kwargs: Any) -> ChatResult:
        system = messages[0].content if messages and isinstance(messages[0], SystemMessage) else ""
        self.systems.append(str(system))
        if "SUPERVISOR" in system:
            msg = AIMessage(content=self.route)
        elif "CONOCIMIENTO" in system:
            self.kb_inputs.append(str(messages[-1].content))
            msg = AIMessage(content=self.kb_answer)
        elif "TRANSACCIONES" in system:
            if isinstance(messages[-1], ToolMessage) or not self.tool_calls:
                msg = AIMessage(content=self.final_text)
            else:
                msg = AIMessage(
                    content="",
                    tool_calls=[
                        {"name": c["name"], "args": c["args"], "id": f"call_{i}"}
                        for i, c in enumerate(self.tool_calls)
                    ],
                )
        else:
            msg = AIMessage(content="?")
        return ChatResult(generations=[ChatGeneration(message=msg)])


class FakeBackend:
    """CRM simulado en memoria expuesto como tools (misma forma que las tools MCP)."""

    def __init__(self) -> None:
        self.customers = {
            1: {"id": 1, "name": "Ana", "email": "ana@example.com", "segment": "premium"},
            2: {"id": 2, "name": "Carlos", "email": "carlos@example.com", "segment": "standard"},
        }
        self.orders = {
            7: {"id": 7, "customer_id": 1, "status": "shipped"},
            8: {"id": 8, "customer_id": 2, "status": "processing"},
        }
        self.tickets = {3: {"id": 3, "customer_id": 2, "status": "open", "escalated": False}}
        self.calls: list[tuple[str, dict[str, Any]]] = []

    def tools(self) -> list[StructuredTool]:
        def rec(name: str, args: dict[str, Any]) -> None:
            self.calls.append((name, args))

        async def get_customer(email: str | None = None, customer_id: int | None = None) -> str:
            rec("get_customer", {"email": email, "customer_id": customer_id})
            for c in self.customers.values():
                if c["email"] == email or c["id"] == customer_id:
                    return json.dumps(c)
            return "No se encontró el recurso solicitado."

        async def get_order_status(order_id: int) -> str:
            rec("get_order_status", {"order_id": order_id})
            return json.dumps(self.orders.get(order_id, {}))

        async def list_customer_orders(customer_id: int) -> str:
            rec("list_customer_orders", {"customer_id": customer_id})
            return json.dumps([o for o in self.orders.values() if o["customer_id"] == customer_id])

        async def list_customer_tickets(customer_id: int) -> str:
            rec("list_customer_tickets", {"customer_id": customer_id})
            return json.dumps([t for t in self.tickets.values() if t["customer_id"] == customer_id])

        async def create_ticket(
            customer_id: int, subject: str, description: str, priority: str = "medium"
        ) -> str:
            rec("create_ticket", {"customer_id": customer_id, "subject": subject})
            tid = max(self.tickets) + 1
            self.tickets[tid] = {"id": tid, "customer_id": customer_id, "status": "open", "escalated": False}
            return json.dumps(self.tickets[tid])

        async def escalate_to_human(ticket_id: int, reason: str) -> str:
            rec("escalate_to_human", {"ticket_id": ticket_id})
            self.tickets[ticket_id].update(escalated=True, status="escalated")
            return json.dumps(self.tickets[ticket_id])

        async def delete_everything() -> str:  # tool NO permitida: debe filtrarse
            rec("delete_everything", {})
            return "boom"

        funcs = [
            get_customer, get_order_status, list_customer_orders, list_customer_tickets,
            create_ticket, escalate_to_human, delete_everything,
        ]
        return [
            StructuredTool.from_function(coroutine=f, name=f.__name__, description=f.__name__)
            for f in funcs
        ]


@pytest.fixture
def settings() -> Settings:
    return Settings(
        embeddings_provider="hash",
        vector_database_url="memory",
        canary_token="CANARY-TEST-123",
        max_tool_steps=4,
        _env_file=None,
    )


@pytest.fixture
def retriever():
    store = InMemoryVectorStore(HashingEmbeddings())
    ingest(store, KNOWLEDGE_DIR)
    return store.as_retriever(search_kwargs={"k": 3})


@pytest.fixture
def backend() -> FakeBackend:
    return FakeBackend()


@pytest.fixture
def make_deps(retriever, backend):
    def _make(llm: BaseChatModel) -> Dependencies:
        tools = backend.tools()

        async def get_tools():
            return tools

        return Dependencies(llm=llm, retriever=retriever, get_tools=get_tools)

    return _make


def initial_state(message: str, email: str | None = None) -> dict[str, Any]:
    return {
        "messages": [HumanMessage(content=message)],
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
