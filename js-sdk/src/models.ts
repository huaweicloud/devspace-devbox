/**
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2027. All rights reserved.
 */

import { ProtocolError } from './errors.js';
import {
  dateValue,
  numberValue,
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
} from './internal/wire.js';

/** Lifecycle states of a sandbox as reported by the Manager API. */
export const SandboxState = {
  Running: 'running',
  Paused: 'paused',
  Stopped: 'stopped',
} as const;
/** One of the {@link SandboxState} values. */
export type SandboxState = (typeof SandboxState)[keyof typeof SandboxState];

/** Entry types returned by the sandbox filesystem. */
export const FileType = { File: 'file', Directory: 'directory', Symlink: 'symlink' } as const;
/** One of the {@link FileType} values. */
export type FileType = (typeof FileType)[keyof typeof FileType];

/** Deadline behavior requested at sandbox creation. */
export interface SandboxLifecycle {
  onTimeout?: 'kill' | 'pause';
  autoResume?: boolean;
}

/** Deadline behavior as reported by the Manager API (all fields resolved). */
export interface SandboxInfoLifecycle {
  onTimeout: 'kill' | 'pause';
  autoResume: boolean;
}

/** A sandbox template: an image with its default resource footprint. */
export interface TemplateInfo {
  templateId: string;
  name: string;
  aliases: string[];
  buildId: string;
  cpuCount: number;
  memoryMb: number;
  diskSizeMb: number;
  names?: string[];
}

/** Sandbox details as reported by the Manager API. */
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

/** Connection credentials for the sandbox-local envd service. */
export interface SandboxConnection {
  sandboxId: string;
  domain: string;
  tunnelId: string;
  connectToken: string;
  tokenExpiration?: number;
  envdAccessToken?: string;
}

/** A process running (or previously run) in the sandbox. */
export interface ProcessInfo {
  pid: number;
  command: string;
  running: boolean;
  startedAt?: Date;
} /** One decoded chunk of command output. */
export interface OutputChunk {
  stream: 'stdout' | 'stderr';
  data: string;
  timestamp?: Date;
}

/** Final result of a completed command. */
export interface CommandResult {
  exitCode: number;
  stdout: string;
  stderr: string;
  pid?: number;
}

/** Terminal dimensions for PTY sessions. */
export interface PtySize {
  rows: number;
  cols: number;
}

/** Health info of the sandbox-local envd service. */
export interface EnvdHealth {
  envdVersion: string;
}

/** One guest-kernel OOM kill event. */
export interface OomKill {
  seq: number;
  process: string;
}

/** CPU, memory and disk usage of the sandbox, as reported by envd. */
export interface EnvdMetrics {
  timestamp: Date;
  cpuCount: number;
  cpuPossible?: number;
  cpuTarget?: number;
  cpuTargetAttempts?: number;
  cpuWritePendingMs?: number;
  cpuUsedPct: number;
  memTotal: number;
  memUsed: number;
  memCache: number;
  diskTotal: number;
  diskUsed: number;
  oomKills?: OomKill[];
}

/** Metadata of a file or directory in the sandbox. */
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

/** One change event from {@link Filesystem.watch}. */
export interface FileWatchEvent {
  name: string;
  type: string;
  path?: string;
  entry?: FileInfo;
}

/** A paginated list result with the server-provided continuation token. */
export interface Page<T> {
  items: T[];
  nextToken?: string;
  total?: number;
}

/** One historical metrics sample of a sandbox, as reported by the Manager API. */
export interface SandboxMetrics {
  timestampUnix: number;
  cpuCount: number;
  cpuUsedPct: number;
  memTotal: number;
  memUsed: number;
  memCache: number;
  diskTotal: number;
  diskUsed: number;
  timestamp?: Date;
  cpuPossible?: number;
  cpuTarget?: number;
  cpuTargetAttempts?: number;
  cpuWritePendingMs?: number;
  envdVersion?: string;
}

/**
 * Parses a Manager API sandbox payload into {@link SandboxInfo}; accepts the
 * documented field names and common snake_case variants. Throws
 * {@link ProtocolError} on an unknown state or lifecycle value.
 */
export function parseSandboxInfo(value: WireObject): SandboxInfo {
  const state = stringValue(pickOr(value, 'running', 'state', 'status'));
  if (!Object.values(SandboxState).includes(state as SandboxState)) {
    throw new ProtocolError('DevBox returned an invalid SandboxState');
  }
  const lifecycle =
    value.lifecycle && typeof value.lifecycle === 'object'
      ? objectValue(value.lifecycle)
      : undefined;
  const onTimeout = lifecycle?.onTimeout ?? 'kill';
  if (onTimeout !== 'kill' && onTimeout !== 'pause') {
    throw new ProtocolError('DevBox returned an invalid sandbox lifecycle');
  }
  return {
    sandboxId: stringValue(pick(value, 'sandboxId', 'sandboxID', 'sandbox_id', 'id')),
    templateId: stringValue(pickOr(value, 'default', 'templateId', 'templateID', 'template_id')),
    state: state as SandboxState,
    startedAt: optionalDate(
      pickOr(value, undefined, 'createdAt', 'created_at', 'startedAt', 'started_at'),
    ),
    endAt: optionalDate(pickOr(value, undefined, 'expiresAt', 'expires_at', 'endAt', 'end_at')),
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

/** Parses a Manager API template payload into {@link TemplateInfo}. */
export function parseTemplateInfo(value: WireObject): TemplateInfo {
  return {
    templateId: stringValue(pick(value, 'templateId', 'templateID', 'template_id')),
    name: stringValue(value.name ?? stringArray(value.names)[0] ?? value.templateID ?? ''),
    ...(value.names ? { names: stringArray(value.names) } : {}),
    aliases: stringArray(value.aliases),
    buildId: stringValue(pick(value, 'buildId', 'buildID', 'build_id')),
    cpuCount: numberValue(pick(value, 'cpuCount')),
    memoryMb: numberValue(pick(value, 'memoryMB', 'memoryMb', 'memory_mb')),
    diskSizeMb: numberValue(pick(value, 'diskSizeMB', 'diskSizeMb', 'disk_size_mb')),
  };
}

/**
 * Parses a connect/create/resume payload into {@link SandboxConnection};
 * normalizes the envd endpoint to a complete HTTPS URL and fills the
 * connect token from `connectToken` or `envdAccessToken`.
 */
export function parseConnection(value: WireObject, sandboxId: string): SandboxConnection {
  let domain = stringValue(value.envdUrl ?? value.domain ?? '');
  const isRelativeDomain = !!domain && !domain.includes('://');
  const needsEnvdUrl = !value.envdUrl && value.envdAccessToken && !value.connectToken;
  if (needsEnvdUrl && isRelativeDomain) {
    domain = `https://49983-${sandboxId}.${domain}`;
  }
  if (domain && !domain.startsWith('http://') && !domain.startsWith('https://')) {
    domain = `https://${domain}`;
  }
  return {
    sandboxId,
    domain,
    tunnelId: stringValue(value.tunnelId ?? ''),
    connectToken: stringValue(value.connectToken ?? value.envdAccessToken ?? ''),
    ...(value.envdAccessToken ? { envdAccessToken: stringValue(value.envdAccessToken) } : {}),
    tokenExpiration: optionalNumber(value.tokenExpiration),
  };
}

/** Parses a process list entry into {@link ProcessInfo}. */
export function parseProcessInfo(value: WireObject): ProcessInfo {
  const process =
    value.config && typeof value.config === 'object' ? objectValue(value.config) : value;
  const command = [String(pickOr(process, '', 'command', 'cmd')), ...stringArray(process.args)]
    .join(' ')
    .trim();
  return {
    pid: numberValue(pick(value, 'pid')),
    command,
    running: Boolean(pickOr(value, true, 'running')),
    startedAt: optionalDate(pickOr(value, undefined, 'startedAt', 'started_at')),
  };
}

/** Parses a filesystem entry into {@link FileInfo}; throws on unknown file types. */
export function parseFileInfo(value: WireObject): FileInfo {
  const rawType = stringValue(pickOr(value, 'file', 'type'));
  const type =
    (
      {
        FILE_TYPE_FILE: 'file',
        FILE_TYPE_DIRECTORY: 'directory',
        FILE_TYPE_SYMLINK: 'symlink',
      } as Record<string, FileType>
    )[rawType] ?? rawType;
  if (!Object.values(FileType).includes(type as FileType)) {
    throw new ProtocolError('DevBox returned an invalid FileType');
  }
  return {
    name: stringValue(pick(value, 'name')),
    path: stringValue(pick(value, 'path')),
    type: type as FileType,
    size: numberValue(pickOr(value, 0, 'size')),
    mode: optionalNumber(value.mode),
    permissions: optionalString(value.permissions),
    owner: optionalString(value.owner),
    group: optionalString(value.group),
    modifiedAt: optionalDate(pickOr(value, undefined, 'modifiedTime', 'modifiedAt', 'modified_at')),
    symlinkTarget: optionalString(pickOr(value, undefined, 'symlinkTarget', 'symlink_target')),
  };
}

/** Parses an envd `/metrics` payload into {@link EnvdMetrics}. */
export function parseEnvdMetrics(value: WireObject): EnvdMetrics {
  const oomKills = value.oom_kills;
  return {
    timestamp: dateValue(pick(value, 'ts')),
    cpuCount: numberValue(pick(value, 'cpu_count')),
    cpuPossible: optionalNumber(value.cpu_possible),
    cpuTarget: optionalNumber(value.cpu_target),
    cpuTargetAttempts: optionalNumber(value.cpu_target_attempts),
    cpuWritePendingMs: optionalNumber(value.cpu_write_pending_ms),
    cpuUsedPct: numberValue(pick(value, 'cpu_used_pct')),
    memTotal: numberValue(pick(value, 'mem_total')),
    memUsed: numberValue(pick(value, 'mem_used')),
    memCache: numberValue(pick(value, 'mem_cache')),
    diskTotal: numberValue(pick(value, 'disk_total')),
    diskUsed: numberValue(pick(value, 'disk_used')),
    ...(Array.isArray(oomKills) ? { oomKills: oomKills.map((entry) => parseOomKill(entry)) } : {}),
  };
}

/** Parses an envd OOM kill entry into {@link OomKill}. */
export function parseOomKill(value: unknown): OomKill {
  const entry = objectValue(value, 'EnvD returned an invalid oom kill entry');
  return { seq: numberValue(pick(entry, 'seq')), process: stringValue(pick(entry, 'process')) };
}

/**
 * Parses a Manager API `/sandboxes/{id}/metrics` payload into
 * {@link SandboxMetrics}.
 */
export function parseMetrics(value: WireObject): SandboxMetrics {
  return {
    timestampUnix: numberValue(pick(value, "timestampUnix")),
    cpuCount: numberValue(pick(value, "cpuCount")),
    cpuUsedPct: numberValue(pick(value, "cpuUsedPct")),
    memTotal: numberValue(pick(value, "memTotal")),
    memUsed: numberValue(pick(value, "memUsed")),
    memCache: numberValue(pick(value, "memCache")),
    diskTotal: numberValue(pick(value, "diskTotal")),
    diskUsed: numberValue(pick(value, "diskUsed")),
    timestamp: optionalDate(value.timestamp),
    cpuPossible: optionalNumber(value.cpuPossible),
    cpuTarget: optionalNumber(value.cpuTarget),
    cpuTargetAttempts: optionalNumber(value.cpuTargetAttempts),
    cpuWritePendingMs: optionalNumber(value.cpuWritePendingMs),
    envdVersion: optionalString(value.envdVersion),
  };
}
