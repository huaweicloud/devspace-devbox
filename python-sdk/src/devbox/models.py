from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass, field
from datetime import datetime, timezone
from enum import Enum
from typing import Any, Generic, TypeVar

from .errors import ProtocolError


class SandboxState(str, Enum):
    CREATING = "creating"
    RUNNING = "running"
    PAUSING = "pausing"
    PAUSED = "paused"
    RESUMING = "resuming"
    STOPPING = "stopping"
    STOPPED = "stopped"
    FAILED = "failed"


class FileType(str, Enum):
    FILE = "file"
    DIRECTORY = "directory"
    SYMLINK = "symlink"


@dataclass(frozen=True, slots=True)
class VolumeMount:
    name: str
    path: str

    def to_wire(self) -> dict[str, str]:
        return {"name": self.name, "path": self.path}

    @classmethod
    def from_wire(cls, value: Mapping[str, Any]) -> VolumeMount:
        return cls(name=str(_pick(value, "name")), path=str(_pick(value, "path")))


@dataclass(frozen=True, slots=True)
class SandboxLifecycle:
    auto_resume: bool
    on_timeout: str

    @classmethod
    def from_wire(cls, value: object) -> SandboxLifecycle | None:
        if not isinstance(value, Mapping):
            return None
        return cls(
            auto_resume=bool(value.get("autoResume", False)),
            on_timeout=str(value.get("onTimeout", "kill")),
        )


@dataclass(frozen=True, slots=True)
class SandboxInfo:
    sandbox_id: str
    template_id: str
    state: SandboxState
    client_id: str = ""
    alias: str | None = None
    started_at: datetime | None = None
    end_at: datetime | None = None
    envd_version: str = ""
    cpu_count: int | None = None
    memory_mb: int | None = None
    disk_size_mb: int | None = None
    metadata: Mapping[str, str] = field(default_factory=dict)
    lifecycle: SandboxLifecycle | None = None
    volume_mounts: tuple[VolumeMount, ...] = ()

    @classmethod
    def from_wire(cls, value: Mapping[str, Any]) -> SandboxInfo:
        return cls(
            sandbox_id=str(_pick(value, "sandboxId", "sandboxID", "sandbox_id", "id")),
            template_id=str(
                _pick(value, "templateId", "templateID", "template_id", default="default")
            ),
            state=_enum(SandboxState, _pick(value, "state", "status", default="running")),
            client_id=str(_pick(value, "clientID", "client_id", default="")),
            alias=_optional_str(value.get("alias")),
            started_at=parse_optional_datetime(
                _pick(value, "createdAt", "created_at", "startedAt", "started_at", default=None)
            ),
            end_at=parse_optional_datetime(
                _pick(value, "expiresAt", "expires_at", "endAt", "end_at", default=None)
            ),
            envd_version=str(_pick(value, "envdVersion", "envd_version", default="")),
            cpu_count=_optional_int(value.get("cpuCount")),
            memory_mb=_optional_int(value.get("memoryMB")),
            disk_size_mb=_optional_int(value.get("diskSizeMB")),
            metadata=_string_map(value.get("metadata")),
            lifecycle=SandboxLifecycle.from_wire(value.get("lifecycle")),
            volume_mounts=tuple(
                VolumeMount.from_wire(item) for item in _mapping_items(value.get("volumeMounts"))
            ),
        )


@dataclass(frozen=True, slots=True)
class SandboxConnection:
    sandbox_id: str
    domain: str
    tunnel_id: str = ""
    connect_token: str = field(default="", repr=False)
    token_lifetime: int | None = None
    token_expiration: int | None = None
    tunnel_lifetime: int | None = None
    tunnel_expiration: int | None = None
    protocol_version: str = "v1"

    @classmethod
    def from_wire(cls, value: Mapping[str, Any], sandbox_id: str) -> SandboxConnection:
        return cls(
            sandbox_id=sandbox_id,
            domain=_domain(value.get("domain")),
            tunnel_id=str(value.get("tunnelId") or ""),
            connect_token=str(value.get("connectToken") or ""),
            token_lifetime=_optional_int(value.get("tokenLifetime")),
            token_expiration=_optional_int(value.get("tokenExpiration")),
            tunnel_lifetime=_optional_int(value.get("tunnelLifetime")),
            tunnel_expiration=_optional_int(value.get("tunnelExpiration")),
            protocol_version=str(value.get("envdVersion") or "v1"),
        )


@dataclass(frozen=True, slots=True)
class SandboxMetrics:
    timestamp_unix: int
    cpu_count: int
    cpu_used_percent: float
    memory_used_bytes: int
    memory_total_bytes: int
    memory_cache_bytes: int
    disk_used_bytes: int
    disk_total_bytes: int
    timestamp: datetime | None = None

    @classmethod
    def from_wire(cls, value: Mapping[str, Any]) -> SandboxMetrics:
        return cls(
            timestamp_unix=_integer(_pick(value, "timestampUnix")),
            cpu_count=_integer(_pick(value, "cpuCount")),
            cpu_used_percent=_number(_pick(value, "cpuUsedPct")),
            memory_used_bytes=_integer(_pick(value, "memUsed")),
            memory_total_bytes=_integer(_pick(value, "memTotal")),
            memory_cache_bytes=_integer(_pick(value, "memCache")),
            disk_used_bytes=_integer(_pick(value, "diskUsed")),
            disk_total_bytes=_integer(_pick(value, "diskTotal")),
            timestamp=parse_optional_datetime(value.get("timestamp")),
        )


class LogLevel(str, Enum):
    ERROR = "ERROR"
    WARNING = "WARNING"
    INFO = "INFO"
    DEBUG = "DEBUG"
    TRACE = "TRACE"


class LogsDirection(str, Enum):
    BACKWARD = "backward"
    FORWARD = "forward"


@dataclass(frozen=True, slots=True)
class SandboxLogEntry:
    timestamp: datetime
    level: LogLevel
    message: str
    fields: Mapping[str, str] = field(default_factory=dict)

    @classmethod
    def from_wire(cls, value: Mapping[str, Any]) -> SandboxLogEntry:
        return cls(
            timestamp=parse_datetime(_pick(value, "timestamp")),
            level=_enum(LogLevel, _pick(value, "level")),
            message=str(_pick(value, "message")),
            fields=_string_map(value.get("fields")),
        )


@dataclass(frozen=True, slots=True)
class ProcessInfo:
    pid: int
    command: str
    running: bool
    started_at: datetime | None = None

    @classmethod
    def from_wire(cls, value: Mapping[str, Any]) -> ProcessInfo:
        config = value.get("config")
        process = config if isinstance(config, Mapping) else value
        cmd = str(_pick(process, "command", "cmd", default=""))
        args = _string_tuple(process.get("args"))
        return cls(
            pid=_integer(_pick(value, "pid")),
            command=" ".join((cmd, *args)).strip(),
            running=bool(_pick(value, "running", default=True)),
            started_at=parse_optional_datetime(
                _pick(value, "startedAt", "started_at", default=None)
            ),
        )


@dataclass(frozen=True, slots=True)
class OutputChunk:
    stream: str
    data: str
    timestamp: datetime | None = None


@dataclass(frozen=True, slots=True)
class CommandResult:
    exit_code: int
    stdout: str = ""
    stderr: str = ""
    pid: int | None = None


@dataclass(frozen=True, slots=True)
class PtySize:
    rows: int = 24
    cols: int = 80

    def __post_init__(self) -> None:
        if self.rows < 1 or self.cols < 1:
            raise ValueError("PTY rows and cols must be positive")


@dataclass(frozen=True, slots=True)
class FileInfo:
    name: str
    path: str
    type: FileType
    size: int
    mode: int | None = None
    permissions: str | None = None
    owner: str | None = None
    group: str | None = None
    modified_at: datetime | None = None
    symlink_target: str | None = None

    @classmethod
    def from_wire(cls, value: Mapping[str, Any]) -> FileInfo:
        file_type = {
            "FILE_TYPE_FILE": FileType.FILE,
            "FILE_TYPE_DIRECTORY": FileType.DIRECTORY,
            "FILE_TYPE_SYMLINK": FileType.SYMLINK,
        }.get(str(_pick(value, "type", default="file")))
        return cls(
            name=str(_pick(value, "name")),
            path=str(_pick(value, "path")),
            type=file_type or _enum(FileType, _pick(value, "type", default="file")),
            size=_integer(_pick(value, "size", default=0)),
            mode=_optional_int(value.get("mode")),
            permissions=_optional_str(value.get("permissions")),
            owner=_optional_str(value.get("owner")),
            group=_optional_str(value.get("group")),
            modified_at=parse_optional_datetime(
                _pick(value, "modifiedTime", "modifiedAt", "modified_at", default=None)
            ),
            symlink_target=_optional_str(
                _pick(value, "symlinkTarget", "symlink_target", default=None)
            ),
        )


@dataclass(frozen=True, slots=True)
class FileWatchEvent:
    name: str
    type: str
    path: str | None = None
    entry: FileInfo | None = None

    @classmethod
    def from_wire(cls, value: Mapping[str, Any]) -> FileWatchEvent:
        raw_entry = value.get("entry")
        return cls(
            name=str(value.get("name", "")),
            type=str(value.get("type", value.get("operation", ""))),
            path=_optional_str(value.get("path")),
            entry=FileInfo.from_wire(raw_entry) if isinstance(raw_entry, Mapping) else None,
        )


T = TypeVar("T")


@dataclass(frozen=True, slots=True)
class Page(Generic[T]):
    items: tuple[T, ...]
    next_token: str | None = None
    total: int | None = None


def parse_datetime(value: object) -> datetime:
    parsed = parse_optional_datetime(value)
    if parsed is None:
        raise ProtocolError("DevBox response timestamp is missing")
    return parsed


def parse_optional_datetime(value: object) -> datetime | None:
    if value is None or value == "":
        return None
    if isinstance(value, datetime):
        return value if value.tzinfo else value.replace(tzinfo=timezone.utc)
    try:
        if isinstance(value, int | float):
            return datetime.fromtimestamp(value, timezone.utc)
        text = str(value).replace("Z", "+00:00")
        parsed = datetime.fromisoformat(text)
    except (OSError, OverflowError, ValueError) as error:
        raise ProtocolError("DevBox returned an invalid timestamp") from error
    return parsed if parsed.tzinfo else parsed.replace(tzinfo=timezone.utc)


def _pick(value: Mapping[str, Any], *keys: str, default: object = ...) -> object:
    for key in keys:
        if key in value:
            return value[key]
    if default is not ...:
        return default
    raise ProtocolError(f"DevBox response field is missing: {keys[0]}")


def _string_map(value: object) -> Mapping[str, str]:
    if not isinstance(value, Mapping):
        return {}
    return {str(key): str(item) for key, item in value.items()}


def _optional_str(value: object) -> str | None:
    return None if value is None else str(value)


def _optional_int(value: object) -> int | None:
    return None if value is None else _integer(value)


def _string_tuple(value: object) -> tuple[str, ...]:
    if not isinstance(value, list | tuple):
        return ()
    return tuple(str(item) for item in value)


def _mapping_items(value: object) -> tuple[Mapping[str, Any], ...]:
    if not isinstance(value, list | tuple):
        return ()
    return tuple(item for item in value if isinstance(item, Mapping))


def _domain(value: object) -> str:
    raw = str(value or "")
    if not raw or raw == "None":
        return ""
    return raw if raw.startswith(("http://", "https://")) else f"https://{raw}"


def _integer(value: object) -> int:
    if isinstance(value, bool):
        return int(value)
    if isinstance(value, int):
        return value
    if isinstance(value, float):
        return int(value)
    if isinstance(value, str | bytes | bytearray):
        try:
            return int(value)
        except ValueError as error:
            raise ProtocolError("DevBox response value is not an integer") from error
    raise ProtocolError("DevBox response value is not an integer")


def _number(value: object) -> float:
    if isinstance(value, int | float | str | bytes | bytearray):
        try:
            return float(value)
        except ValueError as error:
            raise ProtocolError("DevBox response value is not a number") from error
    raise ProtocolError("DevBox response value is not a number")


E = TypeVar("E", bound=Enum)


def _enum(enum_type: type[E], value: object) -> E:
    try:
        return enum_type(str(value))
    except ValueError as error:
        raise ProtocolError(f"DevBox returned an invalid {enum_type.__name__}") from error
