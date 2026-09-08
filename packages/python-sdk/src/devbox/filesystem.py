from __future__ import annotations

from collections.abc import AsyncGenerator, Awaitable, Callable, Generator, Mapping, Sequence
from pathlib import Path
from typing import Any, cast

from ._transport import AsyncTransport, SyncTransport
from .errors import NotFoundError, ProtocolError
from .models import FileInfo

SyncTransportProvider = Callable[[], SyncTransport]
AsyncTransportProvider = Callable[[], Awaitable[AsyncTransport]]

_FILESYSTEM = "/filesystem.Filesystem"


class Filesystem:
    """Read and modify the filesystem inside a sandbox."""

    def __init__(self, transport: SyncTransportProvider) -> None:
        self._transport = transport

    def read_bytes(self, path: str) -> bytes:
        """Read a remote file without text decoding."""
        return self._transport().request_bytes("GET", "/files", params={"path": _path(path)})

    def read(self, path: str, *, encoding: str = "utf-8") -> str:
        """Read and decode a remote text file."""
        return self.read_bytes(path).decode(encoding)

    def write(
        self,
        path: str,
        data: str | bytes,
        *,
        encoding: str = "utf-8",
    ) -> FileInfo:
        """Create or replace one remote file."""
        content = data.encode(encoding) if isinstance(data, str) else data
        payload = self._transport().request_content(
            "POST",
            "/files",
            content,
            params={"path": _path(path)},
            headers={"Content-Type": "application/octet-stream"},
        )
        return FileInfo.from_wire(_first_item(payload))

    def write_batch(
        self, files: Mapping[str, str | bytes], *, encoding: str = "utf-8"
    ) -> tuple[FileInfo, ...]:
        """Write several remote files in mapping order."""
        return tuple(self.write(path, data, encoding=encoding) for path, data in files.items())

    def list(self, path: str, *, depth: int = 1) -> tuple[FileInfo, ...]:
        """List entries below a remote directory."""
        if depth < 1:
            raise ValueError("depth must be positive")
        payload = self._transport().connect_unary(
            f"{_FILESYSTEM}/ListDir", {"path": _path(path), "depth": depth}
        )
        return tuple(FileInfo.from_wire(item) for item in _items(payload, "entries"))

    def stat(self, path: str) -> FileInfo:
        """Return metadata for a remote path."""
        payload = self._transport().connect_unary(f"{_FILESYSTEM}/Stat", {"path": _path(path)})
        return FileInfo.from_wire(_entry(payload))

    def exists(self, path: str) -> bool:
        """Return whether a remote path exists."""
        try:
            self.stat(path)
            return True
        except NotFoundError:
            return False

    def make_dir(self, path: str) -> None:
        """Create a remote directory."""
        self._transport().connect_unary(f"{_FILESYSTEM}/MakeDir", {"path": _path(path)})

    def move(self, source: str, destination: str) -> None:
        """Move or rename a remote path."""
        self._transport().connect_unary(
            f"{_FILESYSTEM}/Move",
            {"source": _path(source), "destination": _path(destination)},
        )

    def remove(self, path: str) -> None:
        """Remove a remote file or directory."""
        self._transport().connect_unary(f"{_FILESYSTEM}/Remove", {"path": _path(path)})

    def upload(self, local_path: str | Path, remote_path: str) -> FileInfo:
        """Upload one local file to the sandbox."""
        return self.write(remote_path, Path(local_path).read_bytes())

    def download(self, remote_path: str, local_path: str | Path) -> Path:
        """Download one sandbox file to the local machine."""
        destination = Path(local_path)
        destination.write_bytes(self.read_bytes(remote_path))
        return destination

    def watch(
        self,
        path: str,
        *,
        recursive: bool = False,
        include_entry: bool = True,
    ) -> Generator[Mapping[str, Any], None, None]:
        """Yield filesystem changes until the returned generator is closed."""
        events = self._transport().connect_stream(
            f"{_FILESYSTEM}/WatchDir",
            {
                "path": _path(path),
                "recursive": recursive,
                "includeEntry": include_entry,
            },
            timeout=None,
        )
        try:
            yield from _filesystem_events(events)
        finally:
            events.close()


class AsyncFilesystem:
    """Read and modify the sandbox filesystem asynchronously."""

    def __init__(self, transport: AsyncTransportProvider) -> None:
        self._transport = transport

    async def read_bytes(self, path: str) -> bytes:
        transport = await self._transport()
        return await transport.request_bytes("GET", "/files", params={"path": _path(path)})

    async def read(self, path: str, *, encoding: str = "utf-8") -> str:
        return (await self.read_bytes(path)).decode(encoding)

    async def write(
        self,
        path: str,
        data: str | bytes,
        *,
        encoding: str = "utf-8",
    ) -> FileInfo:
        content = data.encode(encoding) if isinstance(data, str) else data
        transport = await self._transport()
        payload = await transport.request_content(
            "POST",
            "/files",
            content,
            params={"path": _path(path)},
            headers={"Content-Type": "application/octet-stream"},
        )
        return FileInfo.from_wire(_first_item(payload))

    async def write_batch(
        self, files: Mapping[str, str | bytes], *, encoding: str = "utf-8"
    ) -> tuple[FileInfo, ...]:
        result: list[FileInfo] = []
        for path, data in files.items():
            result.append(await self.write(path, data, encoding=encoding))
        return tuple(result)

    async def list(self, path: str, *, depth: int = 1) -> tuple[FileInfo, ...]:
        if depth < 1:
            raise ValueError("depth must be positive")
        transport = await self._transport()
        payload = await transport.connect_unary(
            f"{_FILESYSTEM}/ListDir", {"path": _path(path), "depth": depth}
        )
        return tuple(FileInfo.from_wire(item) for item in _items(payload, "entries"))

    async def stat(self, path: str) -> FileInfo:
        transport = await self._transport()
        payload = await transport.connect_unary(f"{_FILESYSTEM}/Stat", {"path": _path(path)})
        return FileInfo.from_wire(_entry(payload))

    async def exists(self, path: str) -> bool:
        try:
            await self.stat(path)
            return True
        except NotFoundError:
            return False

    async def make_dir(self, path: str) -> None:
        transport = await self._transport()
        await transport.connect_unary(f"{_FILESYSTEM}/MakeDir", {"path": _path(path)})

    async def move(self, source: str, destination: str) -> None:
        transport = await self._transport()
        await transport.connect_unary(
            f"{_FILESYSTEM}/Move",
            {"source": _path(source), "destination": _path(destination)},
        )

    async def remove(self, path: str) -> None:
        transport = await self._transport()
        await transport.connect_unary(f"{_FILESYSTEM}/Remove", {"path": _path(path)})

    async def upload(self, local_path: str | Path, remote_path: str) -> FileInfo:
        return await self.write(remote_path, Path(local_path).read_bytes())

    async def download(self, remote_path: str, local_path: str | Path) -> Path:
        destination = Path(local_path)
        destination.write_bytes(await self.read_bytes(remote_path))
        return destination

    async def watch(
        self,
        path: str,
        *,
        recursive: bool = False,
        include_entry: bool = True,
    ) -> AsyncGenerator[Mapping[str, Any], None]:
        transport = await self._transport()
        events = transport.connect_stream(
            f"{_FILESYSTEM}/WatchDir",
            {
                "path": _path(path),
                "recursive": recursive,
                "includeEntry": include_entry,
            },
            timeout=None,
        )
        try:
            async for response in events:
                event = _filesystem_event(response)
                if event is not None:
                    yield event
        finally:
            await events.aclose()


def _filesystem_events(
    responses: Generator[Mapping[str, Any], None, None],
) -> Generator[Mapping[str, Any], None, None]:
    for response in responses:
        event = _filesystem_event(response)
        if event is not None:
            yield event


def _filesystem_event(response: Mapping[str, Any]) -> Mapping[str, Any] | None:
    value = response.get("filesystem")
    if value is None:
        event = response.get("event")
        value = event.get("filesystem") if isinstance(event, Mapping) else None
    return value if isinstance(value, Mapping) else None


def _path(value: str) -> str:
    if not value or not value.startswith("/"):
        raise ValueError("sandbox paths must be absolute")
    return value


def _entry(payload: object) -> Mapping[str, Any]:
    if isinstance(payload, Mapping) and isinstance(payload.get("entry"), Mapping):
        return cast(Mapping[str, Any], payload["entry"])
    raise ProtocolError("filesystem response does not contain an entry")


def _first_item(payload: object) -> Mapping[str, Any]:
    values = _items(payload, "files")
    if not values:
        raise ProtocolError("file upload response is empty")
    return values[0]


def _items(value: object, key: str) -> Sequence[Mapping[str, Any]]:
    if isinstance(value, list):
        source = value
    elif isinstance(value, Mapping):
        source = value.get(key, [])
    else:
        source = []
    if not isinstance(source, list):
        raise ProtocolError("filesystem response is invalid")
    return tuple(item for item in source if isinstance(item, Mapping))
