import type { Dispatcher } from "undici";
import { Commands } from "./commands.js";
import { type ConnectionConfig, type DevBoxOptions, resolveConfig } from "./config.js";
import { Envd } from "./envd.js";
import { ConflictError, NotFoundError, ProtocolError } from "./errors.js";
import { Filesystem } from "./filesystem.js";
import { Git } from "./git.js";
import { Transport } from "./internal/transport.js";
import {
  checkedTimeout,
  identifier,
  numberValue,
  objectItems,
  objectValue,
  type WireObject,
} from "./internal/wire.js";
import {
  type Page,
  parseConnection,
  parseSandboxInfo,
  type SandboxConnection,
  type SandboxInfo,
  type SandboxLifecycle,
  SandboxState,
} from "./models.js";
import { Pty } from "./pty.js";

const TOKEN_REFRESH_WINDOW_SECONDS = 300;
const CONNECT_TOKEN_REFRESHED_HEADER = "X-Devbox-Connect-Token-Refreshed";
const CONNECT_TOKEN_EXPIRATION_HEADER = "X-Devbox-Connect-Token-Expiration";

export interface CreateSandboxOptions {
  timeout?: number;
  envs?: Readonly<Record<string, string>>;
  metadata?: Readonly<Record<string, string>>;
  buildId?: string;
  lifecycle?: SandboxLifecycle;
  network?: { allowPublicTraffic?: boolean };
}

export interface ConnectSandboxOptions {
  timeout?: number;
}

export interface PauseSandboxOptions {
  memory?: boolean;
  mode?: "full" | "filesystem";
}

export interface ListSandboxesOptions {
  template?: string;
  metadata?: string;
  states?: readonly SandboxState[];
  limit?: number;
  nextToken?: string;
  startedAfter?: string;
  order?: "asc" | "desc";
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
    });
    const [info, connection] = sandboxPayload(payload);
    return Sandbox.fromConnection(this.#transport, info, connection, this.#context);
  }

  async connect(sandboxId: string, options: ConnectSandboxOptions = {}): Promise<Sandbox> {
    const payload = await this.#transport.request(
      "POST",
      `/sandboxes/${identifier(sandboxId)}/connect`,
      {
        json: { timeout: checkedTimeout(options.timeout ?? 300) },
      },
    );
    const [info, connection] = sandboxPayload(payload);
    return Sandbox.fromConnection(this.#transport, info, connection, this.#context);
  }

  async get(sandboxId: string): Promise<SandboxInfo> {
    return parseSandboxInfo(
      objectValue(await this.#transport.request("GET", `/sandboxes/${identifier(sandboxId)}`)),
    );
  }

  async list(options: ListSandboxesOptions = {}): Promise<Page<SandboxInfo>> {
    const params: Record<string, string | number> = {};
    if (options.template) params.template = options.template;
    if (options.metadata) params.metadata = options.metadata;
    if (options.states?.length) params.state = options.states.join(",");
    if (options.limit !== undefined) {
      if (!Number.isInteger(options.limit) || options.limit < 0 || options.limit > 1000)
        throw new RangeError("limit must be between 0 and 1000 (0 means no limit)");
      params.limit = options.limit;
    }
    if (options.nextToken) params.nextToken = options.nextToken;
    if (options.startedAfter) params.startedAfter = options.startedAfter;
    if (options.order !== undefined) {
      if (options.order !== "asc" && options.order !== "desc")
        throw new RangeError("order must be asc or desc");
      params.order = options.order;
    }
    const { body, headers } = await this.#transport.requestWithHeaders("GET", "/sandboxes", {
      params,
    });
    const total = headers.get("x-total-running");
    return {
      items: objectItems(body).map(parseSandboxInfo),
      nextToken: headers.get("x-next-token") ?? undefined,
      total: total ? responseInteger(total, "X-Total-Running") : undefined,
    };
  }
}

export class Sandbox {
  readonly commands: Commands;
  readonly files: Filesystem;
  readonly pty: Pty;
  readonly git: Git;
  readonly envd: Envd;
  readonly #control: Transport;
  readonly #context: SandboxContext;
  #info: SandboxInfo;
  #connection: SandboxConnection;
  #gateway?: Transport;

  private constructor(
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
    this.envd = new Envd(() => this.#gatewayTransport());
  }

  /** @internal */
  static fromConnection(
    control: Transport,
    info: SandboxInfo,
    connection: SandboxConnection,
    context: SandboxContext,
  ): Sandbox {
    return new Sandbox(control, info, connection, context);
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
    options: DevBoxOptions & ConnectSandboxOptions = {},
  ): Promise<Sandbox> {
    const config = resolveConfig(options);
    const transport = controlTransport(config);
    try {
      return await new Sandboxes(transport, contextFrom(config, true)).connect(sandboxId, {
        timeout: options.timeout,
      });
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

  /** Extend expiration to at least now plus duration; never shorten it. */
  async refresh(duration = 300): Promise<void> {
    await this.#control.request("POST", `/sandboxes/${identifier(this.sandboxId)}/refreshes`, {
      json: { duration: checkedTimeout(duration) },
    });
  }

  /** Save memory and disk. Paused sandbox records do not expire in Manager. */
  async pause(options: PauseSandboxOptions = {}): Promise<void> {
    if (options.memory === false || options.mode === "filesystem")
      throw new RangeError("Only full memory snapshots are supported");
    if (options.memory !== undefined && typeof options.memory !== "boolean")
      throw new TypeError("memory must be a boolean");
    if (options.mode !== undefined && options.mode !== "full")
      throw new RangeError("mode must be full");
    await this.#control.request("POST", `/sandboxes/${identifier(this.sandboxId)}/pause`, {
      json:
        options.memory !== undefined || options.mode !== undefined ? { memory: true } : undefined,
    });
    this.#info = { ...this.#info, state: SandboxState.Paused };
    await this.#closeGateway();
  }

  /** Resume this sandbox and replace the handle's previous connect token. */
  async resume(options: ConnectSandboxOptions = {}): Promise<this> {
    const payload = await this.#control.request(
      "POST",
      `/sandboxes/${identifier(this.sandboxId)}/resume`,
      {
        json: { timeout: checkedTimeout(options.timeout ?? 300) },
      },
    );
    const [info, connection] = sandboxPayload(payload);
    await this.#closeGateway();
    this.#info = info;
    this.#connection = connection;
    return this;
  }

  async kill(): Promise<boolean> {
    try {
      await this.#control.request("DELETE", `/sandboxes/${identifier(this.sandboxId)}`);
    } catch (error) {
      if (
        !(error instanceof NotFoundError) &&
        !(error instanceof ConflictError && error.code === "already_killed")
      )
        throw error;
      this.#info = { ...this.#info, state: SandboxState.Stopped };
      return false;
    } finally {
      await this.#closeGateway();
    }
    this.#info = { ...this.#info, state: SandboxState.Stopped };
    return true;
  }

  async close(): Promise<void> {
    await this.#closeGateway();
    if (this.#context.ownsControl) await this.#control.close();
  }

  async #gatewayTransport(): Promise<Transport> {
    if (this.#info.state === SandboxState.Paused && !this.#info.lifecycle?.autoResume) {
      throw new ConflictError("sandbox is paused; call resume() or Sandbox.connect() first");
    }
    await this.#refreshConnectionIfNeeded();
    if (!this.#gateway) {
      const url = gatewayUrl(this.#connection, this.#context.gatewayUrl);
      if (!url) throw new ProtocolError("sandbox response does not provide an EnvD endpoint");
      if (url.replace(/^https:\/\//, "").endsWith(".sandbox.devbox.local")) {
        throw new ProtocolError("Manager returned a placeholder EnvD endpoint");
      }
      this.#gateway = new Transport(url, {
        headers: gatewayHeaders(this.#connection),
        timeoutMs: this.#context.requestTimeoutMs,
        dispatcher: this.#context.dispatcher,
        onResponse: (headers) => this.#handleRefreshedConnectToken(headers),
      });
    }
    return this.#gateway;
  }

  async #refreshConnectionIfNeeded(): Promise<void> {
    if (!tokenNeedsRefresh(this.#connection)) return;
    const oldUrl = gatewayUrl(this.#connection, this.#context.gatewayUrl);
    const payload = await this.#control.request(
      "POST",
      `/sandboxes/${identifier(this.sandboxId)}/connect`,
      { json: { resume: this.#info.lifecycle?.autoResume ?? false } },
    );
    const [info, connection] = sandboxPayload(payload);
    this.#info = info;
    this.#connection = connection;
    if (this.#gateway) {
      if (oldUrl === gatewayUrl(connection, this.#context.gatewayUrl)) {
        this.#gateway.updateHeaders(gatewayHeaders(connection));
      } else {
        await this.#closeGateway();
      }
    }
  }

  async #closeGateway(): Promise<void> {
    if (this.#gateway) await this.#gateway.close();
    this.#gateway = undefined;
  }

  #handleRefreshedConnectToken(headers: Headers): void {
    const connectToken = headers.get(CONNECT_TOKEN_REFRESHED_HEADER);
    if (!connectToken) return;
    const rawExpiration = headers.get(CONNECT_TOKEN_EXPIRATION_HEADER);
    if (rawExpiration === null) {
      throw new ProtocolError("gateway refreshed token without an expiration");
    }
    const tokenExpiration = Number(rawExpiration);
    if (!Number.isSafeInteger(tokenExpiration) || tokenExpiration <= 0) {
      throw new ProtocolError("gateway refreshed token expiration is invalid");
    }
    this.#connection = { ...this.#connection, connectToken, tokenExpiration };
    this.#gateway?.updateHeaders(gatewayHeaders(this.#connection));
  }
}

function gatewayUrl(connection: SandboxConnection, configuredUrl?: string): string {
  if (!configuredUrl) return connection.domain;
  if (configuredUrl.includes("{tunnel_id}") && !connection.tunnelId)
    throw new ProtocolError("sandbox response does not provide a tunnel ID");
  return configuredUrl.replaceAll("{tunnel_id}", connection.tunnelId).replaceAll("{port}", "49983");
}

function gatewayHeaders(connection: SandboxConnection): Record<string, string> {
  if (!connection.connectToken || /[^A-Za-z0-9._-]/.test(connection.connectToken)) {
    throw new ProtocolError("sandbox response does not provide a valid connectToken");
  }
  if (connection.envdAccessToken && /[^A-Za-z0-9._-]/.test(connection.envdAccessToken))
    throw new ProtocolError("sandbox response does not provide a valid envdAccessToken");
  return {
    Cookie: `relay_token=${connection.connectToken}`,
    ...(connection.envdAccessToken ? { "X-Access-Token": connection.envdAccessToken } : {}),
    "E2B-Sandbox-Id": connection.sandboxId,
    "E2B-Sandbox-Port": "49983",
  };
}

function tokenNeedsRefresh(connection: SandboxConnection): boolean {
  return (
    connection.tokenExpiration !== undefined &&
    connection.tokenExpiration <= Math.floor(Date.now() / 1000) + TOKEN_REFRESH_WINDOW_SECONDS
  );
}

function createBody(template: string, options: CreateSandboxOptions): WireObject {
  if (!template.trim()) throw new TypeError("template must not be blank");
  const body: WireObject = {
    templateID: template,
    timeout: checkedTimeout(options.timeout ?? 300),
    metadata: { ...options.metadata },
    envVars: { ...options.envs },
  };
  if (options.lifecycle) {
    const onTimeout = options.lifecycle.onTimeout ?? "kill";
    if (options.lifecycle.autoResume && onTimeout !== "pause")
      throw new RangeError("lifecycle.autoResume requires onTimeout='pause'");
    body.autoPause = onTimeout === "pause";
    body.autoResume = { enabled: options.lifecycle.autoResume ?? false };
    body.lifecycle = { onTimeout, autoResume: options.lifecycle.autoResume ?? false };
  }
  if (options.buildId) body.buildID = options.buildId;
  if (options.network !== undefined) {
    const network = options.network;
    if (
      !network ||
      typeof network !== "object" ||
      Array.isArray(network) ||
      Object.keys(network).some((key) => key !== "allowPublicTraffic")
    )
      throw new TypeError("network only supports allowPublicTraffic");
    if (network.allowPublicTraffic !== undefined && typeof network.allowPublicTraffic !== "boolean")
      throw new TypeError("network.allowPublicTraffic must be a boolean");
    body.network = { ...network };
  }
  return body;
}

function sandboxPayload(value: unknown): [SandboxInfo, SandboxConnection] {
  const payload = objectValue(value);
  const info = parseSandboxInfo(payload);
  return [info, parseConnection(payload, info.sandboxId)];
}

function controlTransport(config: ConnectionConfig): Transport {
  return new Transport(config.apiUrl, {
    headers: { ...config.headers, "X-API-Key": config.apiKey },
    timeoutMs: config.controlTimeoutMs,
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

function responseInteger(value: string, field: string): number {
  const parsed = numberValue(value);
  if (!Number.isInteger(parsed))
    throw new ProtocolError(`DevBox response header is not an integer: ${field}`);
  return parsed;
}

export { contextFrom, controlTransport };
