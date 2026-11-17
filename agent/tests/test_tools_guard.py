import json

from app.tools import NOT_AUTHORIZED, guard_tools, parse_json, resolve_customer_id, to_text


async def test_disallowed_tools_are_filtered(backend):
    names = {t.name for t in guard_tools(backend.tools(), customer_id=1)}
    assert "delete_everything" not in names
    assert names == {
        "get_customer", "get_order_status", "list_customer_orders",
        "list_customer_tickets", "create_ticket", "escalate_to_human",
    }


async def test_customer_id_is_forced_to_the_authenticated_customer(backend):
    tools = {t.name: t for t in guard_tools(backend.tools(), customer_id=1)}

    await tools["list_customer_orders"].ainvoke({"customer_id": 2})  # el LLM intenta otro id
    await tools["create_ticket"].ainvoke({"customer_id": 2, "subject": "s", "description": "d"})
    await tools["get_customer"].ainvoke({"email": "carlos@example.com"})

    calls = dict((name, args) for name, args in backend.calls)
    assert calls["list_customer_orders"]["customer_id"] == 1
    assert calls["create_ticket"]["customer_id"] == 1
    assert calls["get_customer"] == {"email": None, "customer_id": 1}


async def test_order_ownership_is_enforced_on_the_result(backend):
    tools = {t.name: t for t in guard_tools(backend.tools(), customer_id=1)}

    own = await tools["get_order_status"].ainvoke({"order_id": 7})
    assert json.loads(own)["status"] == "shipped"

    foreign = await tools["get_order_status"].ainvoke({"order_id": 8})  # pedido de Carlos
    assert foreign == NOT_AUTHORIZED


async def test_escalation_only_for_own_tickets(backend):
    ana = {t.name: t for t in guard_tools(backend.tools(), customer_id=1)}
    assert await ana["escalate_to_human"].ainvoke({"ticket_id": 3, "reason": "x"}) == NOT_AUTHORIZED
    assert backend.tickets[3]["escalated"] is False

    carlos = {t.name: t for t in guard_tools(backend.tools(), customer_id=2)}
    await carlos["escalate_to_human"].ainvoke({"ticket_id": 3, "reason": "x"})
    assert backend.tickets[3]["escalated"] is True


async def test_resolve_customer_id(backend):
    tools = backend.tools()
    assert await resolve_customer_id(tools, "ana@example.com") == 1
    assert await resolve_customer_id(tools, "nadie@example.com") is None
    assert await resolve_customer_id([], "ana@example.com") is None


def test_to_text_and_parse_json_handle_mcp_content_blocks():
    assert to_text([{"type": "text", "text": '{"id": 1}'}]) == '{"id": 1}'
    assert parse_json([{"type": "text", "text": '{"id": 1}'}]) == {"id": 1}
    assert parse_json("no es json") is None
