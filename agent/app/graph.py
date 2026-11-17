"""Grafo multiagente (LangGraph): supervisor + especialistas + control de calidad.

    START -> input_guard -> identify -> supervisor
                 |                        |-- knowledge  -> knowledge_agent -> quality_agent
                 | (bloqueado)            |-- both       -> knowledge_agent -> transactions_agent -> quality_agent
                 v                        |-- transactions -> transactions_agent -> quality_agent
              finalize <- quality_agent   `-- human     -> human_handoff -> quality_agent
"""

from __future__ import annotations

import logging
import re
from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from typing import Annotated, Any, TypedDict

from langchain.agents import create_agent
from langchain_core.language_models import BaseChatModel
from langchain_core.messages import (
    AIMessage,
    AnyMessage,
    BaseMessage,
    HumanMessage,
    SystemMessage,
    ToolMessage,
)
from langchain_core.retrievers import BaseRetriever
from langchain_core.tools import BaseTool
from langgraph.errors import GraphRecursionError
from langgraph.graph import END, START, StateGraph
from langgraph.graph.message import add_messages

from app import prompts
from app.config import Settings
from app.guardrails import BLOCKED_REPLY, FALLBACK_REPLY, check_input, check_output
from app.tools import guard_tools, parse_json, resolve_customer_id

log = logging.getLogger(__name__)

HISTORY_WINDOW = 6
_HUMAN_RE = re.compile(
    r"\b(asesor|agente humano|persona real|humano|supervisor|hablar con (una )?persona)\b",
    re.IGNORECASE,
)
_ORDERS_RE = re.compile(r"\b(pedido|orden|ticket|caso|guía|guia|seguimiento)\b", re.IGNORECASE)


class AgentState(TypedDict, total=False):
    messages: Annotated[list[AnyMessage], add_messages]
    customer_email: str | None
    customer_id: int | None
    route: str
    drafts: list[str]
    agents_used: list[str]
    sources: list[str]
    tool_calls: list[dict[str, Any]]
    escalated: bool
    blocked: bool
    guard_reason: str | None
    reply: str


@dataclass
class Dependencies:
    llm: BaseChatModel
    retriever: BaseRetriever
    get_tools: Callable[[], Awaitable[list[BaseTool]]]


def text_of(content: Any) -> str:
    """Extrae texto de un contenido de mensaje (str o lista de bloques)."""
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        return "".join(
            b.get("text", "") if isinstance(b, dict) else str(b) for b in content
        )
    return str(content)


def chat_history(messages: list[AnyMessage], n: int = HISTORY_WINDOW) -> list[BaseMessage]:
    """Solo turnos de texto Human/AI (sin tool calls internos), últimos n."""
    turns: list[BaseMessage] = [
        m for m in messages
        if isinstance(m, HumanMessage) or (isinstance(m, AIMessage) and not m.tool_calls)
    ]
    return turns[-n:]


def parse_route(content: Any, user_text: str) -> str:
    match = re.search(r"\b(BOTH|TRANSACTIONS|KNOWLEDGE|HUMAN)\b", text_of(content).upper())
    if match:
        return match.group(1).lower()
    # Respaldo determinista si el LLM no respondió con una etiqueta válida.
    return "transactions" if _ORDERS_RE.search(user_text) else "knowledge"


def build_graph(deps: Dependencies, settings: Settings, checkpointer: Any | None = None):
    canary = settings.canary_token

    def last_user_text(state: AgentState) -> str:
        for m in reversed(state["messages"]):
            if isinstance(m, HumanMessage):
                return text_of(m.content)
        return ""

    # ------------------------------------------------------------------ guardrails
    async def input_guard(state: AgentState) -> dict[str, Any]:
        last = state["messages"][-1]
        original = text_of(last.content)
        res = check_input(original, max_chars=settings.max_message_chars)
        used = [*state.get("agents_used", []), "guardrails"]

        if not res.allowed:
            log.warning("input_blocked", extra={"reason": res.reason})
            return {
                "blocked": True,
                "guard_reason": res.reason,
                "reply": BLOCKED_REPLY,
                "agents_used": used,
                # el contenido malicioso no debe quedar en el historial
                "messages": [HumanMessage(content="[mensaje bloqueado]", id=last.id)],
            }
        out: dict[str, Any] = {"agents_used": used}
        if res.text != original:
            out["messages"] = [HumanMessage(content=res.text, id=last.id)]
        return out

    async def identify(state: AgentState) -> dict[str, Any]:
        email = state.get("customer_email")
        if not email:
            return {"customer_id": None}
        try:
            tools = await deps.get_tools()
        except Exception:  # noqa: BLE001
            log.exception("mcp_unavailable_for_identify")
            return {"customer_id": None}
        return {"customer_id": await resolve_customer_id(tools, email)}

    # ------------------------------------------------------------------ supervisor
    async def supervisor(state: AgentState) -> dict[str, Any]:
        text = last_user_text(state)
        if _HUMAN_RE.search(text):
            route = "human"  # red de seguridad determinista
        else:
            msgs = [
                SystemMessage(prompts.SUPERVISOR_PROMPT.format(canary=canary)),
                *chat_history(state["messages"], 4),
            ]
            resp = await deps.llm.ainvoke(msgs)
            route = parse_route(resp.content, text)
        return {"route": route, "agents_used": [*state.get("agents_used", []), "supervisor"]}

    def after_supervisor(state: AgentState) -> str:
        return {
            "knowledge": "knowledge_agent",
            "both": "knowledge_agent",
            "transactions": "transactions_agent",
            "human": "human_handoff",
        }[state["route"]]

    # ------------------------------------------------------------------ especialistas
    async def knowledge_agent(state: AgentState) -> dict[str, Any]:
        question = last_user_text(state)
        docs = await deps.retriever.ainvoke(question)
        context = "\n\n".join(
            f"[{d.metadata.get('source', '?')}]\n{d.page_content}" for d in docs
        ) or "(sin resultados)"

        history = chat_history(state["messages"])
        framed = HumanMessage(
            content=f"<contexto>\n{context}\n</contexto>\n\nPregunta del cliente: {question}"
        )
        msgs = [
            SystemMessage(prompts.KNOWLEDGE_PROMPT.format(canary=canary)),
            *history[:-1],
            framed,
        ]
        resp = await deps.llm.ainvoke(msgs)
        sources = sorted({d.metadata["source"] for d in docs if "source" in d.metadata})
        return {
            "drafts": [*state.get("drafts", []), text_of(resp.content).strip()],
            "sources": sources,
            "agents_used": [*state.get("agents_used", []), "knowledge"],
        }

    def after_knowledge(state: AgentState) -> str:
        return "transactions_agent" if state["route"] == "both" else "quality_agent"

    async def transactions_agent(state: AgentState) -> dict[str, Any]:
        used = [*state.get("agents_used", []), "transactions"]
        customer_id = state.get("customer_id")
        drafts = state.get("drafts", [])
        if customer_id is None:
            return {"drafts": [*drafts, prompts.NEEDS_IDENTITY_REPLY], "agents_used": used}

        try:
            tools = guard_tools(await deps.get_tools(), customer_id)
            agent = create_agent(
                deps.llm,
                tools,
                system_prompt=prompts.TRANSACTIONS_PROMPT.format(
                    customer_id=customer_id, canary=canary
                ),
            )
            result = await agent.ainvoke(
                {"messages": chat_history(state["messages"])},
                config={"recursion_limit": settings.max_tool_steps * 2 + 1},
            )
        except GraphRecursionError:
            log.warning("tool_loop_limit_reached")
            return {
                "drafts": [*drafts, "No logré completar la consulta. ¿Te transfiero con un asesor?"],
                "agents_used": used,
            }
        except Exception:  # noqa: BLE001
            log.exception("transactions_agent_failed")
            return {
                "drafts": [*drafts, "Tuve un problema consultando tu información. ¿Intentamos de nuevo?"],
                "agents_used": used,
            }

        calls, escalated = extract_tool_activity(result["messages"])
        final = next(
            (text_of(m.content) for m in reversed(result["messages"])
             if isinstance(m, AIMessage) and not m.tool_calls),
            "",
        )
        return {
            "drafts": [*drafts, final.strip()],
            "agents_used": used,
            "tool_calls": [*state.get("tool_calls", []), *calls],
            "escalated": state.get("escalated", False) or escalated,
        }

    async def human_handoff(state: AgentState) -> dict[str, Any]:
        used = [*state.get("agents_used", []), "human_handoff"]
        customer_id = state.get("customer_id")
        drafts = state.get("drafts", [])
        if customer_id is None:
            return {"drafts": [*drafts, prompts.HUMAN_NEEDS_IDENTITY_REPLY], "agents_used": used}

        try:
            tools = {t.name: t for t in guard_tools(await deps.get_tools(), customer_id)}
            create_args = {
                "customer_id": customer_id,
                "subject": "Solicitud de atención humana",
                "description": last_user_text(state)[:1000] or "El cliente pidió hablar con un asesor.",
                "priority": "high",
            }
            ticket = parse_json(await tools["create_ticket"].ainvoke(create_args))
            if not isinstance(ticket, dict) or "id" not in ticket:
                raise RuntimeError("ticket_not_created")
            esc_args = {"ticket_id": ticket["id"], "reason": "El cliente solicitó atención humana"}
            await tools["escalate_to_human"].ainvoke(esc_args)
        except Exception:  # noqa: BLE001
            log.exception("human_handoff_failed")
            return {
                "drafts": [*drafts, "No pude abrir tu caso en este momento. Intenta nuevamente en unos minutos."],
                "agents_used": used,
            }

        return {
            "drafts": [
                *drafts,
                f"Listo, escalé tu caso (ticket #{ticket['id']}) a un asesor humano. "
                "Te contactarán en horario de atención.",
            ],
            "agents_used": used,
            "tool_calls": [
                *state.get("tool_calls", []),
                {"name": "create_ticket", "args": create_args},
                {"name": "escalate_to_human", "args": esc_args},
            ],
            "escalated": True,
        }

    # ------------------------------------------------------------------ calidad / salida
    async def quality_agent(state: AgentState) -> dict[str, Any]:
        reply = "\n\n".join(d for d in state.get("drafts", []) if d).strip()
        email = state.get("customer_email")
        res = check_output(reply, allowed_emails=[email] if email else [], canary=canary)
        if not res.allowed:
            log.warning("output_blocked", extra={"reason": res.reason})
            reply = FALLBACK_REPLY
        else:
            reply = res.text or FALLBACK_REPLY
        return {"reply": reply, "agents_used": [*state.get("agents_used", []), "quality"]}

    async def finalize(state: AgentState) -> dict[str, Any]:
        return {"messages": [AIMessage(content=state["reply"])]}

    # ------------------------------------------------------------------ ensamblado
    g = StateGraph(AgentState)
    g.add_node("input_guard", input_guard)
    g.add_node("identify", identify)
    g.add_node("supervisor", supervisor)
    g.add_node("knowledge_agent", knowledge_agent)
    g.add_node("transactions_agent", transactions_agent)
    g.add_node("human_handoff", human_handoff)
    g.add_node("quality_agent", quality_agent)
    g.add_node("finalize", finalize)

    g.add_edge(START, "input_guard")
    g.add_conditional_edges(
        "input_guard",
        lambda s: "finalize" if s.get("blocked") else "identify",
        {"finalize": "finalize", "identify": "identify"},
    )
    g.add_edge("identify", "supervisor")
    g.add_conditional_edges(
        "supervisor",
        after_supervisor,
        {
            "knowledge_agent": "knowledge_agent",
            "transactions_agent": "transactions_agent",
            "human_handoff": "human_handoff",
        },
    )
    g.add_conditional_edges(
        "knowledge_agent",
        after_knowledge,
        {"transactions_agent": "transactions_agent", "quality_agent": "quality_agent"},
    )
    g.add_edge("transactions_agent", "quality_agent")
    g.add_edge("human_handoff", "quality_agent")
    g.add_edge("quality_agent", "finalize")
    g.add_edge("finalize", END)

    return g.compile(checkpointer=checkpointer)


def extract_tool_activity(messages: list[AnyMessage]) -> tuple[list[dict[str, Any]], bool]:
    """Devuelve las tools invocadas y si hubo un escalamiento exitoso."""
    calls: list[dict[str, Any]] = []
    escalated = False
    for m in messages:
        if isinstance(m, AIMessage):
            calls.extend({"name": c["name"], "args": c.get("args", {})} for c in m.tool_calls)
        elif isinstance(m, ToolMessage) and m.name == "escalate_to_human":
            body = text_of(m.content)
            escalated = escalated or (m.status != "error" and "No autorizado" not in body)
    return calls, escalated
