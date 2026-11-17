"""Contratos HTTP del servicio (coinciden con domain.ChatRequest/ChatResponse del backend Go)."""

from typing import Any

from pydantic import BaseModel, Field


class ChatRequest(BaseModel):
    session_id: str | None = Field(default=None, max_length=64)
    message: str = Field(min_length=1, max_length=2000)
    customer_email: str | None = Field(default=None, max_length=254)


class ToolCall(BaseModel):
    name: str
    args: dict[str, Any] = Field(default_factory=dict)


class ChatResponse(BaseModel):
    session_id: str
    reply: str
    escalated: bool = False
    agents_used: list[str] = Field(default_factory=list)
    sources: list[str] = Field(default_factory=list)
    tool_calls: list[ToolCall] = Field(default_factory=list)
