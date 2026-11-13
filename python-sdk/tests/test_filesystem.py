from __future__ import annotations

import json
from collections.abc import AsyncIterator, Iterator
from typing import Any

import httpx
import pytest

from devbox._transport import AsyncTransport, SyncTransport
from devbox.filesystem import AsyncFilesystem, Filesystem
from devbox.models import FileType


def test_files_use_rest_content_and_connect_metadata() -> None:
    requests: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        requests.append(request)
        if request.method == "POST" and request.url.path == "/files":
            assert request.url.params["path"] == "/tmp/data.bin"
            assert request.headers["Content-Type"] == "application/octet-stream"
            assert request.content == b"\x00\x01\x02"
            return httpx.Response(
                200,
                json=[{"name": "data.bin", "path": "/tmp/data.bin", "type": "file", "size": 3}],
            )
        if request.method == "GET":
            return httpx.Response(200, content=b"\x00\x01\x02")
        body = json.loads(request.content)
        assert request.url.path == "/filesystem.Filesystem/Stat"
        assert body == {"path": "/tmp/data.bin"}
        return httpx.Response(
            200,
            json={
                "entry": {
                    "name": "data.bin",
                    "path": "/tmp/data.bin",
                    "type": "FILE_TYPE_FILE",
                    "size": "3",
                    "mode": 420,
                }
            },
            headers={"Content-Type": "application/json"},
        )

    with _transport(handler) as transport:
        files = Filesystem(lambda: transport)
        written = files.write("/tmp/data.bin", b"\x00\x01\x02")
        content = files.read_bytes("/tmp/data.bin")
        info = files.stat("/tmp/data.bin")

    assert written.size == 3
    assert content == b"\x00\x01\x02"
    assert info.type is FileType.FILE
    assert info.mode == 0o644
    assert requests[2].headers["Connect-Protocol-Version"] == "1"


def test_list_uses_envd_rpc_shape() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.url.path == "/filesystem.Filesystem/ListDir"
        assert json.loads(request.content) == {"path": "/tmp", "depth": 2}
        return httpx.Response(
            200,
            json={
                "entries": [
                    {
                        "name": "cache",
                        "path": "/tmp/cache",
                        "type": "FILE_TYPE_DIRECTORY",
                    }
                ]
            },
            headers={"Content-Type": "application/json"},
        )

    with _transport(handler) as transport:
        entries = Filesystem(lambda: transport).list("/tmp", depth=2)

    assert entries[0].type is FileType.DIRECTORY


def test_watch_reads_envd_top_level_event() -> None:
    stream = TrackingStream(
        _stream_body(
            {"start": {}},
            {"filesystem": {"name": "created.txt", "type": "EVENT_TYPE_CREATE"}},
        )
    )

    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(
            200,
            stream=stream,
            headers={"Content-Type": "application/connect+json"},
        )

    with _transport(handler) as transport:
        events = Filesystem(lambda: transport).watch("/tmp")
        event = next(events)
        events.close()

    assert event.name == "created.txt"
    assert stream.closed is True


@pytest.mark.asyncio
async def test_async_watch_closes_transport_stream() -> None:
    stream = TrackingAsyncStream(
        _stream_body(
            {"start": {}},
            {"filesystem": {"name": "created.txt", "type": "EVENT_TYPE_CREATE"}},
        )
    )

    async def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(
            200,
            stream=stream,
            headers={"Content-Type": "application/connect+json"},
        )

    transport = AsyncTransport(
        "https://envd.test",
        headers={},
        timeout=30,
        transport=httpx.MockTransport(handler),
    )

    async def provide() -> AsyncTransport:
        return transport

    try:
        events = AsyncFilesystem(provide).watch("/tmp")
        event = await anext(events)
        await events.aclose()
    finally:
        await transport.close()

    assert event.name == "created.txt"
    assert stream.closed is True


def test_exists_maps_not_found_to_false() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(
            404,
            json={"error": {"code": "not_found", "message": "file not found"}},
        )

    with _transport(handler) as transport:
        assert Filesystem(lambda: transport).exists("/tmp/missing") is False


def test_sandbox_path_must_be_absolute() -> None:
    with _transport(lambda request: httpx.Response(500)) as transport:
        files = Filesystem(lambda: transport)
        with pytest.raises(ValueError, match="absolute"):
            files.read("relative.txt")


def _transport(handler: Any) -> SyncTransport:
    return SyncTransport(
        "https://envd.test",
        headers={},
        timeout=30,
        transport=httpx.MockTransport(handler),
    )


def _stream_response(*events: dict[str, object]) -> httpx.Response:
    return httpx.Response(
        200,
        content=_stream_body(*events),
        headers={"Content-Type": "application/connect+json"},
    )


def _stream_body(*events: dict[str, object]) -> bytes:
    return b"".join(_frame(event) for event in events) + _frame({}, flags=2)


def _frame(value: dict[str, object], *, flags: int = 0) -> bytes:
    data = json.dumps(value, separators=(",", ":")).encode()
    return bytes([flags]) + len(data).to_bytes(4, "big") + data


class TrackingStream(httpx.SyncByteStream):
    def __init__(self, content: bytes) -> None:
        self.content = content
        self.closed = False

    def __iter__(self) -> Iterator[bytes]:
        yield self.content

    def close(self) -> None:
        self.closed = True


class TrackingAsyncStream(httpx.AsyncByteStream):
    def __init__(self, content: bytes) -> None:
        self.content = content
        self.closed = False

    async def __aiter__(self) -> AsyncIterator[bytes]:
        yield self.content

    async def aclose(self) -> None:
        self.closed = True
