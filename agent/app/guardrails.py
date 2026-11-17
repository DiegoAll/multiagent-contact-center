"""Guardrails de entrada y salida (defensa en capas, sin depender del LLM).

Mitigan, entre otros, OWASP LLM01 (prompt injection), LLM02 (divulgación de
información sensible), LLM05 (manejo inseguro de salidas) y LLM07 (fuga del
system prompt). Son una capa más: no sustituyen la autorización en las tools.
"""

from __future__ import annotations

import re
from dataclasses import dataclass

BLOCKED_REPLY = (
    "No puedo ayudarte con esa solicitud. Si necesitas ayuda con un pedido, "
    "un ticket o nuestras políticas, cuéntame y con gusto te apoyo."
)
FALLBACK_REPLY = (
    "Lo siento, no pude generar una respuesta confiable. ¿Quieres que te transfiera con un asesor?"
)

_INJECTION_PATTERNS = [
    r"ignor[ae]\s+(todas?\s+)?(las\s+|tus\s+)?(instrucciones|reglas)\s+(anteriores|previas)",
    r"olvida\s+(todas?\s+)?(las\s+|tus\s+)?(instrucciones|reglas)",
    r"ignore\s+(all\s+|any\s+)?(the\s+|your\s+)?(previous|prior|above)\s+(instructions|rules)",
    r"disregard\s+(all\s+|the\s+|your\s+)?(previous|prior|above)",
    r"(system|developer)\s+prompt",
    r"prompt\s+(del\s+)?sistema",
    r"instrucciones\s+(del\s+)?sistema",
    r"(reveal|show|print|repeat)\s+.{0,30}(instructions|prompt)",
    r"(revela|muestra|imprime|repite)\s+.{0,30}(instrucciones|prompt)",
    r"you\s+are\s+now\s+",
    r"ahora\s+(eres|act[uú]as)\s+",
    r"\bDAN\b",
    r"jailbreak",
    r"(modo\s+desarrollador|developer\s+mode)",
    r"<\s*/?\s*(system|assistant|tool)\s*>",
]
_INJECTION_RE = re.compile("|".join(f"(?:{p})" for p in _INJECTION_PATTERNS), re.IGNORECASE)

_CARD_RE = re.compile(r"\b(?:\d[ -]?){13,19}\b")
_SECRET_RE = re.compile(
    r"\b(?:sk-[A-Za-z0-9_\-]{16,}|AKIA[0-9A-Z]{16}|ghp_[A-Za-z0-9]{20,}|"
    r"Bearer\s+[A-Za-z0-9._\-]{20,})\b"
)
_EMAIL_RE = re.compile(r"[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}")

CARD_MASK = "[TARJETA REDACTADA]"
SECRET_MASK = "[SECRETO REDACTADO]"
EMAIL_MASK = "[EMAIL OCULTO]"


@dataclass(frozen=True)
class GuardResult:
    allowed: bool
    text: str
    reason: str | None = None


def _luhn_ok(digits: str) -> bool:
    total, parity = 0, len(digits) % 2
    for i, ch in enumerate(digits):
        d = int(ch)
        if i % 2 == parity:
            d *= 2
            if d > 9:
                d -= 9
        total += d
    return total % 10 == 0


def redact_cards(text: str) -> str:
    def repl(m: re.Match[str]) -> str:
        digits = re.sub(r"\D", "", m.group(0))
        return CARD_MASK if 13 <= len(digits) <= 19 and _luhn_ok(digits) else m.group(0)

    return _CARD_RE.sub(repl, text)


def redact_secrets(text: str) -> str:
    return _SECRET_RE.sub(SECRET_MASK, text)


def check_input(text: str, max_chars: int = 2000) -> GuardResult:
    """Valida y sanea el mensaje del usuario antes de que llegue a cualquier LLM."""
    if not text or not text.strip():
        return GuardResult(False, "", "empty_message")
    if len(text) > max_chars:
        return GuardResult(False, "", "message_too_long")
    if _INJECTION_RE.search(text):
        return GuardResult(False, "", "prompt_injection_suspected")
    return GuardResult(True, redact_secrets(redact_cards(text)).strip())


def check_output(text: str, allowed_emails: list[str] | None = None, canary: str = "") -> GuardResult:
    """Revisa la respuesta final: fuga del prompt, secretos, tarjetas y emails ajenos."""
    if canary and canary in text:
        return GuardResult(False, "", "system_prompt_leak")

    allowed = {e.lower() for e in (allowed_emails or [])}

    def mask_email(m: re.Match[str]) -> str:
        return m.group(0) if m.group(0).lower() in allowed else EMAIL_MASK

    clean = _EMAIL_RE.sub(mask_email, redact_secrets(redact_cards(text)))
    return GuardResult(True, clean.strip())
