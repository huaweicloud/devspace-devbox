"""Exception types and API error decoding for the DevBox SDK.

Every error raised by the SDK derives from :class:`DevBoxError`; HTTP and
Connect (EnvD) error responses are mapped to the concrete subclasses by
:func:`raise_for_response` and :func:`raise_connect_error`.
"""

from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass
from datetime import datetime, timezone
from email.utils import parsedate_to_datetime
from typing import TYPE_CHECKING, Any, NoReturn

import httpx

if TYPE_CHECKING:
    from .models import CommandResult


@dataclass(frozen=True, slots=True)
class ErrorDetail:
    """One structured detail entry from a DevBox API error response."""

    code: str
    message: str
    target: str | None = None


class DevBoxError(Exception):
    """Base exception returned by the DevBox SDK."""

    def __init__(
        self,
        message: str,
        *,
        code: str = "",
        status_code: int | None = None,
        target: str | None = None,
        details: tuple[ErrorDetail, ...] = (),
        request_id: str | None = None,
        retry_after: float | None = None,
    ) -> None:
        super().__init__(message)
        self.message = message
        self.code = code
        self.status_code = status_code
        self.target = target
        self.details = details
        self.request_id = request_id
        self.retry_after = retry_after

    def __str__(self) -> str:
        suffix = f" [{self.code}]" if self.code else ""
        return self.message + suffix


class ConfigurationError(DevBoxError):
    """Invalid client configuration (blank API key, bad URL, bad timeout...)."""


class AuthenticationError(DevBoxError):
    """The API key was rejected (HTTP 401)."""


class PermissionDeniedError(DevBoxError):
    """The API key lacks permission for the operation (HTTP 403)."""


class ValidationError(DevBoxError):
    """The request payload was rejected (HTTP 400/422)."""


class NotFoundError(DevBoxError):
    """The referenced resource does not exist (HTTP 404)."""


class ConflictError(DevBoxError):
    """The operation conflicts with the current resource state (HTTP 409)."""


class RateLimitError(DevBoxError):
    """Rate limit exceeded (HTTP 429); see :attr:`DevBoxError.retry_after`."""


class RequestTimeoutError(DevBoxError):
    """The request timed out (HTTP 408 or transport timeout)."""


class ServiceUnavailableError(DevBoxError):
    """The service is temporarily unavailable (HTTP 503)."""


class ProtocolError(DevBoxError):
    """The server response violated the SDK protocol expectations."""


class CommandExitError(DevBoxError):
    """Raised when a command exits nonzero and ``check`` is true."""

    def __init__(self, result: CommandResult) -> None:
        super().__init__(f"command exited with status {result.exit_code}", code="COMMAND_EXIT")
        self.result = result


def transport_error(error: Exception) -> Exception:
    """Maps an httpx transport failure to a :class:`DevBoxError` subclass."""
    if isinstance(error, httpx.TimeoutException):
        return RequestTimeoutError(f"request timed out ({type(error).__name__})")
    if isinstance(error, httpx.HTTPError):
        detail = str(error.__cause__ or error)
        return ServiceUnavailableError(
            f"DevBox transport failed ({type(error).__name__}): {detail}"
        )
    return error


def raise_for_response(response: httpx.Response) -> NoReturn:
    """Raises the matching :class:`DevBoxError` subclass for a failed HTTP response."""
    payload: Mapping[str, Any] = {}
    try:
        decoded = response.json()
        if isinstance(decoded, Mapping):
            payload = decoded
    except ValueError:
        pass

    raw_error = payload.get("error", payload)
    error = raw_error if isinstance(raw_error, Mapping) else payload
    message = str(
        error.get("message")
        or error.get("error_message")
        or f"request failed with status {response.status_code}"
    )
    code = str(
        error.get("error_code") or error.get("error") or error.get("code") or response.status_code
    )
    target = str(error["target"]) if error.get("target") is not None else None
    details = _details(error.get("details"))
    request_id = response.headers.get("X-Request-Id") or response.headers.get("X-Request-ID")
    retry_after = _retry_after(response.headers.get("Retry-After"))
    exception_type: type[DevBoxError] = {
        400: ValidationError,
        401: AuthenticationError,
        403: PermissionDeniedError,
        404: NotFoundError,
        408: RequestTimeoutError,
        409: ConflictError,
        422: ValidationError,
        429: RateLimitError,
    }.get(response.status_code, DevBoxError)
    if response.status_code >= 500:
        exception_type = ServiceUnavailableError
    raise exception_type(
        message,
        code=code,
        status_code=response.status_code,
        target=target,
        details=details,
        request_id=request_id,
        retry_after=retry_after,
    )


def raise_connect_error(error: Mapping[str, Any]) -> NoReturn:
    """Raises the matching :class:`DevBoxError` subclass for a Connect (EnvD) error."""
    code = str(error.get("code", "unknown"))
    message = str(error.get("message") or "EnvD request failed")
    status_code, exception_type = {
        "invalid_argument": (400, ValidationError),
        "out_of_range": (400, ValidationError),
        "unauthenticated": (401, AuthenticationError),
        "permission_denied": (403, PermissionDeniedError),
        "not_found": (404, NotFoundError),
        "already_exists": (409, ConflictError),
        "aborted": (409, ConflictError),
        "resource_exhausted": (429, RateLimitError),
        "deadline_exceeded": (408, RequestTimeoutError),
        "unavailable": (503, ServiceUnavailableError),
        "internal": (500, ServiceUnavailableError),
    }.get(code, (None, DevBoxError))
    raise exception_type(message, code=code, status_code=status_code)


def _details(value: object) -> tuple[ErrorDetail, ...]:
    if not isinstance(value, list):
        return ()
    result: list[ErrorDetail] = []
    for item in value:
        if not isinstance(item, Mapping):
            continue
        result.append(
            ErrorDetail(
                code=str(item.get("code", "")),
                message=str(item.get("message", "")),
                target=str(item["target"]) if item.get("target") is not None else None,
            )
        )
    return tuple(result)


def _retry_after(value: str | None) -> float | None:
    if value is None:
        return None
    try:
        return float(value)
    except ValueError:
        try:
            retry_at = parsedate_to_datetime(value)
        except (TypeError, ValueError):
            return None
        if retry_at.tzinfo is None:
            retry_at = retry_at.replace(tzinfo=timezone.utc)
        return max(0.0, (retry_at - datetime.now(timezone.utc)).total_seconds())
