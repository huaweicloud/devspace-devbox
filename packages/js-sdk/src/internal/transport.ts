import { rootCertificates } from "node:tls";
import { Agent, type Dispatcher, fetch } from "undici";
import {
  AuthenticationError,
  ConflictError,
  DevBoxError,
  type DevBoxErrorOptions,
  type ErrorDetail,
  NotFoundError,
  PermissionDeniedError,
  ProtocolError,
  RateLimitError,
  RequestTimeoutError,
  ServiceUnavailableError,
  ValidationError,
} from "../errors.js";
import { VERSION } from "../version.js";
import { SERVICE_CA } from "./service-ca.js";
import { objectValue, optionalString, type WireObject } from "./wire.js";

const RETRY_DELAYS_MS = [100, 200] as const;
type FetchResponse = Awaited<ReturnType<typeof fetch>>;

export type QueryValue = string | number | boolean | null | undefined;
export type Query = Record<string, QueryValue>;

export interface RequestOptions {
  json?: unknown;
  body?: Uint8Array;
  params?: Query;
  headers?: Record<string, string>;
}

interface ResponseValue<T> {
  body: T;
  headers: FetchResponse["headers"];
}

export class ConnectStream implements AsyncIterable<WireObject> {
  readonly #controller = new AbortController();
  readonly #open: () => Promise<FetchResponse>;
  #closed = false;

  constructor(open: () => Promise<FetchResponse>) {
    this.#open = open;
  }

  close(): void {
    if (!this.#closed) {
      this.#closed = true;
      this.#controller.abort();
    }
  }

  async *[Symbol.asyncIterator](): AsyncIterator<WireObject> {
    if (this.#closed) return;
    const response = await this.#openWithSignal();
    const contentType = response.headers.get("content-type")?.split(";", 1)[0]?.toLowerCase();
    if (contentType !== "application/connect+json") {
      throw new ProtocolError("EnvD returned an invalid Connect content type");
    }
    if (!response.body) throw new ProtocolError("EnvD returned an empty Connect stream");

    const decoder = new ConnectDecoder();
    const reader = response.body.getReader();
    try {
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        for (const event of decoder.feed(value)) yield event;
      }
      decoder.finish();
    } catch (error) {
      if (this.#closed && isAbort(error)) return;
      throw mapTransportError(error);
    } finally {
      reader.releaseLock();
      this.#closed = true;
    }
  }

  async #openWithSignal(): Promise<FetchResponse> {
    try {
      return await this.#open();
    } catch (error) {
      if (this.#closed && isAbort(error)) throw new RequestTimeoutError("stream was closed");
      throw mapTransportError(error);
    }
  }

  get signal(): AbortSignal {
    return this.#controller.signal;
  }
}

export class Transport {
  readonly #baseUrl: string;
  readonly #headers: Record<string, string>;
  readonly #timeoutMs: number;
  readonly #dispatcher: Dispatcher;
  readonly #ownsDispatcher: boolean;

  constructor(
    baseUrl: string,
    options: {
      headers?: Record<string, string>;
      timeoutMs?: number;
      dispatcher?: Dispatcher;
    } = {},
  ) {
    this.#baseUrl = baseUrl.replace(/\/$/, "");
    this.#headers = { "User-Agent": `devbox-js/${VERSION}`, ...options.headers };
    this.#timeoutMs = options.timeoutMs ?? 30_000;
    this.#dispatcher =
      options.dispatcher ??
      new Agent({
        connect: { ca: [...rootCertificates, SERVICE_CA] },
      });
    this.#ownsDispatcher = !options.dispatcher;
  }

  async request<T = unknown>(
    method: string,
    path: string,
    options: RequestOptions = {},
  ): Promise<T> {
    return (await this.requestWithHeaders<T>(method, path, options)).body;
  }

  async requestWithHeaders<T = unknown>(
    method: string,
    path: string,
    options: RequestOptions = {},
  ): Promise<ResponseValue<T>> {
    const response = await this.#send(method, path, options);
    return { body: (await responseBody(response)) as T, headers: response.headers };
  }

  async requestBytes(
    method: string,
    path: string,
    options: RequestOptions = {},
  ): Promise<Uint8Array> {
    const response = await this.#send(method, path, options);
    return new Uint8Array(await response.arrayBuffer());
  }

  connectUnary<T = unknown>(path: string, body: unknown): Promise<T> {
    return this.request("POST", path, {
      json: body,
      headers: { "Connect-Protocol-Version": "1", "Content-Type": "application/json" },
    });
  }

  connectStream(
    path: string,
    body: unknown,
    timeoutMs?: number,
    extraHeaders: Record<string, string> = {},
  ): ConnectStream {
    const stream = new ConnectStream(async () => {
      const normalizedTimeout = normalizeStreamTimeout(timeoutMs);
      const headers: Record<string, string> = {
        ...this.#headers,
        "Connect-Protocol-Version": "1",
        "Content-Type": "application/connect+json",
        ...extraHeaders,
      };
      if (normalizedTimeout !== undefined)
        headers["Connect-Timeout-Ms"] = String(normalizedTimeout);

      const signal =
        normalizedTimeout === undefined
          ? stream.signal
          : AbortSignal.any([stream.signal, AbortSignal.timeout(normalizedTimeout)]);
      const response = await fetch(this.#url(path), {
        method: "POST",
        headers,
        body: connectFrame(body),
        redirect: "manual",
        signal,
        dispatcher: this.#dispatcher,
      });
      await validateResponse(response);
      return response;
    });
    return stream;
  }

  async close(): Promise<void> {
    if (this.#ownsDispatcher && "close" in this.#dispatcher) {
      await this.#dispatcher.close();
    }
  }

  async #send(method: string, path: string, options: RequestOptions): Promise<FetchResponse> {
    for (let attempt = 0; ; attempt += 1) {
      try {
        const headers = { ...this.#headers, ...options.headers };
        let body: Uint8Array | string | undefined;
        if (options.json !== undefined) {
          headers["Content-Type"] ??= "application/json";
          body = JSON.stringify(options.json);
        } else {
          body = options.body;
        }
        const response = await fetch(this.#url(path, options.params), {
          method,
          headers,
          body,
          redirect: "manual",
          signal: AbortSignal.timeout(this.#timeoutMs),
          dispatcher: this.#dispatcher,
        });
        await validateResponse(response);
        return response;
      } catch (error) {
        if (isConnectionError(error) && attempt < RETRY_DELAYS_MS.length) {
          await sleep(RETRY_DELAYS_MS[attempt] ?? 0);
          continue;
        }
        throw mapTransportError(error);
      }
    }
  }

  #url(path: string, params?: Query): string {
    const url = new URL(path, `${this.#baseUrl}/`);
    for (const [key, value] of Object.entries(params ?? {})) {
      if (value !== undefined && value !== null) url.searchParams.set(key, String(value));
    }
    return url.toString();
  }
}

class ConnectDecoder {
  #buffer: Uint8Array<ArrayBufferLike> = new Uint8Array();
  #ended = false;

  feed(chunk: Uint8Array<ArrayBufferLike>): WireObject[] {
    if (this.#ended && chunk.length > 0)
      throw new ProtocolError("EnvD sent data after the Connect stream ended");
    this.#buffer = concat(this.#buffer, chunk);
    const events: WireObject[] = [];

    while (this.#buffer.length >= 5) {
      const flags = this.#buffer[0] ?? 0;
      const size = new DataView(this.#buffer.buffer, this.#buffer.byteOffset + 1, 4).getUint32(0);
      if (this.#buffer.length < size + 5) break;
      const data = this.#buffer.slice(5, size + 5);
      this.#buffer = this.#buffer.slice(size + 5);
      if ((flags & 1) !== 0)
        throw new ProtocolError("compressed Connect messages are not supported");
      const payload = decodeObject(data);
      if ((flags & 2) !== 0) {
        this.#ended = true;
        if (payload.error && typeof payload.error === "object")
          raiseConnectError(objectValue(payload.error));
        if (this.#buffer.length > 0)
          throw new ProtocolError("EnvD sent data after the Connect stream ended");
        break;
      }
      events.push(payload);
    }
    return events;
  }

  finish(): void {
    if (this.#buffer.length > 0)
      throw new ProtocolError("EnvD returned a truncated Connect stream");
    if (!this.#ended) throw new ProtocolError("EnvD closed the Connect stream without a trailer");
  }
}

async function validateResponse(response: FetchResponse): Promise<void> {
  if (response.status >= 300 && response.status < 400) {
    await response.body?.cancel();
    throw new ProtocolError("DevBox service returned an unexpected redirect");
  }
  if (!response.ok) await raiseForResponse(response);
}

async function responseBody(response: FetchResponse): Promise<unknown> {
  if (response.status === 204) return undefined;
  const text = await response.text();
  if (!text) return undefined;
  try {
    return JSON.parse(text);
  } catch (cause) {
    throw new ProtocolError("DevBox returned invalid JSON", { cause });
  }
}

async function raiseForResponse(response: FetchResponse): Promise<never> {
  let payload: WireObject = {};
  try {
    payload = objectValue(await response.json());
  } catch {
    // Some gateways return an empty or plain-text error response.
  }
  const raw = payload.error;
  const error = raw && typeof raw === "object" ? objectValue(raw) : payload;
  const message = String(
    error.message ?? error.error_message ?? `request failed with status ${response.status}`,
  );
  const code = String(error.code ?? error.error_code ?? error.error ?? response.status);
  const details = errorDetails(error.details);
  const options: DevBoxErrorOptions = {
    code,
    statusCode: response.status,
    target: optionalString(error.target),
    details,
    requestId: response.headers.get("x-request-id") ?? undefined,
    retryAfter: retryAfter(response.headers.get("retry-after")),
  };
  const ErrorType = errorType(response.status);
  throw new ErrorType(message, options);
}

function raiseConnectError(error: WireObject): never {
  const code = String(error.code ?? "unknown");
  const message = String(error.message ?? "EnvD request failed");
  const mapping: Record<string, [number, typeof DevBoxError]> = {
    invalid_argument: [400, ValidationError],
    out_of_range: [400, ValidationError],
    unauthenticated: [401, AuthenticationError],
    permission_denied: [403, PermissionDeniedError],
    not_found: [404, NotFoundError],
    already_exists: [409, ConflictError],
    aborted: [409, ConflictError],
    resource_exhausted: [429, RateLimitError],
    deadline_exceeded: [408, RequestTimeoutError],
    unavailable: [503, ServiceUnavailableError],
    internal: [500, ServiceUnavailableError],
  };
  const [statusCode, ErrorType] = mapping[code] ?? [undefined, DevBoxError];
  throw new ErrorType(message, { code, statusCode });
}

function errorType(status: number): typeof DevBoxError {
  if (status >= 500) return ServiceUnavailableError;
  return (
    {
      400: ValidationError,
      401: AuthenticationError,
      403: PermissionDeniedError,
      404: NotFoundError,
      408: RequestTimeoutError,
      409: ConflictError,
      422: ValidationError,
      429: RateLimitError,
    }[status] ?? DevBoxError
  );
}

function errorDetails(value: unknown): ErrorDetail[] {
  if (!Array.isArray(value)) return [];
  return value.flatMap((item) => {
    if (!item || typeof item !== "object" || Array.isArray(item)) return [];
    const detail = item as WireObject;
    return [
      {
        code: String(detail.code ?? ""),
        message: String(detail.message ?? ""),
        target: optionalString(detail.target),
      },
    ];
  });
}

function retryAfter(value: string | null): number | undefined {
  if (!value) return undefined;
  const seconds = Number(value);
  if (Number.isFinite(seconds)) return seconds;
  const date = Date.parse(value);
  return Number.isNaN(date) ? undefined : Math.max(0, (date - Date.now()) / 1000);
}

function connectFrame(value: unknown): Uint8Array {
  const data = new TextEncoder().encode(JSON.stringify(value));
  const frame = new Uint8Array(data.length + 5);
  new DataView(frame.buffer).setUint32(1, data.length);
  frame.set(data, 5);
  return frame;
}

function decodeObject(value: Uint8Array): WireObject {
  try {
    return objectValue(
      JSON.parse(new TextDecoder().decode(value)),
      "EnvD returned an invalid Connect message",
    );
  } catch (error) {
    if (error instanceof DevBoxError) throw error;
    throw new ProtocolError("EnvD returned invalid Connect JSON", { cause: error });
  }
}

function normalizeStreamTimeout(value?: number): number | undefined {
  if (value === undefined || value === 0) return undefined;
  if (value < 0) throw new RangeError("timeoutMs must be non-negative");
  return Math.max(1, Math.round(value));
}

function mapTransportError(error: unknown): Error {
  if (error instanceof DevBoxError) return error;
  if (isAbort(error)) return new RequestTimeoutError("request timed out", { cause: error });
  if (error instanceof Error)
    return new ServiceUnavailableError("unable to reach DevBox service", { cause: error });
  return new ServiceUnavailableError("unable to reach DevBox service");
}

function isAbort(error: unknown): boolean {
  return error instanceof Error && (error.name === "AbortError" || error.name === "TimeoutError");
}

function isConnectionError(error: unknown): boolean {
  if (!(error instanceof TypeError)) return false;
  const cause = error.cause as NodeJS.ErrnoException | undefined;
  return ["ECONNREFUSED", "ECONNRESET", "ENOTFOUND", "EAI_AGAIN", "ETIMEDOUT"].includes(
    cause?.code ?? "",
  );
}

function concat(left: Uint8Array, right: Uint8Array): Uint8Array {
  const result = new Uint8Array(left.length + right.length);
  result.set(left);
  result.set(right, left.length);
  return result;
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
