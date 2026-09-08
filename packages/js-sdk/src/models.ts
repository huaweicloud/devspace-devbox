import { ProtocolError } from "./errors.js";
import {
  dateValue,
  numberValue,
  objectItems,
  objectValue,
  optionalDate,
  optionalNumber,
  optionalString,
  pick,
  pickOr,
  stringArray,
  stringRecord,
  stringValue,
  type WireObject,
} from "./internal/wire.js";

export const SandboxState = {
  Creating: "creating",
  Running: "running",
  Pausing: "pausing",
  Paused: "paused",
  Resuming: "resuming",
  Stopping: "stopping",
  Stopped: "stopped",
  Failed: "failed",
} as const;
export type SandboxState = (typeof SandboxState)[keyof typeof SandboxState];

export const FileType = { File: "file", Directory: "directory", Symlink: "symlink" } as const;
export type FileType = (typeof FileType)[keyof typeof FileType];

export const LogLevel = {
  Error: "ERROR",
  Warning: "WARNING",
  Info: "INFO",
  Debug: "DEBUG",
  Trace: "TRACE",
} as const;
export type LogLevel = (typeof LogLevel)[keyof typeof LogLevel];

export const LogsDirection = { Backward: "backward", Forward: "forward" } as const;
export type LogsDirection = (typeof LogsDirection)[keyof typeof LogsDirection];

export const NodeStatus = { Ready: "ready", Draining: "draining", Offline: "offline" } as const;
export type NodeStatus = (typeof NodeStatus)[keyof typeof NodeStatus];

export interface NetworkRule {
  headers?: Record<string, string>;
}

export interface NetworkConfig {
  allowInternetAccess?: boolean;
  allowPublicTraffic?: boolean;
  allowOut?: string[];
  denyOut?: string[];
  maskRequestHost?: string;
  rules?: Record<string, NetworkRule[]>;
}

export interface VolumeMount {
  name: string;
  path: string;
}

export interface SandboxLifecycle {
  autoResume: boolean;
  onTimeout: string;
}

export interface SandboxInfo {
  sandboxId: string;
  templateId: string;
  state: SandboxState;
  clientId: string;
  alias?: string;
  startedAt?: Date;
  endAt?: Date;
  envdVersion: string;
  cpuCount?: number;
  memoryMb?: number;
  diskSizeMb?: number;
  metadata: Record<string, string>;
  network: Required<
    Pick<
      NetworkConfig,
      "allowInternetAccess" | "allowPublicTraffic" | "allowOut" | "denyOut" | "rules"
    >
  > &
    Pick<NetworkConfig, "maskRequestHost">;
  lifecycle?: SandboxLifecycle;
  volumeMounts: VolumeMount[];
}

export interface SandboxConnection {
  sandboxId: string;
  gatewayUrl: string;
  accessToken: string;
  expiresAt?: Date;
  protocolVersion: string;
}

export interface SandboxMetrics {
  timestampUnix: number;
  cpuCount: number;
  cpuUsedPercent: number;
  memoryUsedBytes: number;
  memoryTotalBytes: number;
  memoryCacheBytes: number;
  diskUsedBytes: number;
  diskTotalBytes: number;
  timestamp?: Date;
}

export interface SnapshotInfo {
  snapshotId: string;
  names: string[];
}

export interface SandboxLogEntry {
  timestamp: Date;
  level: LogLevel;
  message: string;
  fields: Record<string, string>;
}

export interface ProcessInfo {
  pid: number;
  command: string;
  running: boolean;
  startedAt?: Date;
}

export interface OutputChunk {
  stream: "stdout" | "stderr";
  data: string;
  timestamp?: Date;
}

export interface CommandResult {
  exitCode: number;
  stdout: string;
  stderr: string;
  pid?: number;
}

export interface PtySize {
  rows: number;
  cols: number;
}

export interface FileInfo {
  name: string;
  path: string;
  type: FileType;
  size: number;
  mode?: number;
  permissions?: string;
  owner?: string;
  group?: string;
  modifiedAt?: Date;
  symlinkTarget?: string;
}

export interface FileWatchEvent {
  name: string;
  type: string;
  path?: string;
  entry?: FileInfo;
}

export interface TemplateInput {
  alias: string;
  name: string;
  public?: boolean;
  vcpu?: number;
  ramMb?: number;
  totalDiskMb?: number;
  startCommand?: string;
}

export interface TemplateInfo {
  templateId: string;
  namespace: string;
  name?: string;
  public: boolean;
  createdBy?: string;
  spawnCount: number;
  createdAt?: Date;
}

export interface TemplateBuildInfo {
  templateId: string;
  buildId: string;
  namespace?: string;
  status?: string;
  tag?: string;
  vcpu?: number;
  ramMb?: number;
  totalDiskMb?: number;
  kernelVersion?: string;
  firecrackerVersion?: string;
  reason?: string;
  createdAt?: Date;
}

export interface TemplateDetail {
  template: TemplateInfo;
  builds: TemplateBuildInfo[];
}

export interface TemplateAliasInfo {
  alias: string;
  templateId: string;
  namespace: string;
}

export interface TemplateFileInfo {
  exists: boolean;
  uploadUrl?: string;
}

export interface NodeInfo {
  nodeId: string;
  nodeName?: string;
  clusterId?: string;
  ipAddress?: string;
  cpuTotal?: number;
  cpuFree?: number;
  ramTotalMb?: number;
  ramFreeMb?: number;
  diskTotalMb?: number;
  diskFreeMb?: number;
  currentSandboxCount?: number;
  status?: string;
}

export interface Page<T> {
  items: T[];
  nextToken?: string;
  total?: number;
}

export function networkCreateWire(config: NetworkConfig = {}): WireObject {
  const value: WireObject = {
    allowPublicTraffic: config.allowPublicTraffic ?? false,
    allowOut: config.allowOut ?? [],
    denyOut: config.denyOut ?? [],
    rules: rulesToWire(config.rules ?? {}),
  };
  if (config.maskRequestHost !== undefined) value.maskRequestHost = config.maskRequestHost;
  return value;
}

export function networkUpdateWire(config: NetworkConfig): WireObject {
  return {
    allowOut: config.allowOut ?? [],
    denyOut: config.denyOut ?? [],
    rules: rulesToWire(config.rules ?? {}),
    allow_internet_access: config.allowInternetAccess ?? true,
  };
}

export function parseSandboxInfo(value: WireObject): SandboxInfo {
  const state = stringValue(pickOr(value, "running", "state", "status"));
  if (!Object.values(SandboxState).includes(state as SandboxState)) {
    throw new ProtocolError("DevBox returned an invalid SandboxState");
  }
  const network =
    value.network && typeof value.network === "object" ? objectValue(value.network) : {};
  const lifecycle =
    value.lifecycle && typeof value.lifecycle === "object"
      ? objectValue(value.lifecycle)
      : undefined;
  return {
    sandboxId: stringValue(pick(value, "sandboxId", "sandboxID", "sandbox_id", "id")),
    templateId: stringValue(pickOr(value, "default", "templateId", "templateID", "template_id")),
    state: state as SandboxState,
    clientId: stringValue(pickOr(value, "", "clientID", "client_id")),
    alias: optionalString(value.alias),
    startedAt: optionalDate(
      pickOr(value, undefined, "createdAt", "created_at", "startedAt", "started_at"),
    ),
    endAt: optionalDate(pickOr(value, undefined, "expiresAt", "expires_at", "endAt", "end_at")),
    envdVersion: stringValue(pickOr(value, "", "envdVersion", "envd_version")),
    cpuCount: optionalNumber(value.cpuCount),
    memoryMb: optionalNumber(value.memoryMB),
    diskSizeMb: optionalNumber(value.diskSizeMB),
    metadata: stringRecord(value.metadata),
    network: {
      allowInternetAccess: Boolean(value.allowInternetAccess ?? true),
      allowPublicTraffic: Boolean(network.allowPublicTraffic ?? true),
      allowOut: stringArray(network.allowOut),
      denyOut: stringArray(network.denyOut),
      maskRequestHost: optionalString(network.maskRequestHost),
      rules: rulesFromWire(network.rules),
    },
    lifecycle: lifecycle
      ? {
          autoResume: Boolean(lifecycle.autoResume ?? false),
          onTimeout: String(lifecycle.onTimeout ?? "kill"),
        }
      : undefined,
    volumeMounts: Array.isArray(value.volumeMounts)
      ? value.volumeMounts
          .map((item) => objectValue(item))
          .map((item) => ({
            name: stringValue(pick(item, "name")),
            path: stringValue(pick(item, "path")),
          }))
      : [],
  };
}

export function parseConnection(value: WireObject, sandboxId: string): SandboxConnection {
  let gatewayUrl = stringValue(
    pickOr(value, "", "gatewayUrl", "gateway_url", "envdUrl", "envd_url", "domain"),
  );
  if (gatewayUrl && !gatewayUrl.startsWith("http://") && !gatewayUrl.startsWith("https://"))
    gatewayUrl = `https://${gatewayUrl}`;
  return {
    sandboxId,
    gatewayUrl,
    accessToken: stringValue(
      pickOr(
        value,
        "",
        "accessToken",
        "access_token",
        "envdAccessToken",
        "envd_access_token",
        "token",
      ),
    ),
    expiresAt: optionalDate(pickOr(value, undefined, "expiresAt", "expires_at")),
    protocolVersion: stringValue(
      pickOr(value, "v1", "protocolVersion", "protocol_version", "envdVersion", "envd_version"),
    ),
  };
}

export function parseMetrics(value: WireObject): SandboxMetrics {
  return {
    timestampUnix: numberValue(pick(value, "timestampUnix")),
    cpuCount: numberValue(pick(value, "cpuCount")),
    cpuUsedPercent: numberValue(pick(value, "cpuUsedPct")),
    memoryUsedBytes: numberValue(pick(value, "memUsed")),
    memoryTotalBytes: numberValue(pick(value, "memTotal")),
    memoryCacheBytes: numberValue(pick(value, "memCache")),
    diskUsedBytes: numberValue(pick(value, "diskUsed")),
    diskTotalBytes: numberValue(pick(value, "diskTotal")),
    timestamp: optionalDate(value.timestamp),
  };
}

export function parseSnapshot(value: WireObject): SnapshotInfo {
  return {
    snapshotId: stringValue(pick(value, "snapshotID", "snapshotId", "snapshot_id", "id")),
    names: stringArray(value.names),
  };
}

export function parseLogEntry(value: WireObject): SandboxLogEntry {
  const level = stringValue(pick(value, "level"));
  if (!Object.values(LogLevel).includes(level as LogLevel))
    throw new ProtocolError("DevBox returned an invalid LogLevel");
  return {
    timestamp: dateValue(pick(value, "timestamp")),
    level: level as LogLevel,
    message: stringValue(pick(value, "message")),
    fields: stringRecord(value.fields),
  };
}

export function parseProcessInfo(value: WireObject): ProcessInfo {
  const process =
    value.config && typeof value.config === "object" ? objectValue(value.config) : value;
  const command = [String(pickOr(process, "", "command", "cmd")), ...stringArray(process.args)]
    .join(" ")
    .trim();
  return {
    pid: numberValue(pick(value, "pid")),
    command,
    running: Boolean(pickOr(value, true, "running")),
    startedAt: optionalDate(pickOr(value, undefined, "startedAt", "started_at")),
  };
}

export function parseFileInfo(value: WireObject): FileInfo {
  const rawType = stringValue(pickOr(value, "file", "type"));
  const type =
    (
      {
        FILE_TYPE_FILE: "file",
        FILE_TYPE_DIRECTORY: "directory",
        FILE_TYPE_SYMLINK: "symlink",
      } as Record<string, FileType>
    )[rawType] ?? rawType;
  if (!Object.values(FileType).includes(type as FileType))
    throw new ProtocolError("DevBox returned an invalid FileType");
  return {
    name: stringValue(pick(value, "name")),
    path: stringValue(pick(value, "path")),
    type: type as FileType,
    size: numberValue(pickOr(value, 0, "size")),
    mode: optionalNumber(value.mode),
    permissions: optionalString(value.permissions),
    owner: optionalString(value.owner),
    group: optionalString(value.group),
    modifiedAt: optionalDate(pickOr(value, undefined, "modifiedTime", "modifiedAt", "modified_at")),
    symlinkTarget: optionalString(pickOr(value, undefined, "symlinkTarget", "symlink_target")),
  };
}

export function parseTemplateInfo(value: WireObject): TemplateInfo {
  return {
    templateId: stringValue(pick(value, "template_id", "templateID", "templateId", "id")),
    namespace: stringValue(pick(value, "namespace")),
    name: optionalString(value.name),
    public: Boolean(value.public ?? false),
    createdBy: optionalString(value.created_by),
    spawnCount: numberValue(value.spawn_count ?? 0),
    createdAt: optionalDate(value.created_at),
  };
}

export function parseTemplateBuild(value: WireObject): TemplateBuildInfo {
  return {
    templateId: stringValue(pick(value, "template_id", "templateID", "templateId")),
    buildId: stringValue(pick(value, "build_id", "buildID", "buildId", "id")),
    namespace: optionalString(value.namespace),
    status: optionalString(value.status),
    tag: optionalString(value.tag),
    vcpu: optionalNumber(value.vcpu),
    ramMb: optionalNumber(value.ram_mb),
    totalDiskMb: optionalNumber(value.total_disk_mb),
    kernelVersion: optionalString(value.kernel_version),
    firecrackerVersion: optionalString(value.firecracker_version),
    reason: optionalString(value.reason),
    createdAt: optionalDate(value.created_at),
  };
}

export function parseNode(value: WireObject): NodeInfo {
  return {
    nodeId: stringValue(pick(value, "node_id")),
    nodeName: optionalString(value.node_name),
    clusterId: optionalString(value.cluster_id),
    ipAddress: optionalString(value.ip_address),
    cpuTotal: optionalNumber(value.cpu_total),
    cpuFree: optionalNumber(value.cpu_free),
    ramTotalMb: optionalNumber(value.ram_total_mb),
    ramFreeMb: optionalNumber(value.ram_free_mb),
    diskTotalMb: optionalNumber(value.disk_total_mb),
    diskFreeMb: optionalNumber(value.disk_free_mb),
    currentSandboxCount: optionalNumber(value.current_sandbox_count),
    status: optionalString(value.status),
  };
}

function rulesToWire(rules: Record<string, NetworkRule[]>): WireObject {
  return Object.fromEntries(
    Object.entries(rules).map(([path, values]) => [
      path,
      values.map((rule) => ({ transform: { headers: { ...rule.headers } } })),
    ]),
  );
}

function rulesFromWire(value: unknown): Record<string, NetworkRule[]> {
  if (!value || typeof value !== "object" || Array.isArray(value)) return {};
  return Object.fromEntries(
    Object.entries(value).map(([path, items]) => [
      path,
      Array.isArray(items)
        ? items
            .map((item) => objectValue(item))
            .map((item) => {
              const transform =
                item.transform && typeof item.transform === "object"
                  ? objectValue(item.transform)
                  : {};
              return { headers: stringRecord(transform.headers) };
            })
        : [],
    ]),
  );
}

export function parseObjectItems(value: unknown): WireObject[] {
  return objectItems(value);
}
