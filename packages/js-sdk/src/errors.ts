export interface ErrorDetail {
  code: string;
  message: string;
  target?: string;
}

export interface DevBoxErrorOptions {
  code?: string;
  statusCode?: number;
  target?: string;
  details?: ErrorDetail[];
  requestId?: string;
  retryAfter?: number;
  cause?: unknown;
}

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
    this.code = options.code ?? "";
    this.statusCode = options.statusCode;
    this.target = options.target;
    this.details = options.details ?? [];
    this.requestId = options.requestId;
    this.retryAfter = options.retryAfter;
  }
}

export class ConfigurationError extends DevBoxError {}
export class AuthenticationError extends DevBoxError {}
export class PermissionDeniedError extends DevBoxError {}
export class ValidationError extends DevBoxError {}
export class NotFoundError extends DevBoxError {}
export class ConflictError extends DevBoxError {}
export class RateLimitError extends DevBoxError {}
export class RequestTimeoutError extends DevBoxError {}
export class ServiceUnavailableError extends DevBoxError {}
export class ProtocolError extends DevBoxError {}

export interface CommandResultLike {
  exitCode: number;
  stdout: string;
  stderr: string;
  pid?: number;
}

export class CommandExitError extends DevBoxError {
  readonly result: CommandResultLike;

  constructor(result: CommandResultLike) {
    super(`command exited with status ${result.exitCode}`, { code: "COMMAND_EXIT" });
    this.result = result;
  }
}
