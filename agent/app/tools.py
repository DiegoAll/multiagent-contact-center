"""Acceso a las tools MCP y capa de autorización sobre ellas.

El LLM NUNCA recibe las tools "crudas": se envuelven para imponer, en código,
que solo se opere sobre el cliente autenticado (mitiga OWASP LLM06 - Excessive
Agency - y BOLA/IDOR sobre la API). El prompt es una ayuda, no el control.
"""

from __future__ import annotations

import asyncio
import json
import logging
from collections.abc import Awaitable, Callable
from typing import Any

from langchain_core.tools import BaseTool, StructuredTool

from app.config import Settings

log = logging.getLogger(__name__)

ALLOWED_TOOLS = {
    "get_customer",
    "get_order_status",
    "list_customer_orders",
    "list_customer_tickets",
    "create_ticket",
    "escalate_to_human",
}
NOT_AUTHORIZED = "No autorizado: ese recurso no pertenece al cliente autenticado."


def to_text(result: Any) -> str:
    """Normaliza el resultado de una tool MCP (str o lista de bloques de contenido)."""
    if isinstance(result, str):
        return result
    if isinstance(result, list):
        parts = []
        for item in result:
            if isinstance(item, str):
                parts.append(item)
            elif isinstance(item, dict):
                parts.append(str(item.get("text", "")))
            else:
                parts.append(str(getattr(item, "text", item)))
        return "".join(parts)
    return str(result)


def parse_json(result: Any) -> Any:
    try:
        return json.loads(to_text(result))
    except (ValueError, TypeError):
        return None


def guard_tools(tools: list[BaseTool], customer_id: int) -> list[BaseTool]:
    """Devuelve las tools permitidas, envueltas con las reglas de autorización."""
    by_name = {t.name: t for t in tools if t.name in ALLOWED_TOOLS}

    async def owned_ticket_ids() -> set[int]:
        lister = by_name.get("list_customer_tickets")
        if lister is None:
            return set()
        data = parse_json(await lister.ainvoke({"customer_id": customer_id}))
        return {t["id"] for t in data if isinstance(t, dict) and "id" in t} if isinstance(
            data, list
        ) else set()

    def wrap(tool: BaseTool) -> BaseTool:
        async def run(**kwargs: Any) -> Any:
            args = dict(kwargs)
            name = tool.name

            if name == "get_customer":
                args = {"customer_id": customer_id}  # solo el propio cliente
            elif name in {"list_customer_orders", "list_customer_tickets", "create_ticket"}:
                args["customer_id"] = customer_id
            elif name == "escalate_to_human":
                try:
                    ticket_id = int(args.get("ticket_id", -1))
                except (TypeError, ValueError):
                    return NOT_AUTHORIZED
                if ticket_id not in await owned_ticket_ids():
                    log.warning("tool_denied", extra={"tool": name, "reason": "ticket_not_owned"})
                    return NOT_AUTHORIZED

            result = await tool.ainvoke(args)

            if name == "get_order_status":
                order = parse_json(result)
                if isinstance(order, dict) and order.get("customer_id") != customer_id:
                    log.warning("tool_denied", extra={"tool": name, "reason": "order_not_owned"})
                    return NOT_AUTHORIZED
            return result

        return StructuredTool.from_function(
            coroutine=run,
            name=tool.name,
            description=tool.description,
            args_schema=tool.args_schema,
        )

    return [wrap(t) for t in by_name.values()]


async def resolve_customer_id(tools: list[BaseTool], email: str) -> int | None:
    """Identifica al cliente por email usando la tool MCP get_customer."""
    tool = next((t for t in tools if t.name == "get_customer"), None)
    if tool is None:
        return None
    try:
        data = parse_json(await tool.ainvoke({"email": email}))
    except Exception:  # noqa: BLE001 - el error de la tool no debe tumbar la conversación
        log.exception("identify_failed")
        return None
    if isinstance(data, dict) and isinstance(data.get("id"), int):
        return data["id"]
    return None


class ToolProvider:
    """Carga perezosa (con reintentos) de las tools del servidor MCP."""

    def __init__(
        self,
        settings: Settings,
        loader: Callable[[Settings], Awaitable[list[BaseTool]]] | None = None,
    ) -> None:
        self._settings = settings
        self._loader = loader or load_mcp_tools
        self._tools: list[BaseTool] | None = None
        self._lock = asyncio.Lock()

    async def get_tools(self) -> list[BaseTool]:
        if self._tools is not None:
            return self._tools
        async with self._lock:
            if self._tools is None:
                self._tools = await self._loader(self._settings)
                log.info("mcp_tools_loaded", extra={"count": len(self._tools)})
        return self._tools


async def load_mcp_tools(settings: Settings) -> list[BaseTool]:
    from langchain_mcp_adapters.client import MultiServerMCPClient

    headers = {}
    if settings.internal_token is not None:
        headers["X-Internal-Token"] = settings.internal_token.get_secret_value()

    client = MultiServerMCPClient(
        {"crm": {"transport": "streamable_http", "url": settings.mcp_url, "headers": headers}}
    )
    last_exc: Exception | None = None
    for attempt in range(1, 6):
        try:
            return await client.get_tools()
        except Exception as exc:  # noqa: BLE001
            last_exc = exc
            log.warning("mcp_connect_retry", extra={"attempt": attempt})
            await asyncio.sleep(attempt)
    raise RuntimeError("no se pudo conectar con el servidor MCP") from last_exc
