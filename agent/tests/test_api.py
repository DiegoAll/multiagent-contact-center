import asyncio

import pytest
from fastapi.testclient import TestClient
from pydantic import SecretStr

from app.config import Settings
from app.main import create_app


class FakeGraph:
    def __init__(self, result=None, exc=None, delay=0.0):
        self.result, self.exc, self.delay = result, exc, delay
        self.calls = []

    async def ainvoke(self, state, config=None):
        self.calls.append((state, config))
        if self.delay:
            await asyncio.sleep(self.delay)
        if self.exc:
            raise self.exc
        return self.result


OK_RESULT = {
    "reply": "Hola",
    "escalated": True,
    "agents_used": ["supervisor"],
    "sources": ["envios.md"],
    "tool_calls": [{"name": "create_ticket", "args": {"customer_id": 1}}],
}


def client_for(graph, **settings_kw):
    settings = Settings(_env_file=None, **settings_kw)
    return TestClient(create_app(settings, graph=graph))


def test_chat_maps_graph_state_to_response():
    graph = FakeGraph(OK_RESULT)
    with client_for(graph) as c:
        r = c.post("/chat", json={"message": "hola", "session_id": "s1", "customer_email": " ANA@example.com "})
    assert r.status_code == 200
    body = r.json()
    assert body["reply"] == "Hola" and body["escalated"] is True
    assert body["session_id"] == "s1"
    assert body["tool_calls"][0]["name"] == "create_ticket"

    state, config = graph.calls[0]
    assert state["customer_email"] == "ana@example.com"
    assert config["configurable"]["thread_id"] == "s1"
    # el email nunca viaja en claro a la observabilidad (LangSmith)
    assert "ana@example.com" not in str(config["metadata"])
    assert config["metadata"]["customer_hash"]


def test_session_id_is_generated_when_missing():
    with client_for(FakeGraph(OK_RESULT)) as c:
        assert c.post("/chat", json={"message": "hola"}).json()["session_id"]


@pytest.mark.parametrize("body", [{}, {"message": ""}, {"message": "x" * 2001}])
def test_request_validation(body):
    with client_for(FakeGraph(OK_RESULT)) as c:
        assert c.post("/chat", json=body).status_code == 422


def test_internal_token_is_required_when_configured():
    graph = FakeGraph(OK_RESULT)
    with client_for(graph, internal_token=SecretStr("tok")) as c:
        assert c.post("/chat", json={"message": "hola"}).status_code == 401
        assert c.post("/chat", json={"message": "hola"}, headers={"X-Internal-Token": "bad"}).status_code == 401
        assert c.post("/chat", json={"message": "hola"}, headers={"X-Internal-Token": "tok"}).status_code == 200
        assert c.get("/healthz").status_code == 200  # la sonda no exige token


def test_graph_errors_do_not_leak_details():
    with client_for(FakeGraph(exc=RuntimeError("password=hunter2"))) as c:
        r = c.post("/chat", json={"message": "hola"})
    assert r.status_code == 502
    assert "hunter2" not in r.text


def test_timeout_returns_504():
    with client_for(FakeGraph(OK_RESULT, delay=0.5), request_timeout_seconds=0.05) as c:
        assert c.post("/chat", json={"message": "hola"}).status_code == 504
