from __future__ import annotations

from collections.abc import Awaitable, Callable, Mapping

from ._transport import AsyncTransport, SyncTransport
from .errors import ProtocolError
from .models import EnvdHealth, EnvdMetrics


class Envd:
    """Synchronous EnvD runtime API operations."""

    def __init__(self, transport: Callable[[], SyncTransport]) -> None:
        self._transport = transport

    def get_metrics(self) -> EnvdMetrics:
        """Return one realtime CPU, memory and disk sample."""
        return EnvdMetrics.from_wire(_mapping(self._transport().request("GET", "/metrics")))

    def get_envs(self) -> Mapping[str, str]:
        """Return EnvD's current environment variables."""
        value = self._transport().request("GET", "/envs")
        if not isinstance(value, Mapping):
            raise ProtocolError("EnvD returned an invalid envs response")
        return {str(key): str(item) for key, item in value.items()}

    def health(self) -> EnvdHealth:
        """Probe EnvD directly and return its version header."""
        _, headers = self._transport().request_with_headers("GET", "/health")
        return EnvdHealth(envd_version=headers.get("X-Envd-Version", ""))


class AsyncEnvd:
    """Asynchronous EnvD runtime API operations."""

    def __init__(self, transport: Callable[[], Awaitable[AsyncTransport]]) -> None:
        self._transport = transport

    async def get_metrics(self) -> EnvdMetrics:
        """Return one realtime CPU, memory and disk sample."""
        transport = await self._transport()
        return EnvdMetrics.from_wire(_mapping(await transport.request("GET", "/metrics")))

    async def get_envs(self) -> Mapping[str, str]:
        """Return EnvD's current environment variables."""
        transport = await self._transport()
        value = await transport.request("GET", "/envs")
        if not isinstance(value, Mapping):
            raise ProtocolError("EnvD returned an invalid envs response")
        return {str(key): str(item) for key, item in value.items()}

    async def health(self) -> EnvdHealth:
        """Probe EnvD directly and return its version header."""
        transport = await self._transport()
        _, headers = await transport.request_with_headers("GET", "/health")
        return EnvdHealth(envd_version=headers.get("X-Envd-Version", ""))


def _mapping(value: object) -> Mapping[str, object]:
    if not isinstance(value, Mapping):
        raise ProtocolError("EnvD returned an invalid object response")
    return value
