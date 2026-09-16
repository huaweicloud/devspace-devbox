"""Public DevBox SDK API.

Start with :class:`Sandbox` for one sandbox. Use :class:`DevBox` only when an
application needs to manage several sandboxes through one reusable client.
"""

from ._version import __version__
from .client import AsyncDevBox, DevBox
from .commands import AsyncCommandHandle, CommandHandle
from .errors import (
    AuthenticationError,
    CommandExitError,
    ConfigurationError,
    ConflictError,
    DevBoxError,
    ErrorDetail,
    NotFoundError,
    PermissionDeniedError,
    ProtocolError,
    RateLimitError,
    RequestTimeoutError,
    ServiceUnavailableError,
    ValidationError,
)
from .git import GitCredentials
from .models import (
    CommandResult,
    FileInfo,
    FileType,
    FileWatchEvent,
    LogLevel,
    LogsDirection,
    Page,
    ProcessInfo,
    PtySize,
    SandboxInfo,
    SandboxLifecycle,
    SandboxLogEntry,
    SandboxMetrics,
    SandboxState,
    VolumeMount,
)
from .sandbox import AsyncSandbox, Sandbox

__all__ = [
    "AsyncCommandHandle",
    "AsyncDevBox",
    "AsyncSandbox",
    "AuthenticationError",
    "CommandExitError",
    "CommandHandle",
    "CommandResult",
    "ConfigurationError",
    "ConflictError",
    "DevBox",
    "DevBoxError",
    "ErrorDetail",
    "FileInfo",
    "FileType",
    "FileWatchEvent",
    "GitCredentials",
    "LogLevel",
    "LogsDirection",
    "NotFoundError",
    "Page",
    "PermissionDeniedError",
    "ProcessInfo",
    "ProtocolError",
    "PtySize",
    "RateLimitError",
    "RequestTimeoutError",
    "Sandbox",
    "SandboxInfo",
    "SandboxLifecycle",
    "SandboxLogEntry",
    "SandboxMetrics",
    "SandboxState",
    "ServiceUnavailableError",
    "ValidationError",
    "VolumeMount",
    "__version__",
]
