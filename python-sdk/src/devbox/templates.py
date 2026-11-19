"""Template operations against the DevBox Manager API."""

from __future__ import annotations

from collections.abc import Mapping

from ._transport import AsyncTransport, SyncTransport
from .errors import ProtocolError
from .models import TemplateInfo


class Templates:
    """Template operations against the Manager API."""

    def __init__(self, transport: SyncTransport) -> None:
        self._transport = transport

    def list(self) -> list[TemplateInfo]:
        """List the latest usable build of each public template."""
        return _parse_templates(self._transport.request("GET", "/templates"))


class AsyncTemplates:
    """Asynchronous counterpart of :class:`Templates`."""

    def __init__(self, transport: AsyncTransport) -> None:
        self._transport = transport

    async def list(self) -> list[TemplateInfo]:
        """List the latest usable build of each public template."""
        return _parse_templates(await self._transport.request("GET", "/templates"))


def _parse_templates(value: object) -> list[TemplateInfo]:
    if not isinstance(value, list) or not all(isinstance(item, Mapping) for item in value):
        raise ProtocolError("DevBox returned an invalid template list")
    return [TemplateInfo.from_wire(item) for item in value]
