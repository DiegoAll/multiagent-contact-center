"""Pruebas del ejecutor de evaluaciones y coherencia del dataset dorado con el código."""

from __future__ import annotations

import json

import httpx
import pytest

from app.graph import build_graph
from app.tools import ALLOWED_TOOLS
from evals import run_evals
from evals.metrics import (
    Check,
    evaluate,
    is_blocked,
    load_dataset,
    passed,
    percentile,
    summarize,
)
from tests.conftest import FakeLLM, initial_state

SEED_EMAILS = {None, "ana.gomez@example.com", "carlos.ruiz@example.com", "laura.mejia@example.com"}
KNOWN_AGENTS = {"guardrails", "supervisor", "knowledge", "transactions", "human_handoff", "quality"}
CATEGORIES = {"knowledge", "transactions", "both", "human", "adversarial"}
EXPECT_KEYS = {
    "agents_include",
    "tools_include",
    "tools_include_any",
    "tools_exclude",
    "max_tool_calls",
    "escalated",
    "must_include",
    "must_include_any",
    "must_not_include",
    "blocked",
}


# ------------------------------------------------------------------ dataset
def test_dataset_is_consistent_with_the_code():
    cases = load_dataset(run_evals.DEFAULT_DATASET)
    assert len(cases) >= 20
    for c in cases:
        assert c["category"] in CATEGORIES, c["id"]
        assert c.get("customer_email") in SEED_EMAILS, c["id"]
        assert 0 < len(c["message"]) <= 2000, c["id"]
        exp = c["expect"]
        assert set(exp) <= EXPECT_KEYS, (c["id"], set(exp) - EXPECT_KEYS)
        assert set(exp.get("agents_include", [])) <= KNOWN_AGENTS, c["id"]
        used_tools = set(exp.get("tools_include", [])) | set(exp.get("tools_include_any", []))
        used_tools |= set(exp.get("tools_exclude", []))
        assert used_tools <= ALLOWED_TOOLS, (c["id"], used_tools - ALLOWED_TOOLS)
    assert {c["category"] for c in cases} == CATEGORIES  # cubre todas las categorías


def test_load_dataset_rejects_bad_files(tmp_path):
    bad_json = tmp_path / "a.jsonl"
    bad_json.write_text("{no es json}\n", encoding="utf-8")
    with pytest.raises(ValueError, match="JSON inválido"):
        load_dataset(bad_json)

    missing = tmp_path / "b.jsonl"
    missing.write_text(json.dumps({"id": "x", "category": "knowledge"}) + "\n", encoding="utf-8")
    with pytest.raises(ValueError, match="falta el campo"):
        load_dataset(missing)

    dup = tmp_path / "c.jsonl"
    line = json.dumps({"id": "x", "category": "knowledge", "message": "m", "expect": {}})
    dup.write_text(f"{line}\n\n{line}\n", encoding="utf-8")
    with pytest.raises(ValueError, match="duplicados"):
        load_dataset(dup)


# ------------------------------------------------------------------ evaluate()
CASE = {
    "id": "t",
    "category": "transactions",
    "customer_email": "ana.gomez@example.com",
    "expect": {
        "agents_include": ["transactions"],
        "tools_include": ["list_customer_orders"],
        "tools_exclude": ["escalate_to_human"],
        "escalated": False,
        "must_include": ["enviado"],
        "must_include_any": ["audífonos", "auriculares"],
        "must_not_include": ["Monitor"],
        "blocked": False,
    },
}
GOOD = {
    "reply": "Tu pedido de audífonos está ENVIADO.",
    "agents_used": ["guardrails", "supervisor", "transactions", "quality"],
    "tool_calls": [{"name": "list_customer_orders", "args": {"customer_id": 1}}],
    "escalated": False,
}


def failed(checks: list[Check]) -> set[str]:
    return {c.name for c in checks if not c.passed}


def test_evaluate_passes_a_good_response_case_insensitively():
    checks = evaluate(CASE, 200, GOOD)
    assert passed(checks) and failed(checks) == set()


@pytest.mark.parametrize(
    ("patch", "expected_failure"),
    [
        (
            {"agents_used": ["guardrails", "supervisor", "knowledge", "quality"]},
            "agent:transactions",
        ),
        ({"tool_calls": []}, "tool+:list_customer_orders"),
        (
            {"tool_calls": [{"name": "list_customer_orders"}, {"name": "escalate_to_human"}]},
            "tool-:escalate_to_human",
        ),
        ({"escalated": True}, "escalated"),
        ({"reply": "Tu pedido de audífonos va en camino."}, "has:enviado"),
        ({"reply": "Enviado. Pedido de cables."}, "has-any"),
        ({"reply": "Enviado: audífonos y Monitor."}, "not:Monitor"),
        ({"agents_used": ["guardrails"]}, "blocked"),
        ({"reply": "Enviado: audífonos. Escríbenos a otra@example.com"}, "no_foreign_email"),
    ],
)
def test_evaluate_flags_each_kind_of_failure(patch, expected_failure):
    checks = evaluate(CASE, 200, {**GOOD, **patch})
    assert expected_failure in failed(checks)
    assert not passed(checks)


def test_evaluate_allows_the_customers_own_email_but_detects_the_canary():
    own = {**GOOD, "reply": "Enviado: audífonos para ana.gomez@example.com"}
    assert "no_foreign_email" not in failed(evaluate(CASE, 200, own))

    leak = {**GOOD, "reply": "Enviado: audífonos. CANARY-XYZ"}
    assert "no_canary_leak" in failed(evaluate(CASE, 200, leak, canary="CANARY-XYZ"))
    assert "no_canary_leak" not in failed(evaluate(CASE, 200, GOOD, canary="CANARY-XYZ"))


def test_evaluate_http_error_fails_the_whole_case():
    checks = evaluate(CASE, 502, None)
    assert [c.name for c in checks] == ["http_200"] and not passed(checks)


def test_tool_budget_and_any_tools():
    case = {
        "customer_email": None,
        "expect": {"max_tool_calls": 1, "tools_include_any": ["a", "b"]},
    }
    ok = {"reply": "", "agents_used": [], "tool_calls": [{"name": "b"}]}
    assert passed(evaluate(case, 200, ok))
    bad = {**ok, "tool_calls": [{"name": "b"}, {"name": "b"}]}
    assert "tool_budget" in failed(evaluate(case, 200, bad))
    assert "tool+any" in failed(evaluate(case, 200, {**ok, "tool_calls": []}))


def test_percentile_nearest_rank():
    assert percentile([], 95) is None
    assert percentile([5.0], 99) == 5.0
    data = [float(i) for i in range(1, 11)]  # 1..10
    assert percentile(data, 50) == 5.0
    assert percentile(data, 95) == 10.0
    assert percentile(data, 10) == 1.0


# ------------------------------------------------------------------ summarize()
def _run(ok: bool, status: int = 200, latency: float = 1.0, dim: str = "content"):
    return {"status": status, "latency_s": latency, "checks": [Check("c", dim, ok)]}


def test_summarize_pass_all_k_is_stricter_than_pass_rate():
    results = [
        {"id": "a", "category": "knowledge", "runs": [_run(True), _run(True), _run(True)]},
        {"id": "b", "category": "knowledge", "runs": [_run(True), _run(False), _run(True)]},
        {
            "id": "c",
            "category": "adversarial",
            "runs": [_run(True, dim="blocked"), _run(False, dim="blocked")],
        },
        {"id": "d", "category": "transactions", "runs": [_run(False, status=502, dim="http")]},
    ]
    s = summarize(results)
    o = s["overall"]
    assert o["cases"] == 4 and o["runs"] == 9
    assert o["pass_rate"] == round(6 / 9, 4)  # a: 3 + b: 2 + c: 1 + d: 0
    assert o["pass_all_k"] == 0.25  # solo "a" pasa en todas sus repeticiones
    assert o["error_rate"] == round(1 / 9, 4)
    assert s["by_category"]["knowledge"]["pass_rate"] == round(5 / 6, 4)
    assert s["attack_success_rate"] == 0.5
    assert set(o["accuracy"]) == {"content", "blocked", "http"}


def test_summarize_without_adversarial_cases_has_no_attack_rate():
    s = summarize([{"id": "a", "category": "knowledge", "runs": [_run(True)]}])
    assert "attack_success_rate" not in s
    assert s["overall"]["latency_s"]["p50"] == 1.0


# ------------------------------------------------ detección de bloqueo contra el grafo real
async def _run_graph(settings, make_deps, message):
    graph = build_graph(make_deps(FakeLLM(route="KNOWLEDGE")), settings)
    return await graph.ainvoke(initial_state(message), config={"configurable": {"thread_id": "t"}})


async def test_is_blocked_matches_what_the_real_graph_reports(settings, make_deps):
    blocked_cases = [
        c for c in load_dataset(run_evals.DEFAULT_DATASET) if c["expect"].get("blocked") is True
    ]
    assert blocked_cases
    for c in blocked_cases:
        state = await _run_graph(settings, make_deps, c["message"])
        assert is_blocked(state), c["id"]

    normal = await _run_graph(settings, make_deps, "¿Cuánto tarda el envío a Bogotá?")
    assert not is_blocked(normal)


async def test_expected_regex_gap_is_really_not_blocked(settings, make_deps):
    """ad-004 documenta una variante parafraseada que el guardrail por regex no detecta."""
    case = next(c for c in load_dataset(run_evals.DEFAULT_DATASET) if c["id"] == "ad-004")
    state = await _run_graph(settings, make_deps, case["message"])
    assert not is_blocked(state)
    assert "CANARY" not in state["reply"] and settings.canary_token not in state["reply"]


# ------------------------------------------------------------------ ejecutor (HTTP simulado)
def _mock_client_factory(handler):
    real = httpx.Client

    def factory(**kw):
        return real(transport=httpx.MockTransport(handler), **kw)

    return factory


def _write_dataset(path, cases):
    path.write_text(
        "\n".join(json.dumps(c, ensure_ascii=False) for c in cases) + "\n", encoding="utf-8"
    )


def test_runner_end_to_end_with_mocked_api(tmp_path, monkeypatch):
    dataset = tmp_path / "d.jsonl"
    _write_dataset(
        dataset,
        [
            {
                "id": "ok",
                "category": "knowledge",
                "message": "hola",
                "customer_email": None,
                "expect": {"must_include": ["hola"]},
            },
            {
                "id": "bad",
                "category": "adversarial",
                "message": "x",
                "customer_email": "ana.gomez@example.com",
                "expect": {"must_include": ["nunca aparece"]},
            },
        ],
    )
    seen = []

    def handler(request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content)
        seen.append((request.headers.get("x-api-key"), body))
        return httpx.Response(
            200,
            json={
                "session_id": body["session_id"],
                "reply": "hola mundo",
                "agents_used": ["supervisor"],
                "tool_calls": [],
                "escalated": False,
            },
        )

    monkeypatch.setattr(run_evals.httpx, "Client", _mock_client_factory(handler))
    out = tmp_path / "out" / "report.json"
    code = run_evals.main(
        [
            "--dataset",
            str(dataset),
            "--runs",
            "2",
            "--api-key",
            "k",
            "--out",
            str(out),
            "--canary",
            "",
        ]
    )

    assert code == 0
    report = json.loads(out.read_text(encoding="utf-8"))
    assert report["summary"]["overall"]["cases"] == 2 and report["summary"]["overall"]["runs"] == 4
    assert report["summary"]["overall"]["pass_all_k"] == 0.5
    assert report["summary"]["attack_success_rate"] == 1.0
    assert report["meta"]["runs_per_case"] == 2 and len(report["meta"]["dataset_sha256"]) == 16
    assert all(key == "k" for key, _ in seen)
    # sesión distinta por ejecución y el email solo se envía cuando existe
    assert len({b["session_id"] for _, b in seen}) == 4
    assert [("customer_email" in b) for _, b in seen] == [False, False, True, True]


def test_runner_fail_under_returns_nonzero(tmp_path, monkeypatch, capsys):
    dataset = tmp_path / "d.jsonl"
    _write_dataset(
        dataset,
        [
            {
                "id": "bad",
                "category": "knowledge",
                "message": "x",
                "expect": {"must_include": ["zzz"]},
            }
        ],
    )
    monkeypatch.setattr(
        run_evals.httpx,
        "Client",
        _mock_client_factory(
            lambda r: httpx.Response(200, json={"reply": "hola", "agents_used": []})
        ),
    )
    assert run_evals.main(["--dataset", str(dataset), "--runs", "1", "--fail-under", "0.9"]) == 1
    assert "umbral" in capsys.readouterr().err


def test_call_chat_retries_on_429_then_succeeds(monkeypatch):
    monkeypatch.setattr(run_evals.time, "sleep", lambda s: None)
    attempts = []

    def handler(request):
        attempts.append(1)
        if len(attempts) < 3:
            return httpx.Response(429, headers={"Retry-After": "1"}, json={"error": "rate"})
        return httpx.Response(200, json={"reply": "ok"})

    with httpx.Client(transport=httpx.MockTransport(handler), base_url="http://x") as client:
        status, data, latency, retries = run_evals.call_chat(client, {"message": "m"}, "s")
    assert (status, data, retries) == (200, {"reply": "ok"}, 2) and latency >= 0


def test_call_chat_gives_up_after_max_429_retries(monkeypatch):
    monkeypatch.setattr(run_evals.time, "sleep", lambda s: None)
    with httpx.Client(
        transport=httpx.MockTransport(lambda r: httpx.Response(429)), base_url="http://x"
    ) as client:
        status, data, _, retries = run_evals.call_chat(client, {"message": "m"}, "s")
    assert status == 429 and data is None and retries == run_evals.MAX_429_RETRIES


def test_call_chat_maps_timeouts_network_errors_and_bad_json():
    def timeout(request):
        raise httpx.ReadTimeout("t", request=request)

    def refused(request):
        raise httpx.ConnectError("c", request=request)

    def not_json(request):
        return httpx.Response(200, content=b"<html>")

    for handler, expected in [(timeout, 504), (refused, 0), (not_json, 502)]:
        with httpx.Client(transport=httpx.MockTransport(handler), base_url="http://x") as client:
            status, data, _, _ = run_evals.call_chat(client, {"message": "m"}, "s")
        assert status == expected and data is None


def test_runner_rejects_empty_selection(tmp_path):
    dataset = tmp_path / "d.jsonl"
    _write_dataset(dataset, [{"id": "a", "category": "knowledge", "message": "m", "expect": {}}])
    with pytest.raises(SystemExit):
        run_evals.main(["--dataset", str(dataset), "--category", "human"])
