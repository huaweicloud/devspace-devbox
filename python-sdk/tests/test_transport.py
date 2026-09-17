from __future__ import annotations

from collections.abc import Callable
from unittest.mock import patch

import httpx
import pytest

import devbox._transport as transport_module
from devbox import ProtocolError, RequestTimeoutError, ServiceUnavailableError
from devbox._transport import AsyncTransport, SyncTransport
from devbox.config import gateway_verify_tls
from devbox.errors import transport_error


@pytest.mark.parametrize(
    ("source_type", "mapped_type"),
    [
        (httpx.ConnectError, ServiceUnavailableError),
        (httpx.ReadError, ServiceUnavailableError),
        (httpx.ConnectTimeout, RequestTimeoutError),
        (httpx.ReadTimeout, RequestTimeoutError),
    ],
)
def test_transport_error_identifies_category_without_raw_details(
    source_type: type[httpx.HTTPError], mapped_type: type[Exception]
) -> None:
    error = source_type("private-host?token=secret")
    mapped = transport_error(error)
    assert isinstance(mapped, mapped_type)
    assert source_type.__name__ in str(mapped)
    assert "private-host" not in str(mapped)
    assert "secret" not in str(mapped)


def test_transport_preserves_original_cause(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(transport_module, "_CONNECT_RETRY_DELAYS", ())
    source = httpx.ConnectError("certificate verification failed")

    def handler(request: httpx.Request) -> httpx.Response:
        raise source

    with _transport(handler) as transport, pytest.raises(ServiceUnavailableError) as caught:
        transport.request("GET", "/sandboxes")
    assert caught.value.__cause__ is source


@pytest.mark.parametrize("value", ["true", "1", "yes", "on", " TRUE "])
def test_gateway_tls_debug_opt_in(monkeypatch: pytest.MonkeyPatch, value: str) -> None:
    monkeypatch.setenv("DEVBOX_GATEWAY_SKIP_TLS_VERIFY", value)
    with pytest.warns(RuntimeWarning, match="disabled for debugging"):
        assert gateway_verify_tls() is False


@pytest.mark.parametrize("value", ["", "false", "0", "no", "off", "typo"])
def test_gateway_tls_enabled_by_default(monkeypatch: pytest.MonkeyPatch, value: str) -> None:
    monkeypatch.setenv("DEVBOX_GATEWAY_SKIP_TLS_VERIFY", value)
    monkeypatch.setenv("DEVBOX_SKIP_TLS_VERIFY", "true")
    assert gateway_verify_tls() is True


@pytest.mark.parametrize("verify", [True, False])
def test_sync_tls_is_explicit(monkeypatch: pytest.MonkeyPatch, verify: bool) -> None:
    monkeypatch.setenv("DEVBOX_GATEWAY_SKIP_TLS_VERIFY", "true")
    monkeypatch.setenv("DEVBOX_SKIP_TLS_VERIFY", "true")
    with patch.object(httpx, "Client") as constructor:
        SyncTransport("https://api.test", headers={}, timeout=30)
        assert constructor.call_args.kwargs["verify"] is True
        SyncTransport("https://runtime.test", headers={}, timeout=30, verify=verify)
        assert constructor.call_args.kwargs["verify"] is verify


@pytest.mark.parametrize("verify", [True, False])
def test_async_tls_is_explicit(monkeypatch: pytest.MonkeyPatch, verify: bool) -> None:
    monkeypatch.setenv("DEVBOX_GATEWAY_SKIP_TLS_VERIFY", "true")
    monkeypatch.setenv("DEVBOX_SKIP_TLS_VERIFY", "true")
    with patch.object(httpx, "AsyncClient") as constructor:
        AsyncTransport("https://api.test", headers={}, timeout=30)
        assert constructor.call_args.kwargs["verify"] is True
        AsyncTransport("https://runtime.test", headers={}, timeout=30, verify=verify)
        assert constructor.call_args.kwargs["verify"] is verify


def test_retries_connection_establishment_only(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(transport_module, "_CONNECT_RETRY_DELAYS", (0.0, 0.0))
    attempts = 0

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal attempts
        attempts += 1
        if attempts < 3:
            raise httpx.ConnectError("connection refused", request=request)
        return httpx.Response(200, json={"status": "ready"})

    with _transport(handler) as transport:
        assert transport.request("POST", "/sandboxes", json_body={}) == {"status": "ready"}

    assert attempts == 3


def test_does_not_retry_server_errors() -> None:
    attempts = 0

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal attempts
        attempts += 1
        return httpx.Response(503, json={"message": "unavailable"})

    with _transport(handler) as transport, pytest.raises(ServiceUnavailableError):
        transport.request("GET", "/sandboxes")

    assert attempts == 1


def test_does_not_hide_programming_errors() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        raise RuntimeError("broken callback")

    with _transport(handler) as transport, pytest.raises(RuntimeError, match="broken callback"):
        transport.request("GET", "/sandboxes")


def test_rejects_redirects() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(302, headers={"Location": "https://login.test"})

    with (
        _transport(handler) as transport,
        pytest.raises(ProtocolError, match="unexpected redirect"),
    ):
        transport.request("GET", "/sandboxes")


def test_connect_stream_rejects_redirects() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(302, headers={"Location": "https://login.test"})

    with (
        _transport(handler) as transport,
        pytest.raises(ProtocolError, match="unexpected redirect"),
    ):
        next(transport.connect_stream("/process.Process/Start", {}))


@pytest.mark.asyncio
async def test_async_retries_connection_establishment(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(transport_module, "_CONNECT_RETRY_DELAYS", (0.0,))
    attempts = 0

    async def handler(request: httpx.Request) -> httpx.Response:
        nonlocal attempts
        attempts += 1
        if attempts == 1:
            raise httpx.ConnectError("connection refused", request=request)
        return httpx.Response(200, json={"status": "ready"})

    transport = AsyncTransport(
        "https://api.test",
        headers={},
        timeout=30,
        transport=httpx.MockTransport(handler),
    )
    try:
        assert await transport.request("POST", "/sandboxes", json_body={}) == {"status": "ready"}
    finally:
        await transport.close()

    assert attempts == 2


def _transport(
    handler: Callable[[httpx.Request], httpx.Response],
) -> SyncTransport:
    return SyncTransport(
        "https://api.test",
        headers={},
        timeout=30,
        transport=httpx.MockTransport(handler),
    )
