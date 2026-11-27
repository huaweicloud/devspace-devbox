/**
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2027. All rights reserved.
 */

/** One structured detail entry from a DevBox API error response. */
export interface ErrorDetail {
  code: string;
  message: string;
  target?: string;
}

/** Options accepted by every {@link DevBoxError} constructor. */
export interface DevBoxErrorOptions {
  code?: string;
  statusCode?: number;
  target?: string;
  details?: ErrorDetail[];
  requestId?: string;
  retryAfter?: number;
  cause?: unknown;
}

/** Base class of every error thrown by the SDK. */
export class DevBoxError extends Error {
  readonly code: string;
  readonly statusCode?: number;
  readonly target?: string;
  readonly details: readonly ErrorDetail[];
  readonly requestId?: string;
  readonly retryAfter?: number;

  constructor(message: string, options: DevBoxErrorOptions = {}) {
    super(message, { cause: options.cause });
    this.name = new.target.name;
    this.code = options.code ?? '';
    this.statusCode = options.statusCode;
    this.target = options.target;
    this.details = options.details ?? [];
    this.requestId = options.requestId;
    this.retryAfter = options.retryAfter;
  }
}

/** Invalid client configuration (blank API key, bad URL, bad timeout...). */
export class ConfigurationError extends DevBoxError {}
/** The API key was rejected (HTTP 401). */
export class AuthenticationError extends DevBoxError {}
/** The API key lacks permission for the operation (HTTP 403). */
export class PermissionDeniedError extends DevBoxError {}
/** The request payload was rejected (HTTP 400/422). */
export class ValidationError extends DevBoxError {}
/** The referenced resource does not exist (HTTP 404). */
export class NotFoundError extends DevBoxError {}
/** The operation conflicts with the current resource state (HTTP 409). */
export class ConflictError extends DevBoxError {}
/** Rate limit exceeded (HTTP 429); see {@link DevBoxError.retryAfter}. */
export class RateLimitError extends DevBoxError {}
/** The request timed out (HTTP 408 or transport timeout). */
export class RequestTimeoutError extends DevBoxError {}
/** The service is temporarily unavailable (HTTP 503). */
export class ServiceUnavailableError extends DevBoxError {}
/** The server response violated the SDK protocol expectations. */
export class ProtocolError extends DevBoxError {}

/** The subset of {@link CommandResult} carried by {@link CommandExitError}. */
export interface CommandResultLike {
  exitCode: number;
  stdout: string;
  stderr: string;
  pid?: number;
}

/** Thrown by `run`/`wait` when a command exits nonzero and `check` is true. */
export class CommandExitError extends DevBoxError {
  readonly result: CommandResultLike;

  constructor(result: CommandResultLike) {
    super(`command exited with status ${result.exitCode}`, { code: 'COMMAND_EXIT' });
    this.result = result;
  }
}
