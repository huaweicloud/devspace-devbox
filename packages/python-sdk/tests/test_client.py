from __future__ import annotations

import json
from collections.abc import Callable
from datetime import datetime, timezone

import httpx
import pytest

from devbox import (
    AsyncDevBox,
    ConfigurationError,
    ConflictError,
    DevBox,
    ProtocolError,
    RateLimitError,
    SandboxState,
)
from devbox.config import ConnectionConfig
from devbox.models import SandboxConnection
from devbox.sandbox import _gateway_url


def test_create_uses_manager_contract() -> None:
    captured: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        captured.append(request)
        return httpx.Response(201, json=connection_response())

    with client(handler) as api:
        sandbox = api.sandboxes.create(
            "python",
            timeout=300,
            envs={"A": "1"},
            metadata={"job": "test"},
        )

    body = json.loads(captured[0].content)
    assert captured[0].url.path == "/sandboxes"
    assert captured[0].headers["X-API-Key"] == "secret"
    assert body["templateID"] == "python"
    assert body["envVars"] == {"A": "1"}
    assert "autoPause" not in body
    assert "autoResume" not in body
    assert "network" not in body
    assert "allow_internet_access" not in body
    assert sandbox.sandbox_id == "sbx_123"
    assert sandbox._connection.tunnel_id == "aaaadysa"
    assert sandbox._connection.connect_token == "connect-token"
    assert sandbox._connection.token_lifetime == 86400
    assert sandbox._connection.token_expiration == 1789029315
    assert sandbox._connection.tunnel_lifetime == 86400
    assert sandbox._connection.tunnel_expiration == 1788946515
    assert "envd-token" not in repr(sandbox._connection)
    assert "connect-token" not in repr(sandbox._connection)


def test_missing_token_expiration_is_not_inferred_from_tunnel_expiration() -> None:
    connection = SandboxConnection.from_wire({"tunnelExpiration": 1788946515}, "sbx_123")
    assert connection.connect_token == ""
    assert connection.token_lifetime is None
    assert connection.token_expiration is None
    assert connection.tunnel_expiration == 1788946515


def test_connect_reads_latest_manager_connection() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.url.path == "/sandboxes/sbx_123/connect"
        assert json.loads(request.content) == {"timeout": 300}
        return httpx.Response(200, json=connection_response())

    with client(handler) as api:
        sandbox = api.sandboxes.connect("sbx_123")
        assert sandbox._connection.connect_token == "connect-token"
        assert sandbox._connection.token_expiration == 1789029315
        sandbox.close()


@pytest.mark.parametrize("code", ["already_killed", "another_conflict"])
def test_kill_handles_only_already_killed_conflict(code: str) -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        if request.method == "POST":
            return httpx.Response(201, json=connection_response())
        return httpx.Response(409, json={"error": code, "message": "conflict"})

    with client(handler) as api:
        sandbox = api.sandboxes.create()
        if code == "already_killed":
            assert sandbox.kill() is False
            assert sandbox.info.state is SandboxState.STOPPED
        else:
            with pytest.raises(ConflictError):
                sandbox.kill()


@pytest.mark.asyncio
async def test_async_kill_handles_expired_sandbox() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        if request.method == "POST":
            return httpx.Response(201, json=connection_response())
        return httpx.Response(409, json={"error": "already_killed", "message": "already killed"})

    async with AsyncDevBox(
        api_key="secret", api_url="https://api.test", http_transport=httpx.MockTransport(handler)
    ) as api:
        sandbox = await api.sandboxes.create()
        assert await sandbox.kill() is False
        assert sandbox.info.state is SandboxState.STOPPED


def test_v2_list_reads_pagination_headers() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.url.path == "/v2/sandboxes"
        assert request.url.params["state"] == "running,paused"
        return httpx.Response(
            200,
            json=[detail_response()],
            headers={"X-Next-Token": "next", "X-Total-Running": "7"},
        )

    with client(handler) as api:
        page = api.sandboxes.list(states=[SandboxState.RUNNING, SandboxState.PAUSED], limit=10)

    assert page.next_token == "next"
    assert page.total == 7
    assert page.items[0].sandbox_id == "sbx_123"


def test_lifecycle_uses_documented_paths_and_bodies() -> None:
    captured: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        captured.append(request)
        if request.url.path == "/sandboxes":
            return httpx.Response(201, json=connection_response())
        return httpx.Response(204)

    with client(handler) as api:
        sandbox = api.sandboxes.create()
        sandbox.refresh(120)

    assert [item.url.path for item in captured] == [
        "/sandboxes",
        "/sandboxes/sbx_123/refreshes",
    ]
    assert json.loads(captured[1].content) == {"duration": 120}


def test_logs_metrics_and_aggregate_metrics() -> None:
    metric = {
        "timestampUnix": 1,
        "cpuCount": 2,
        "cpuUsedPct": 25.5,
        "memUsed": 10,
        "memTotal": 20,
        "memCache": 1,
        "diskUsed": 30,
        "diskTotal": 40,
    }

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/sandboxes":
            return httpx.Response(201, json=connection_response())
        if request.url.path.endswith("/logs"):
            return httpx.Response(
                200,
                json={
                    "logs": [
                        {
                            "timestamp": "2026-09-02T00:00:00Z",
                            "level": "INFO",
                            "message": "ready",
                            "fields": {"source": "vm"},
                        }
                    ]
                },
            )
        if request.url.path == "/sandboxes/metrics":
            return httpx.Response(200, json={"sandboxes": {"sbx_123": metric}})
        return httpx.Response(200, json=[metric])

    with client(handler) as api:
        sandbox = api.sandboxes.create()
        logs = sandbox.get_logs(search="ready")
        metrics = sandbox.get_metrics(start=1, end=2)
        aggregate = api.sandboxes.metrics(["sbx_123"])

    assert logs[0].message == "ready"
    assert metrics[0].cpu_used_percent == 25.5
    assert aggregate["sbx_123"].disk_total_bytes == 40


def test_naive_metric_datetimes_are_interpreted_as_utc() -> None:
    captured: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        captured.append(request)
        if request.url.path == "/sandboxes":
            return httpx.Response(201, json=connection_response())
        return httpx.Response(200, json=[])

    start = datetime(2026, 1, 1)
    end = datetime(2026, 1, 1, 0, 5, tzinfo=timezone.utc)
    with client(handler) as api:
        api.sandboxes.create().get_metrics(start=start, end=end)

    params = captured[1].url.params
    assert params["start"] == str(int(start.replace(tzinfo=timezone.utc).timestamp()))
    assert params["end"] == str(int(end.timestamp()))


def test_error_shape_preserves_message_and_code() -> None:
    with (
        client(
            lambda request: httpx.Response(
                429, json={"error": "rate_limited", "message": "too many requests"}
            )
        ) as api,
        pytest.raises(RateLimitError) as raised,
    ):
        api.sandboxes.get("sbx_123")
    assert raised.value.code == "rate_limited"
    assert raised.value.message == "too many requests"


@pytest.mark.asyncio
async def test_async_client_uses_same_contract() -> None:
    async def handler(request: httpx.Request) -> httpx.Response:
        assert request.headers["X-API-Key"] == "secret"
        return httpx.Response(201, json=connection_response("sbx_async"))

    async with AsyncDevBox(
        api_key="secret", api_url="https://api.test", http_transport=httpx.MockTransport(handler)
    ) as api:
        sandbox = await api.sandboxes.create()
    assert sandbox.sandbox_id == "sbx_async"
    assert sandbox._connection.connect_token == "connect-token"
    assert sandbox._connection.token_expiration == 1789029315


@pytest.mark.asyncio
async def test_async_connect_reads_latest_manager_connection() -> None:
    async def handler(request: httpx.Request) -> httpx.Response:
        assert request.url.path == "/sandboxes/sbx_async/connect"
        return httpx.Response(200, json=connection_response("sbx_async"))

    async with AsyncDevBox(
        api_key="secret", api_url="https://api.test", http_transport=httpx.MockTransport(handler)
    ) as api:
        sandbox = await api.sandboxes.connect("sbx_async")
        assert sandbox._connection.connect_token == "connect-token"
        assert sandbox._connection.token_expiration == 1789029315
        await sandbox.close()


@pytest.mark.asyncio
async def test_async_sandbox_context_deletes_remote_sandbox() -> None:
    methods: list[str] = []

    async def handler(request: httpx.Request) -> httpx.Response:
        methods.append(request.method)
        if request.method == "POST":
            return httpx.Response(201, json=connection_response("sbx_async"))
        return httpx.Response(204)

    async with AsyncDevBox(
        api_key="secret",
        api_url="https://api.test",
        http_transport=httpx.MockTransport(handler),
    ) as api:
        async with await api.sandboxes.create() as sandbox:
            assert sandbox.sandbox_id == "sbx_async"
        assert sandbox.info.state is SandboxState.STOPPED

    assert methods == ["POST", "DELETE"]


def test_invalid_pagination_header_is_a_protocol_error() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, json=[], headers={"X-Total-Running": "invalid"})

    with client(handler) as api, pytest.raises(ProtocolError, match="X-Total-Running"):
        api.sandboxes.list()


def test_https_gateway_url_can_override_manager_placeholder(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv(
        "DEVBOX_GATEWAY_URL",
        "https://{tunnel_id}-{port}.gateway.example.test/",
    )
    config = ConnectionConfig.resolve(api_key="secret")
    connection = SandboxConnection(
        sandbox_id="sbx_123",
        domain="https://sbx_123.sandbox.devbox.local",
        envd_access_token="token",
        tunnel_id="aaaadysa",
    )

    assert (
        _gateway_url(connection, config.gateway_url)
        == "https://aaaadysa-49983.gateway.example.test"
    )


def test_gateway_url_override_requires_https(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("DEVBOX_GATEWAY_URL", "http://gateway.example.test")

    with pytest.raises(ConfigurationError, match="must use https"):
        ConnectionConfig.resolve(api_key="secret")


def test_sandbox_context_deletes_remote_sandbox() -> None:
    methods: list[str] = []

    def handler(request: httpx.Request) -> httpx.Response:
        methods.append(request.method)
        if request.method == "POST":
            return httpx.Response(201, json=connection_response())
        return httpx.Response(204)

    with client(handler) as api:
        with api.sandboxes.create() as sandbox:
            assert sandbox.sandbox_id == "sbx_123"
        assert sandbox.info.state is SandboxState.STOPPED

    assert methods == ["POST", "DELETE"]


def test_kill_is_idempotent() -> None:
    delete_count = 0

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal delete_count
        if request.method == "POST":
            return httpx.Response(201, json=connection_response())
        delete_count += 1
        if delete_count == 1:
            return httpx.Response(204)
        return httpx.Response(404, json={"code": "not_found", "message": "not found"})

    with client(handler) as api:
        sandbox = api.sandboxes.create()
        assert sandbox.kill() is True
        assert sandbox.kill() is False


def test_is_running_returns_false_when_sandbox_is_missing() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        if request.method == "POST":
            return httpx.Response(201, json=connection_response())
        return httpx.Response(404, json={"code": "not_found", "message": "not found"})

    with client(handler) as api:
        sandbox = api.sandboxes.create()
        assert sandbox.is_running() is False


def client(handler: Callable[[httpx.Request], httpx.Response]) -> DevBox:
    return DevBox(
        api_key="secret", api_url="https://api.test", http_transport=httpx.MockTransport(handler)
    )


def connection_response(sandbox_id: str = "sbx_123") -> dict[str, object]:
    return {
        "templateID": "base",
        "sandboxID": sandbox_id,
        "clientID": "client_1",
        "envdVersion": "1.0.0",
        "envdAccessToken": "envd-token",
        "domain": "sbx_123.sandbox.devbox.local",
        "sandboxProxyDomain": "devbox.example.test",
        "trafficAccessToken": "traffic-token",
        "tunnelId": "aaaadysa",
        "connectToken": "connect-token",
        "tokenLifetime": 86400,
        "tokenExpiration": 1789029315,
        "tunnelLifetime": 86400,
        "tunnelExpiration": 1788946515,
    }


def detail_response() -> dict[str, object]:
    return {
        **connection_response(),
        "startedAt": "2026-09-02T00:00:00Z",
        "endAt": "2026-09-02T00:05:00Z",
        "cpuCount": 2,
        "memoryMB": 512,
        "diskSizeMB": 10240,
        "state": "running",
    }
