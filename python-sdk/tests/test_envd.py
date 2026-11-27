from __future__ import annotations

from collections.abc import Mapping
from typing import Any

import httpx
import pytest

from devbox._transport import AsyncTransport, SyncTransport
from devbox.envd import AsyncEnvd, Envd
from devbox.models import OomKill


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
                    "cpu_possible": 16,
                    "cpu_target": 8,
                    "cpu_target_attempts": 3,
                    "cpu_write_pending_ms": 1500,
                    "cpu_used_pct": 8.25,
                    "mem_total": 1024,
                    "mem_used": 512,
                    "mem_cache": 128,
                    "disk_total": 2048,
                    "disk_used": 1024,
                    "oom_kills": [{"seq": 3, "process": "python3"}],
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
    assert metrics.cpu_possible == 16
    assert metrics.cpu_target == 8
    assert metrics.cpu_target_attempts == 3
    assert metrics.cpu_write_pending_ms == 1500
    assert metrics.cpu_used_pct == 8.25
    assert metrics.disk_used == 1024
    assert metrics.oom_kills == (OomKill(seq=3, process="python3"),)
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
        assert metrics.cpu_possible == 16
        assert metrics.cpu_target == 8
        assert metrics.cpu_target_attempts == 3
        assert metrics.cpu_write_pending_ms == 1500
        assert metrics.cpu_used_pct == 8.25
        assert metrics.oom_kills == (OomKill(seq=3, process="python3"),)
        assert (await envd.get_envs())["DEVBOX_ID"] == "sbx-1"
        assert (await envd.health()).envd_version == "1.2.3"
    finally:
        await transport.close()


def _metrics_payload() -> Mapping[str, Any]:
    return {
        "ts": 1763000000,
        "cpu_count": 2,
        "cpu_possible": 16,
        "cpu_target": 8,
        "cpu_target_attempts": 3,
        "cpu_write_pending_ms": 1500,
        "cpu_used_pct": 8.25,
        "mem_total": 1024,
        "mem_used": 512,
        "mem_cache": 128,
        "disk_total": 2048,
        "disk_used": 1024,
        "oom_kills": [{"seq": 3, "process": "python3"}],
    }


def test_envd_metrics_without_oom_kills() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.url.path == "/metrics"
        payload = dict(_metrics_payload())
        del payload["oom_kills"]
        return httpx.Response(200, json=payload)

    with _transport(handler) as transport:
        metrics = Envd(lambda: transport).get_metrics()

    assert metrics.oom_kills is None


def test_envd_metrics_without_cpu_hotplug_fields() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.url.path == "/metrics"
        payload = dict(_metrics_payload())
        for key in (
            "cpu_possible",
            "cpu_target",
            "cpu_target_attempts",
            "cpu_write_pending_ms",
        ):
            del payload[key]
        return httpx.Response(200, json=payload)

    with _transport(handler) as transport:
        metrics = Envd(lambda: transport).get_metrics()

    assert metrics.cpu_possible is None
    assert metrics.cpu_target is None
    assert metrics.cpu_target_attempts is None
    assert metrics.cpu_write_pending_ms is None


def _transport(handler: Any) -> SyncTransport:
    return SyncTransport(
        "https://envd.test",
        headers={"Cookie": "relay_token=connect-token", "E2B-Sandbox-Id": "sbx-1"},
        timeout=30,
        transport=httpx.MockTransport(handler),
    )
