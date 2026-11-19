import { CommandHandle, type Commands } from "./commands.js";
import type { Transport } from "./internal/transport.js";
import type { PtySize } from "./models.js";

type TransportProvider = () => Promise<Transport>;

/** Options for {@link Pty.start}. */
export interface PtyStartOptions {
  size?: PtySize;
  envs?: Record<string, string>;
  cwd?: string;
  user?: string;
}

/**
 * Interactive terminal sessions in the sandbox. Input goes through
 * {@link CommandHandle.sendStdin}, output arrives via the handle's event
 * stream. Accessed via {@link Sandbox.pty}.
 */
export class Pty {
  readonly #commands: Commands;
  readonly #transport: TransportProvider;

  constructor(commands: Commands, transport: TransportProvider) {
    this.#commands = commands;
    this.#transport = transport;
  }

  /**
   * Starts an interactive PTY session (a login shell by default) and returns
   * its handle. The first event carries the process PID.
   */
  async start(command = "/bin/bash", options: PtyStartOptions = {}): Promise<CommandHandle> {
    const size = options.size ?? { rows: 24, cols: 80 };
    const process: Record<string, unknown> = {
      cmd: command,
      args: command === "/bin/bash" ? ["-i", "-l"] : [],
      envs: { TERM: "xterm-256color", ...options.envs },
    };
    if (options.cwd) process.cwd = options.cwd;
    return this.#commands.start(
      { process },
      { timeoutMs: null, user: options.user, pty: size, inputStream: "pty" },
    );
  }

  /** Re-attaches to the PTY of an existing process by PID. */
  async connect(pid: number, options: { timeoutMs?: number | null } = {}): Promise<CommandHandle> {
    const timeoutMs = options.timeoutMs ?? null;
    return CommandHandle.fromProcess(
      pid,
      this.#commands,
      await this.#commands.connectEvents(pid, timeoutMs),
      "pty",
      timeoutMs,
    );
  }

  /** Resizes the PTY of a running process. */
  async resize(pid: number, size: PtySize): Promise<void> {
    if (size.rows < 1 || size.cols < 1) throw new RangeError("PTY rows and cols must be positive");
    await (await this.#transport()).connectUnary("/process.Process/Update", {
      process: { pid },
      pty: { size: { rows: size.rows, cols: size.cols } },
    });
  }
}
