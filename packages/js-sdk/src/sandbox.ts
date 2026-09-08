import { randomUUID } from "node:crypto";
import type { Dispatcher } from "undici";
import { Commands } from "./commands.js";
import { type ConnectionConfig, type DevBoxOptions, resolveConfig } from "./config.js";
import { type ErrorDetail, NotFoundError, ProtocolError } from "./errors.js";
import { Filesystem } from "./filesystem.js";
import { Git } from "./git.js";
import { Transport } from "./internal/transport.js";
import {
  checkedTimeout,
  identifier,
  numberValue,
  objectValue,
  type WireObject,
} from "./internal/wire.js";
import {
  type LogLevel,
  type LogsDirection,
  type NetworkConfig,
  networkCreateWire,
  networkUpdateWire,
  type Page,
  parseConnection,
  parseLogEntry,
  parseMetrics,
  parseObjectItems,
  parseSandboxInfo,
  parseSnapshot,
  type SandboxConnection,
  type SandboxInfo,
  type SandboxLogEntry,
  type SandboxMetrics,
  SandboxState,
  type SnapshotInfo,
  type VolumeMount,
} from "./models.js";
import { Pty } from "./pty.js";

export interface CreateSandboxOptions {
  timeout?: number;
  envs?: Record<string, string>;
  metadata?: Record<string, string>;
  network?: NetworkConfig;
  secure?: boolean;
  clientId?: string;
  buildId?: string;
  volumeMounts?: VolumeMount[];
  idempotencyKey?: string;
}

export interface ListSandboxesOptions {
  metadata?: string;
  states?: SandboxState[];
  limit?: number;
  nextToken?: string;
}

export interface ListSnapshotsOptions {
  sandboxId?: string;
  name?: string;
  limit?: number;
  nextToken?: string;
}

export interface SandboxForkResult {
  sandbox?: Sandbox;
  error?: ErrorDetail;
}

interface SandboxContext {
  requestTimeoutMs: number;
  gatewayUrl?: string;
  dispatcher?: Dispatcher;
  ownsControl?: boolean;
}

export class Sandboxes {
  readonly #transport: Transport;
  readonly #context: SandboxContext;

  constructor(transport: Transport, context: SandboxContext) {
    this.#transport = transport;
    this.#context = context;
  }

  async create(template = "default", options: CreateSandboxOptions = {}): Promise<Sandbox> {
    const payload = await this.#transport.request("POST", "/sandboxes", {
      json: createBody(template, options),
      headers: { "Idempotency-Key": options.idempotencyKey ?? randomUUID() },
    });
    const [info, connection] = sandboxPayload(payload);
    return new Sandbox(this.#transport, info, connection, this.#context);
  }

  async connect(sandboxId: string, timeout = 300): Promise<Sandbox> {
    const payload = await this.#transport.request(
      "POST",
      `/sandboxes/${identifier(sandboxId)}/connect`,
      {
        json: { timeout: checkedTimeout(timeout) },
      },
    );
    const [info, connection] = sandboxPayload(payload);
    return new Sandbox(this.#transport, info, connection, this.#context);
  }

  async get(sandboxId: string): Promise<SandboxInfo> {
    return parseSandboxInfo(
      objectValue(await this.#transport.request("GET", `/sandboxes/${identifier(sandboxId)}`)),
    );
  }

  async list(options: ListSandboxesOptions = {}): Promise<Page<SandboxInfo>> {
    const params: Record<string, string | number> = {};
    if (options.metadata) params.metadata = options.metadata;
    if (options.states?.length) params.state = options.states.join(",");
    if (options.limit !== undefined) {
      if (!Number.isInteger(options.limit) || options.limit < 1 || options.limit > 100)
        throw new RangeError("limit must be between 1 and 100");
      params.limit = options.limit;
    }
    if (options.nextToken) params.nextToken = options.nextToken;
    const { body, headers } = await this.#transport.requestWithHeaders("GET", "/v2/sandboxes", {
      params,
    });
    const total = headers.get("x-total-running");
    return {
      items: parseObjectItems(body).map(parseSandboxInfo),
      nextToken: headers.get("x-next-token") ?? undefined,
      total: total ? responseInteger(total, "X-Total-Running") : undefined,
    };
  }

  async metrics(sandboxIds: string[]): Promise<Record<string, SandboxMetrics>> {
    const ids = [...new Set(sandboxIds.filter(Boolean))];
    if (ids.length < 1 || ids.length > 100)
      throw new RangeError("sandboxIds must contain between 1 and 100 unique IDs");
    const payload = objectValue(
      await this.#transport.request("GET", "/sandboxes/metrics", {
        params: { sandbox_ids: ids.join(",") },
      }),
    );
    const values = objectValue(payload.sandboxes ?? {});
    return Object.fromEntries(
      Object.entries(values).map(([id, value]) => [id, parseMetrics(objectValue(value))]),
    );
  }
}

export class Snapshots {
  readonly #transport: Transport;

  constructor(transport: Transport) {
    this.#transport = transport;
  }

  async list(options: ListSnapshotsOptions = {}): Promise<Page<SnapshotInfo>> {
    const params = {
      sandboxID: options.sandboxId,
      name: options.name,
      limit: options.limit,
      nextToken: options.nextToken,
    };
    const { body, headers } = await this.#transport.requestWithHeaders("GET", "/snapshots", {
      params,
    });
    return {
      items: parseObjectItems(body).map(parseSnapshot),
      nextToken: headers.get("x-next-token") ?? undefined,
    };
  }
}

export class Sandbox {
  readonly commands: Commands;
  readonly files: Filesystem;
  readonly pty: Pty;
  readonly git: Git;
  readonly #control: Transport;
  readonly #context: SandboxContext;
  #info: SandboxInfo;
  #connection: SandboxConnection;
  #gateway?: Transport;

  constructor(
    control: Transport,
    info: SandboxInfo,
    connection: SandboxConnection,
    context: SandboxContext,
  ) {
    this.#control = control;
    this.#info = info;
    this.#connection = connection;
    this.#context = context;
    this.commands = new Commands(() => this.#gatewayTransport());
    this.files = new Filesystem(() => this.#gatewayTransport());
    this.pty = new Pty(this.commands, () => this.#gatewayTransport());
    this.git = new Git(this.commands);
  }

  static async create(
    template = "default",
    options: CreateSandboxOptions & DevBoxOptions = {},
  ): Promise<Sandbox> {
    const config = resolveConfig(options);
    const transport = controlTransport(config);
    try {
      const sandbox = await new Sandboxes(transport, contextFrom(config, true)).create(
        template,
        options,
      );
      return sandbox;
    } catch (error) {
      await transport.close();
      throw error;
    }
  }

  static async connect(
    sandboxId: string,
    options: DevBoxOptions & { timeout?: number } = {},
  ): Promise<Sandbox> {
    const config = resolveConfig(options);
    const transport = controlTransport(config);
    try {
      return await new Sandboxes(transport, contextFrom(config, true)).connect(
        sandboxId,
        options.timeout ?? 300,
      );
    } catch (error) {
      await transport.close();
      throw error;
    }
  }

  get sandboxId(): string {
    return this.#info.sandboxId;
  }

  get info(): SandboxInfo {
    return this.#info;
  }

  async getInfo(): Promise<SandboxInfo> {
    this.#info = parseSandboxInfo(
      objectValue(await this.#control.request("GET", `/sandboxes/${identifier(this.sandboxId)}`)),
    );
    return this.#info;
  }

  async isRunning(): Promise<boolean> {
    try {
      return (await this.getInfo()).state === SandboxState.Running;
    } catch (error) {
      if (!(error instanceof NotFoundError)) throw error;
      this.#info = { ...this.#info, state: SandboxState.Stopped };
      return false;
    }
  }

  async setTimeout(timeout: number): Promise<void> {
    await this.#control.request("POST", `/sandboxes/${identifier(this.sandboxId)}/timeout`, {
      json: { timeout: checkedTimeout(timeout) },
    });
  }

  async refresh(duration = 300): Promise<void> {
    await this.#control.request("POST", `/sandboxes/${identifier(this.sandboxId)}/refreshes`, {
      json: { duration: checkedTimeout(duration) },
    });
  }

  async kill(): Promise<boolean> {
    try {
      await this.#control.request("DELETE", `/sandboxes/${identifier(this.sandboxId)}`);
    } catch (error) {
      if (!(error instanceof NotFoundError)) throw error;
      this.#info = { ...this.#info, state: SandboxState.Stopped };
      return false;
    } finally {
      await this.#closeGateway();
    }
    this.#info = { ...this.#info, state: SandboxState.Stopped };
    return true;
  }

  async snapshot(name?: string): Promise<SnapshotInfo> {
    return parseSnapshot(
      objectValue(
        await this.#control.request("POST", `/sandboxes/${identifier(this.sandboxId)}/snapshots`, {
          json: name ? { name } : {},
        }),
      ),
    );
  }

  async fork(options: { timeout?: number; count?: number } = {}): Promise<SandboxForkResult[]> {
    const count = options.count ?? 1;
    if (!Number.isInteger(count) || count < 1 || count > 100)
      throw new RangeError("count must be between 1 and 100");
    const payload = await this.#control.request(
      "POST",
      `/sandboxes/${identifier(this.sandboxId)}/fork`,
      {
        json: { timeout: checkedTimeout(options.timeout ?? 300), count },
      },
    );
    return parseObjectItems(payload).map((item) => this.#forkResult(item));
  }

  async getLogs(
    options: {
      cursor?: number;
      limit?: number;
      direction?: LogsDirection;
      level?: LogLevel;
      search?: string;
    } = {},
  ): Promise<SandboxLogEntry[]> {
    const limit = options.limit ?? 1000;
    if (!Number.isInteger(limit) || limit < 0 || limit > 1000)
      throw new RangeError("limit must be between 0 and 1000");
    if (options.search && options.search.length > 256)
      throw new RangeError("search must not exceed 256 characters");
    const payload = objectValue(
      await this.#control.request("GET", `/v2/sandboxes/${identifier(this.sandboxId)}/logs`, {
        params: {
          cursor: options.cursor,
          limit,
          direction: options.direction,
          level: options.level,
          search: options.search,
        },
      }),
    );
    return parseObjectItems(payload.logs ?? []).map(parseLogEntry);
  }

  async getMetrics(
    options: { start?: number | Date; end?: number | Date } = {},
  ): Promise<SandboxMetrics[]> {
    const payload = await this.#control.request(
      "GET",
      `/sandboxes/${identifier(this.sandboxId)}/metrics`,
      {
        params: { start: metricTimestamp(options.start), end: metricTimestamp(options.end) },
      },
    );
    return parseObjectItems(payload).map(parseMetrics);
  }

  async updateNetwork(network: NetworkConfig): Promise<void> {
    await this.#control.request("PUT", `/sandboxes/${identifier(this.sandboxId)}/network`, {
      json: networkUpdateWire(network),
    });
  }

  async close(): Promise<void> {
    await this.#closeGateway();
    if (this.#context.ownsControl) await this.#control.close();
  }

  async #gatewayTransport(): Promise<Transport> {
    if (this.#connection.expiresAt && this.#connection.expiresAt.getTime() <= Date.now() + 30_000) {
      await this.#refreshConnection();
    }
    if (!this.#gateway) {
      const url = this.#context.gatewayUrl ?? this.#connection.gatewayUrl;
      if (!url) throw new ProtocolError("sandbox response does not provide an EnvD endpoint");
      if (url.replace(/^https:\/\//, "").endsWith(".sandbox.devbox.local")) {
        throw new ProtocolError("Manager returned a placeholder EnvD endpoint");
      }
      this.#gateway = new Transport(url, {
        headers: {
          "X-Access-Token": this.#connection.accessToken,
          "E2B-Sandbox-Id": this.sandboxId,
        },
        timeoutMs: this.#context.requestTimeoutMs,
        dispatcher: this.#context.dispatcher,
      });
    }
    return this.#gateway;
  }

  async #refreshConnection(): Promise<void> {
    const payload = await this.#control.request(
      "POST",
      `/sandboxes/${identifier(this.sandboxId)}/connect`,
      { json: { timeout: 300 } },
    );
    [this.#info, this.#connection] = sandboxPayload(payload);
    await this.#closeGateway();
  }

  async #closeGateway(): Promise<void> {
    if (this.#gateway) await this.#gateway.close();
    this.#gateway = undefined;
  }

  #forkResult(value: WireObject): SandboxForkResult {
    let sandbox: Sandbox | undefined;
    if (value.sandbox && typeof value.sandbox === "object") {
      const [info, connection] = sandboxPayload(value.sandbox);
      sandbox = new Sandbox(this.#control, info, connection, this.#context);
    }
    let error: ErrorDetail | undefined;
    if (value.error && typeof value.error === "object") {
      const detail = objectValue(value.error);
      error = {
        code: String(detail.error ?? detail.code ?? ""),
        message: String(detail.message ?? ""),
      };
    }
    return { sandbox, error };
  }
}

function createBody(template: string, options: CreateSandboxOptions): WireObject {
  if (!template.trim()) throw new TypeError("template must not be blank");
  const network = options.network ?? {};
  const body: WireObject = {
    templateID: template,
    timeout: checkedTimeout(options.timeout ?? 300),
    secure: options.secure ?? true,
    allow_internet_access: network.allowInternetAccess ?? true,
    network: networkCreateWire(network),
    metadata: { ...options.metadata },
    envVars: { ...options.envs },
    volumeMounts: (options.volumeMounts ?? []).map((mount) => ({
      name: mount.name,
      path: mount.path,
    })),
  };
  if (options.clientId) body.clientID = options.clientId;
  if (options.buildId) body.buildID = options.buildId;
  return body;
}

function sandboxPayload(value: unknown): [SandboxInfo, SandboxConnection] {
  const payload = objectValue(value);
  const sandbox =
    payload.sandbox && typeof payload.sandbox === "object" ? objectValue(payload.sandbox) : payload;
  const info = parseSandboxInfo(sandbox);
  const connectionValue = payload.connection ?? sandbox.connection ?? sandbox;
  return [info, parseConnection(objectValue(connectionValue), info.sandboxId)];
}

function controlTransport(config: ConnectionConfig): Transport {
  return new Transport(config.apiUrl, {
    headers: { ...config.headers, "X-API-Key": config.apiKey },
    timeoutMs: config.requestTimeoutMs,
    dispatcher: config.dispatcher,
  });
}

function contextFrom(config: ConnectionConfig, ownsControl = false): SandboxContext {
  return {
    requestTimeoutMs: config.requestTimeoutMs,
    gatewayUrl: config.gatewayUrl,
    dispatcher: config.dispatcher,
    ownsControl,
  };
}

function metricTimestamp(value?: number | Date): number | undefined {
  if (value === undefined) return undefined;
  return value instanceof Date ? Math.floor(value.getTime() / 1000) : value;
}

function responseInteger(value: string, field: string): number {
  const parsed = numberValue(value);
  if (!Number.isInteger(parsed))
    throw new ProtocolError(`DevBox response header is not an integer: ${field}`);
  return parsed;
}

export { contextFrom, controlTransport };
