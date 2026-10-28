from __future__ import annotations

from collections.abc import Mapping
from typing import Any

import httpx
import pytest

from devbox._transport import AsyncTransport, SyncTransport
from devbox.envd import AsyncEnvd, Envd


def test_envd_reads_metrics_envs_and_health() -> None:
    requests: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        requests.append(request)
        assert request.headers["Cookie"] == "relay_token=connect-token"
        assert request.headers["E2B-Sandbox-Id"] == "sbx-1"
        if request.url.path == "/metrics":
            return httpx.Response(
                200,
                json={
                    "ts": 1763000000,
                    "cpu_count": 2,
                    "cpu_used_pct": 8.25,
                    "mem_total": 1024,
                    "mem_used": 512,
                    "mem_cache": 128,
                    "disk_total": 2048,
                    "disk_used": 1024,
                },
            )
        if request.url.path == "/envs":
            return httpx.Response(200, json={"DEVBOX_ID": "sbx-1", "DEVBOX_TEMPLATE_ID": "tpl-1"})
        assert request.url.path == "/health"
        return httpx.Response(204, headers={"X-Envd-Version": "1.2.3"})

    with _transport(handler) as transport:
        envd = Envd(lambda: transport)
        metrics = envd.get_metrics()
        envs = envd.get_envs()
        health = envd.health()

    assert metrics.cpu_count == 2
    assert metrics.cpu_used_pct == 8.25
    assert metrics.disk_used == 1024
    assert envs == {"DEVBOX_ID": "sbx-1", "DEVBOX_TEMPLATE_ID": "tpl-1"}
    assert health.envd_version == "1.2.3"
    assert [request.method for request in requests] == ["GET", "GET", "GET"]


@pytest.mark.asyncio
async def test_async_envd_reads_metrics_envs_and_health() -> None:
    async def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/metrics":
            return httpx.Response(200, json=_metrics_payload())
        if request.url.path == "/envs":
            return httpx.Response(200, json={"DEVBOX_ID": "sbx-1"})
        assert request.url.path == "/health"
        return httpx.Response(204, headers={"X-Envd-Version": "1.2.3"})

    transport = AsyncTransport(
        "https://envd.test", headers={}, timeout=30, transport=httpx.MockTransport(handler)
    )

    async def provide() -> AsyncTransport:
        return transport

    try:
        envd = AsyncEnvd(provide)
        metrics = await envd.get_metrics()
        assert metrics.cpu_count == 2
        assert metrics.cpu_used_pct == 8.25
        assert (await envd.get_envs())["DEVBOX_ID"] == "sbx-1"
        assert (await envd.health()).envd_version == "1.2.3"
    finally:
        await transport.close()


def _metrics_payload() -> Mapping[str, Any]:
    return {
        "ts": 1763000000,
        "cpu_count": 2,
        "cpu_used_pct": 8.25,
        "mem_total": 1024,
        "mem_used": 512,
        "mem_cache": 128,
        "disk_total": 2048,
        "disk_used": 1024,
    }


def _transport(handler: Any) -> SyncTransport:
    return SyncTransport(
        "https://envd.test",
        headers={"Cookie": "relay_token=connect-token", "E2B-Sandbox-Id": "sbx-1"},
        timeout=30,
        transport=httpx.MockTransport(handler),
    )
