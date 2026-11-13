from __future__ import annotations

import math
from collections.abc import Mapping
from dataclasses import dataclass, field
from datetime import datetime, timezone
from enum import Enum
from typing import Any, Generic, Literal, TypeVar, cast

from .errors import ProtocolError


class SandboxState(str, Enum):
    RUNNING = "running"
    PAUSED = "paused"
    STOPPED = "stopped"


class FileType(str, Enum):
    FILE = "file"
    DIRECTORY = "directory"
    SYMLINK = "symlink"


@dataclass(frozen=True, slots=True)
class SandboxLifecycle:
    on_timeout: Literal["kill", "pause"] = "kill"
    auto_resume: bool = False

    @classmethod
    def from_wire(cls, value: object) -> SandboxLifecycle | None:
        if not isinstance(value, Mapping):
            return None
        on_timeout = str(value.get("onTimeout", "kill"))
        if on_timeout not in {"kill", "pause"}:
            raise ProtocolError("DevBox returned an invalid sandbox lifecycle")
        return cls(
            auto_resume=bool(value.get("autoResume", False)),
            on_timeout=cast(Literal["kill", "pause"], on_timeout),
        )


@dataclass(frozen=True, slots=True)
class TemplateInfo:
    template_id: str
    name: str
    aliases: tuple[str, ...]
    build_id: str
    cpu_count: int
    memory_mb: int
    disk_size_mb: int
    names: tuple[str, ...] = ()

    @classmethod
    def from_wire(cls, value: Mapping[str, Any]) -> TemplateInfo:
        return cls(
            template_id=str(_pick(value, "templateId", "templateID", "template_id")),
            name=str(
                value.get("name")
                or next(iter(_string_tuple(value.get("names"))), value.get("templateID", ""))
            ),
            aliases=_string_tuple(value.get("aliases")),
            build_id=str(_pick(value, "buildId", "buildID", "build_id")),
            cpu_count=_integer(_pick(value, "cpuCount")),
            memory_mb=_integer(_pick(value, "memoryMB", "memoryMb", "memory_mb")),
            disk_size_mb=_integer(_pick(value, "diskSizeMB", "diskSizeMb", "disk_size_mb")),
            names=_string_tuple(value.get("names")),
        )


@dataclass(frozen=True, slots=True)
class SandboxInfo:
    sandbox_id: str
    template_id: str
    state: SandboxState
    started_at: datetime | None = None
    end_at: datetime | None = None
    cpu_count: int | None = None
    memory_mb: int | None = None
    disk_size_mb: int | None = None
    metadata: Mapping[str, str] = field(default_factory=dict)
    lifecycle: SandboxLifecycle | None = None

    @classmethod
    def from_wire(cls, value: Mapping[str, Any]) -> SandboxInfo:
        return cls(
            sandbox_id=str(_pick(value, "sandboxId", "sandboxID", "sandbox_id", "id")),
            template_id=str(
                _pick(value, "templateId", "templateID", "template_id", default="default")
            ),
            state=_enum(SandboxState, _pick(value, "state", "status", default="running")),
            started_at=parse_optional_datetime(
                _pick(value, "createdAt", "created_at", "startedAt", "started_at", default=None)
            ),
            end_at=parse_optional_datetime(
                _pick(value, "expiresAt", "expires_at", "endAt", "end_at", default=None)
            ),
            cpu_count=_optional_int(value.get("cpuCount")),
            memory_mb=_optional_int(value.get("memoryMB")),
            disk_size_mb=_optional_int(value.get("diskSizeMB")),
            metadata=_string_map(value.get("metadata")),
            lifecycle=SandboxLifecycle.from_wire(value.get("lifecycle")),
        )


@dataclass(frozen=True, slots=True)
class SandboxConnection:
    sandbox_id: str
    domain: str
    tunnel_id: str = ""
    connect_token: str = field(default="", repr=False)
    token_expiration: int | None = None
    envd_access_token: str = field(default="", repr=False)

    @classmethod
    def from_wire(cls, value: Mapping[str, Any], sandbox_id: str) -> SandboxConnection:
        domain = str(value.get("domain") or "")
        url = value.get("envdUrl")
        if not url:
            url = domain
            if (
                value.get("envdAccessToken")
                and not value.get("connectToken")
                and "://" not in domain
            ):
                url = f"https://49983-{sandbox_id}.{domain}" if domain else ""
        return cls(
            sandbox_id=sandbox_id,
            domain=_domain(url),
            tunnel_id=str(value.get("tunnelId") or ""),
            connect_token=str(value.get("connectToken", value.get("envdAccessToken")) or ""),
            token_expiration=_optional_int(value.get("tokenExpiration")),
            envd_access_token=str(value.get("envdAccessToken") or ""),
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
class EnvdHealth:
    envd_version: str


@dataclass(frozen=True, slots=True)
class OomKill:
    seq: int
    process: str

    @classmethod
    def from_wire(cls, value: Mapping[str, Any]) -> OomKill:
        return cls(seq=_integer(_pick(value, "seq")), process=str(_pick(value, "process")))


@dataclass(frozen=True, slots=True)
class EnvdMetrics:
    timestamp: datetime
    cpu_count: int
    cpu_used_pct: float
    mem_total: int
    mem_used: int
    mem_cache: int
    disk_total: int
    disk_used: int
    oom_kills: tuple[OomKill, ...] | None = None
    cpu_possible: int | None = None
    cpu_target: int | None = None
    cpu_target_attempts: int | None = None
    cpu_write_pending_ms: int | None = None

    @classmethod
    def from_wire(cls, value: Mapping[str, Any]) -> EnvdMetrics:
        timestamp = parse_optional_datetime(_pick(value, "ts"))
        if timestamp is None:
            raise ProtocolError("EnvD metrics response field is missing: ts")
        oom_kills: tuple[OomKill, ...] | None = None
        raw_oom_kills = value.get("oom_kills")
        if raw_oom_kills is not None:
            if not isinstance(raw_oom_kills, list) or not all(
                isinstance(entry, Mapping) for entry in raw_oom_kills
            ):
                raise ProtocolError("EnvD metrics response field is invalid: oom_kills")
            oom_kills = tuple(OomKill.from_wire(entry) for entry in raw_oom_kills)
        return cls(
            timestamp=timestamp,
            cpu_count=_integer(_pick(value, "cpu_count")),
            cpu_used_pct=_number(_pick(value, "cpu_used_pct")),
            mem_total=_integer(_pick(value, "mem_total")),
            mem_used=_integer(_pick(value, "mem_used")),
            mem_cache=_integer(_pick(value, "mem_cache")),
            disk_total=_integer(_pick(value, "disk_total")),
            disk_used=_integer(_pick(value, "disk_used")),
            oom_kills=oom_kills,
            cpu_possible=_optional_int(value.get("cpu_possible")),
            cpu_target=_optional_int(value.get("cpu_target")),
            cpu_target_attempts=_optional_int(value.get("cpu_target_attempts")),
            cpu_write_pending_ms=_optional_int(value.get("cpu_write_pending_ms")),
        )


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
    if isinstance(value, bool):
        value = int(value)
    if isinstance(value, int | float):
        try:
            number = float(value)
        except OverflowError as error:
            raise ProtocolError("DevBox response value is not a number") from error
        if not math.isfinite(number):
            raise ProtocolError("DevBox response value is not a finite number")
        return number
    if isinstance(value, str | bytes | bytearray):
        try:
            return float(value)
        except (OverflowError, ValueError) as error:
            raise ProtocolError("DevBox response value is not a number") from error
    raise ProtocolError("DevBox response value is not a number")


E = TypeVar("E", bound=Enum)


def _enum(enum_type: type[E], value: object) -> E:
    try:
        return enum_type(str(value))
    except ValueError as error:
        raise ProtocolError(f"DevBox returned an invalid {enum_type.__name__}") from error
