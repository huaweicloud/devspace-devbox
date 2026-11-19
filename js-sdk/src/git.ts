import type { Commands } from "./commands.js";
import type { CommandResult } from "./models.js";

/** Credentials passed to git via a sandbox-side credential helper. */
export interface GitCredentials {
  username: string;
  password: string;
}

/** Options for network-bound git operations. */
export interface GitNetworkOptions {
  credentials?: GitCredentials;
  timeoutMs?: number | null;
}

/**
 * Git operations inside the sandbox, executed through
 * {@link Commands.run} against the sandbox's git binary. Accessed via
 * {@link Sandbox.git}.
 */
export class Git {
  readonly #commands: Commands;

  constructor(commands: Commands) {
    this.#commands = commands;
  }

  /** Clones a repository into `path`; defaults to a 300s timeout. */
  clone(
    url: string,
    path: string,
    options: GitNetworkOptions & { branch?: string; depth?: number } = {},
  ): Promise<CommandResult> {
    const command = ["git", "clone"];
    if (options.branch) command.push("--branch", options.branch);
    if (options.depth !== undefined) {
      if (!Number.isInteger(options.depth) || options.depth < 1)
        throw new RangeError("git clone depth must be positive");
      command.push("--depth", String(options.depth));
    }
    command.push(url, path);
    return this.#run(command, {
      ...options,
      timeoutMs: options.timeoutMs === undefined ? 300_000 : options.timeoutMs,
    });
  }

  /** Prints the working tree status (`git status --short --branch`). */
  status(repository: string): Promise<CommandResult> {
    return this.#run(inRepository(repository, "status", "--short", "--branch"));
  }

  /** Checks out a reference; `options.create` creates a new branch (`-b`). */
  checkout(
    repository: string,
    reference: string,
    options: { create?: boolean } = {},
  ): Promise<CommandResult> {
    return this.#run(
      inRepository(repository, "checkout", ...(options.create ? ["-b"] : []), reference),
    );
  }

  /** Stages paths for commit (defaults to `.`). */
  add(repository: string, paths: string | readonly string[] = "."): Promise<CommandResult> {
    const values = typeof paths === "string" ? [paths] : paths;
    if (values.length === 0) throw new TypeError("at least one git path is required");
    return this.#run(inRepository(repository, "add", "--", ...values));
  }

  /** Creates a commit with the given message. */
  commit(repository: string, message: string): Promise<CommandResult> {
    if (!message.trim()) throw new TypeError("commit message must not be blank");
    return this.#run(inRepository(repository, "commit", "-m", message));
  }

  /** Pulls from the remote (default `origin`); defaults to a 300s timeout. */
  pull(
    repository: string,
    options: GitNetworkOptions & { remote?: string; branch?: string } = {},
  ): Promise<CommandResult> {
    const command = inRepository(repository, "pull", options.remote ?? "origin");
    if (options.branch) command.push(options.branch);
    return this.#run(command, {
      ...options,
      timeoutMs: options.timeoutMs === undefined ? 300_000 : options.timeoutMs,
    });
  }

  /** Pushes to the remote (default `origin`); defaults to a 300s timeout. */
  push(
    repository: string,
    options: GitNetworkOptions & { remote?: string; branch?: string } = {},
  ): Promise<CommandResult> {
    const command = inRepository(repository, "push", options.remote ?? "origin");
    if (options.branch) command.push(options.branch);
    return this.#run(command, {
      ...options,
      timeoutMs: options.timeoutMs === undefined ? 300_000 : options.timeoutMs,
    });
  }

  /** Sets a repository-local git config value (`git config key value`). */
  setConfig(repository: string, key: string, value: string): Promise<CommandResult> {
    return this.#run(inRepository(repository, "config", key, value));
  }

  #run(command: string[], options: GitNetworkOptions = {}): Promise<CommandResult> {
    const credentials = options.credentials;
    if (credentials && (!credentials.username || !credentials.password)) {
      throw new TypeError("git username and password must not be blank");
    }
    return this.#commands.run(shellCommand(command, Boolean(credentials)), {
      envs: credentials
        ? {
            DEVBOX_GIT_USERNAME: credentials.username,
            DEVBOX_GIT_PASSWORD: credentials.password,
            GIT_TERMINAL_PROMPT: "0",
          }
        : undefined,
      timeoutMs: options.timeoutMs === undefined ? 60_000 : options.timeoutMs,
    });
  }
}

function shellCommand(command: string[], credentials: boolean): string {
  const arguments_ = [...command];
  if (credentials) {
    arguments_.splice(
      1,
      0,
      "-c",
      'credential.helper=!f() { printf \'%s\\n\' "username=$DEVBOX_GIT_USERNAME" "password=$DEVBOX_GIT_PASSWORD"; }; f',
      "-c",
      "credential.useHttpPath=true",
    );
  }
  return arguments_.map(shellQuote).join(" ");
}

function shellQuote(value: string): string {
  if (/^[A-Za-z0-9_@%+=:,./-]+$/.test(value)) return value;
  return `'${value.replaceAll("'", `'"'"'`)}'`;
}

function inRepository(repository: string, ...arguments_: string[]): string[] {
  if (!repository) throw new TypeError("repository path must not be blank");
  return ["git", "-C", repository, ...arguments_];
}
