import { ProtocolError } from "./errors.js";
import {
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

export interface SandboxLifecycle {
  onTimeout?: "kill" | "pause";
  autoResume?: boolean;
}

export interface SandboxInfoLifecycle {
  onTimeout: "kill" | "pause";
  autoResume: boolean;
}

export interface SandboxInfo {
  sandboxId: string;
  templateId: string;
  state: SandboxState;
  startedAt?: Date;
  endAt?: Date;
  cpuCount?: number;
  memoryMb?: number;
  diskSizeMb?: number;
  metadata: Record<string, string>;
  lifecycle?: SandboxInfoLifecycle;
}

export interface SandboxConnection {
  sandboxId: string;
  domain: string;
  tunnelId: string;
  connectToken: string;
  tokenExpiration?: number;
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

export interface Page<T> {
  items: T[];
  nextToken?: string;
  total?: number;
}

export function parseSandboxInfo(value: WireObject): SandboxInfo {
  const state = stringValue(pickOr(value, "running", "state", "status"));
  if (!Object.values(SandboxState).includes(state as SandboxState)) {
    throw new ProtocolError("DevBox returned an invalid SandboxState");
  }
  const lifecycle =
    value.lifecycle && typeof value.lifecycle === "object"
      ? objectValue(value.lifecycle)
      : undefined;
  const onTimeout = lifecycle?.onTimeout ?? "kill";
  if (onTimeout !== "kill" && onTimeout !== "pause")
    throw new ProtocolError("DevBox returned an invalid sandbox lifecycle");
  return {
    sandboxId: stringValue(pick(value, "sandboxId", "sandboxID", "sandbox_id", "id")),
    templateId: stringValue(pickOr(value, "default", "templateId", "templateID", "template_id")),
    state: state as SandboxState,
    startedAt: optionalDate(
      pickOr(value, undefined, "createdAt", "created_at", "startedAt", "started_at"),
    ),
    endAt: optionalDate(pickOr(value, undefined, "expiresAt", "expires_at", "endAt", "end_at")),
    cpuCount: optionalNumber(value.cpuCount),
    memoryMb: optionalNumber(value.memoryMB),
    diskSizeMb: optionalNumber(value.diskSizeMB),
    metadata: stringRecord(value.metadata),
    lifecycle: lifecycle
      ? {
          autoResume: Boolean(lifecycle.autoResume ?? false),
          onTimeout,
        }
      : undefined,
  };
}

export function parseConnection(value: WireObject, sandboxId: string): SandboxConnection {
  let domain = stringValue(value.domain ?? "");
  if (domain && !domain.startsWith("http://") && !domain.startsWith("https://"))
    domain = `https://${domain}`;
  return {
    sandboxId,
    domain,
    tunnelId: stringValue(value.tunnelId ?? ""),
    connectToken: stringValue(value.connectToken ?? ""),
    tokenExpiration: optionalNumber(value.tokenExpiration),
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

export function parseObjectItems(value: unknown): WireObject[] {
  return objectItems(value);
}
