import { Buffer } from "node:buffer";
import { CommandExitError, ProtocolError } from "./errors.js";
import type { ConnectStream, Transport } from "./internal/transport.js";
import { numberValue, objectValue, pick, pickOr, type WireObject } from "./internal/wire.js";
import {
  type CommandResult,
  type OutputChunk,
  type ProcessInfo,
  type PtySize,
  parseProcessInfo,
} from "./models.js";

const PROCESS = "/process.Process";
type TransportProvider = () => Promise<Transport>;
export type OutputHandler = (value: string) => void | Promise<void>;

export interface RunOptions {
  background?: boolean;
  envs?: Record<string, string>;
  cwd?: string;
  user?: string;
  stdin?: boolean;
  timeoutMs?: number | null;
  onStdout?: OutputHandler;
  onStderr?: OutputHandler;
  check?: boolean;
}

export interface WaitOptions {
  onStdout?: OutputHandler;
  onStderr?: OutputHandler;
  check?: boolean;
}

interface EventSource {
  iterator: AsyncIterator<WireObject>;
  close(): void;
}

export class CommandHandle {
  readonly pid: number;
  readonly #commands: Commands;
  readonly #inputStream: "stdin" | "pty";
  readonly #reconnectTimeoutMs: number | null;
  #events?: EventSource;
  #result?: CommandResult;

  constructor(
    pid: number,
    commands: Commands,
    events?: EventSource,
    inputStream: "stdin" | "pty" = "stdin",
    reconnectTimeoutMs: number | null = 60_000,
  ) {
    this.pid = pid;
    this.#commands = commands;
    this.#events = events;
    this.#inputStream = inputStream;
    this.#reconnectTimeoutMs = reconnectTimeoutMs;
  }

  async wait(options: WaitOptions = {}): Promise<CommandResult> {
    if (!this.#result) {
      const events =
        this.#events ?? (await this.#commands.connectEvents(this.pid, this.#reconnectTimeoutMs));
      this.#events = undefined;
      try {
        this.#result = await collectEvents(
          events.iterator,
          this.pid,
          options.onStdout,
          options.onStderr,
        );
      } finally {
        events.close();
      }
    }
    if ((options.check ?? true) && this.#result.exitCode !== 0)
      throw new CommandExitError(this.#result);
    return this.#result;
  }

  async sendStdin(data: string | Uint8Array): Promise<void> {
    this.#ensureActive();
    await this.#commands.sendInput(this.pid, data, this.#inputStream);
  }

  async closeStdin(): Promise<void> {
    this.#ensureActive();
    await this.#commands.closeStdin(this.pid);
  }

  async sendSignal(signal: "SIGTERM" | "SIGKILL"): Promise<void> {
    this.#ensureActive();
    await this.#commands.sendSignal(this.pid, signal);
  }

  kill(): Promise<void> {
    return this.sendSignal("SIGKILL");
  }

  disconnect(): void {
    this.#events?.close();
    this.#events = undefined;
  }

  #ensureActive(): void {
    if (this.#result) throw new Error("process has already completed");
  }
}

export class Commands {
  readonly #transport: TransportProvider;

  constructor(transport: TransportProvider) {
    this.#transport = transport;
  }

  async run(command: string, options: RunOptions & { background: true }): Promise<CommandHandle>;
  async run(command: string, options?: RunOptions & { background?: false }): Promise<CommandResult>;
  async run(command: string, options: RunOptions): Promise<CommandResult | CommandHandle>;
  async run(command: string, options: RunOptions = {}): Promise<CommandResult | CommandHandle> {
    if (options.background && (options.onStdout || options.onStderr)) {
      throw new TypeError("background output callbacks belong on handle.wait()");
    }
    const handle = await this.start(commandBody(command, options), {
      timeoutMs: options.timeoutMs === undefined ? 60_000 : options.timeoutMs,
      user: options.user,
    });
    if (options.background) return handle;
    return handle.wait({
      onStdout: options.onStdout,
      onStderr: options.onStderr,
      check: options.check,
    });
  }

  async connect(pid: number, options: { timeoutMs?: number | null } = {}): Promise<CommandHandle> {
    validatePid(pid);
    const timeoutMs = options.timeoutMs === undefined ? 60_000 : options.timeoutMs;
    return new CommandHandle(
      pid,
      this,
      await this.connectEvents(pid, timeoutMs),
      "stdin",
      timeoutMs,
    );
  }

  async list(): Promise<ProcessInfo[]> {
    const payload = objectValue(
      await (await this.#transport()).connectUnary(`${PROCESS}/List`, {}),
    );
    const processes = payload.processes;
    if (!Array.isArray(processes)) throw new ProtocolError("process response is invalid");
    return processes.map((item) => objectValue(item)).map(parseProcessInfo);
  }

  sendStdin(pid: number, data: string | Uint8Array): Promise<void> {
    return this.sendInput(pid, data, "stdin");
  }

  async closeStdin(pid: number): Promise<void> {
    validatePid(pid);
    await (await this.#transport()).connectUnary(`${PROCESS}/CloseStdin`, { process: { pid } });
  }

  async sendSignal(pid: number, signal: "SIGTERM" | "SIGKILL"): Promise<void> {
    validatePid(pid);
    await (await this.#transport()).connectUnary(`${PROCESS}/SendSignal`, {
      process: { pid },
      signal: `SIGNAL_${signal}`,
    });
  }

  async start(
    body: WireObject,
    options: {
      timeoutMs: number | null;
      user?: string;
      pty?: PtySize;
      inputStream?: "stdin" | "pty";
    },
  ): Promise<CommandHandle> {
    const request = { ...body };
    if (options.pty) {
      if (options.pty.rows < 1 || options.pty.cols < 1)
        throw new RangeError("PTY rows and cols must be positive");
      request.pty = { size: { rows: options.pty.rows, cols: options.pty.cols } };
    }
    const transport = await this.#transport();
    const headers: Record<string, string> = { "Keepalive-Ping-Interval": "50" };
    if (options.user)
      headers.Authorization = `Basic ${Buffer.from(`${options.user}:`).toString("base64")}`;
    const stream = transport.connectStream(
      `${PROCESS}/Start`,
      request,
      options.timeoutMs ?? undefined,
      headers,
    );
    const events = eventSource(stream);
    try {
      const pid = await firstPid(events.iterator, "start process");
      return new CommandHandle(pid, this, events, options.inputStream, options.timeoutMs);
    } catch (error) {
      events.close();
      throw error;
    }
  }

  async connectEvents(pid: number, timeoutMs: number | null): Promise<EventSource> {
    const stream = (await this.#transport()).connectStream(
      `${PROCESS}/Connect`,
      { process: { pid } },
      timeoutMs ?? undefined,
      { "Keepalive-Ping-Interval": "50" },
    );
    const events = eventSource(stream);
    try {
      if ((await firstPid(events.iterator, "connect to process")) !== pid) {
        throw new ProtocolError("EnvD connected to an unexpected process");
      }
      return events;
    } catch (error) {
      events.close();
      throw error;
    }
  }

  async sendInput(pid: number, data: string | Uint8Array, stream: "stdin" | "pty"): Promise<void> {
    validatePid(pid);
    const bytes = typeof data === "string" ? Buffer.from(data) : Buffer.from(data);
    await (await this.#transport()).connectUnary(`${PROCESS}/SendInput`, {
      process: { pid },
      input: { [stream]: bytes.toString("base64") },
    });
  }
}

function commandBody(command: string, options: RunOptions): WireObject {
  if (!command.trim()) throw new TypeError("command must not be blank");
  const process: WireObject = {
    cmd: "/bin/bash",
    args: ["-l", "-c", command],
    envs: { ...options.envs },
  };
  if (options.cwd) process.cwd = options.cwd;
  return { process, stdin: options.stdin ?? false };
}

function eventSource(stream: ConnectStream): EventSource {
  return { iterator: stream[Symbol.asyncIterator](), close: () => stream.close() };
}

async function firstPid(events: AsyncIterator<WireObject>, action: string): Promise<number> {
  const response = await events.next();
  if (response.done) throw new ProtocolError(`EnvD did not ${action}`);
  const event = objectValue(pick(response.value, "event"));
  const start = objectValue(pick(event, "start"));
  const pid = numberValue(pick(start, "pid"));
  validatePid(pid);
  return pid;
}

async function collectEvents(
  events: AsyncIterator<WireObject>,
  pid: number,
  onStdout?: OutputHandler,
  onStderr?: OutputHandler,
): Promise<CommandResult> {
  const stdout: string[] = [];
  const stderr: string[] = [];
  const stdoutDecoder = new TextDecoder("utf-8");
  const stderrDecoder = new TextDecoder("utf-8");
  let exitCode: number | undefined;

  while (true) {
    const next = await events.next();
    if (next.done) break;
    const event =
      next.value.event && typeof next.value.event === "object"
        ? objectValue(next.value.event)
        : undefined;
    if (!event) continue;
    const data = event.data && typeof event.data === "object" ? objectValue(event.data) : undefined;
    if (data) {
      const chunks: OutputChunk[] = [];
      for (const source of ["stdout", "stderr", "pty"] as const) {
        if (!(source in data)) continue;
        const target = source === "pty" ? "stdout" : source;
        const decoder = target === "stdout" ? stdoutDecoder : stderrDecoder;
        const value = decoder.decode(decodeBase64(data[source]), { stream: true });
        if (value) chunks.push({ stream: target, data: value });
      }
      for (const chunk of chunks) {
        (chunk.stream === "stdout" ? stdout : stderr).push(chunk.data);
        await (chunk.stream === "stdout" ? onStdout?.(chunk.data) : onStderr?.(chunk.data));
      }
    }
    if (event.end && typeof event.end === "object") {
      const end = objectValue(event.end);
      exitCode = numberValue(pickOr(end, 0, "exitCode", "exit_code"));
    }
  }

  const stdoutTail = stdoutDecoder.decode();
  const stderrTail = stderrDecoder.decode();
  if (stdoutTail) {
    stdout.push(stdoutTail);
    await onStdout?.(stdoutTail);
  }
  if (stderrTail) {
    stderr.push(stderrTail);
    await onStderr?.(stderrTail);
  }
  if (exitCode === undefined)
    throw new ProtocolError("command stream ended without an exit status");
  return { exitCode, stdout: stdout.join(""), stderr: stderr.join(""), pid };
}

function decodeBase64(value: unknown): Uint8Array {
  const text = String(value);
  if (!/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(text)) {
    throw new ProtocolError("EnvD returned invalid process output");
  }
  return Buffer.from(text, "base64");
}

function validatePid(pid: number): void {
  if (!Number.isInteger(pid) || pid < 1) throw new RangeError("pid must be a positive integer");
}
