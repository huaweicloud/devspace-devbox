"""Client entry points for the DevBox Python SDK.

Both clients resolve the API key from the ``api_key`` argument or the
``DEVBOX_API_KEY`` / ``E2B_API_KEY`` environment variables, and expose the
``sandboxes`` and ``templates`` namespaces.
"""

from __future__ import annotations

from collections.abc import Mapping
from types import TracebackType

import httpx

from ._transport import AsyncTransport, SyncTransport
from .config import ConnectionConfig
from .sandbox import AsyncSandboxes, Sandboxes
from .templates import AsyncTemplates, Templates


class DevBox:
    """Reusable client for listing or operating multiple sandboxes.

    Most applications can start with ``Sandbox.create()`` instead.
    """

    def __init__(
        self,
        *,
        api_key: str | None = None,
        api_url: str | None = None,
        gateway_url: str | None = None,
        request_timeout: float | None = None,
        headers: Mapping[str, str] | None = None,
        http_transport: httpx.BaseTransport | None = None,
    ) -> None:
        config = ConnectionConfig.resolve(
            api_key=api_key,
            api_url=api_url,
            gateway_url=gateway_url,
            request_timeout=request_timeout,
            headers=headers,
        )
        request_headers = {**config.headers, "X-API-Key": config.api_key}
        self._transport = SyncTransport(
            config.api_url,
            headers=request_headers,
            timeout=config.control_timeout,
            transport=http_transport,
        )
        self.sandboxes = Sandboxes(
            self._transport,
            config.request_timeout,
            gateway_url=config.gateway_url,
        )
        self.templates = Templates(self._transport)

    def close(self) -> None:
        """Closes the shared control-plane transport and idle connections."""
        self._transport.close()

    def __enter__(self) -> DevBox:
        return self

    def __exit__(
        self,
        exc_type: type[BaseException] | None,
        exc_value: BaseException | None,
        traceback: TracebackType | None,
    ) -> None:
        self.close()


class AsyncDevBox:
    """Asynchronous counterpart of :class:`DevBox`.

    Most applications can start with ``await AsyncSandbox.create()`` instead.
    """

    def __init__(
        self,
        *,
        api_key: str | None = None,
        api_url: str | None = None,
        gateway_url: str | None = None,
        request_timeout: float | None = None,
        headers: Mapping[str, str] | None = None,
        http_transport: httpx.AsyncBaseTransport | None = None,
    ) -> None:
        config = ConnectionConfig.resolve(
            api_key=api_key,
            api_url=api_url,
            gateway_url=gateway_url,
            request_timeout=request_timeout,
            headers=headers,
        )
        request_headers = {**config.headers, "X-API-Key": config.api_key}
        self._transport = AsyncTransport(
            config.api_url,
            headers=request_headers,
            timeout=config.control_timeout,
            transport=http_transport,
        )
        self.sandboxes = AsyncSandboxes(
            self._transport,
            config.request_timeout,
            gateway_url=config.gateway_url,
        )
        self.templates = AsyncTemplates(self._transport)

    async def close(self) -> None:
        """Closes the shared control-plane transport and idle connections."""
        await self._transport.close()

    async def __aenter__(self) -> AsyncDevBox:
        return self

    async def __aexit__(
        self,
        exc_type: type[BaseException] | None,
        exc_value: BaseException | None,
        traceback: TracebackType | None,
    ) -> None:
        await self.close()
