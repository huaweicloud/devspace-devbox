import type { Commands } from "./commands.js";
import type { CommandResult } from "./models.js";

export interface GitCredentials {
  username: string;
  password: string;
}

export interface GitNetworkOptions {
  credentials?: GitCredentials;
  timeoutMs?: number | null;
}

export class Git {
  readonly #commands: Commands;

  constructor(commands: Commands) {
    this.#commands = commands;
  }

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
    return this.#run(command, { ...options, timeoutMs: options.timeoutMs ?? 300_000 });
  }

  status(repository: string): Promise<CommandResult> {
    return this.#run(inRepository(repository, "status", "--short", "--branch"));
  }

  checkout(
    repository: string,
    reference: string,
    options: { create?: boolean } = {},
  ): Promise<CommandResult> {
    return this.#run(
      inRepository(repository, "checkout", ...(options.create ? ["-b"] : []), reference),
    );
  }

  add(repository: string, paths: string | string[] = "."): Promise<CommandResult> {
    const values = typeof paths === "string" ? [paths] : paths;
    if (values.length === 0) throw new TypeError("at least one git path is required");
    return this.#run(inRepository(repository, "add", "--", ...values));
  }

  commit(repository: string, message: string): Promise<CommandResult> {
    if (!message.trim()) throw new TypeError("commit message must not be blank");
    return this.#run(inRepository(repository, "commit", "-m", message));
  }

  pull(
    repository: string,
    options: GitNetworkOptions & { remote?: string; branch?: string } = {},
  ): Promise<CommandResult> {
    const command = inRepository(repository, "pull", options.remote ?? "origin");
    if (options.branch) command.push(options.branch);
    return this.#run(command, { ...options, timeoutMs: options.timeoutMs ?? 300_000 });
  }

  push(
    repository: string,
    options: GitNetworkOptions & { remote?: string; branch?: string } = {},
  ): Promise<CommandResult> {
    const command = inRepository(repository, "push", options.remote ?? "origin");
    if (options.branch) command.push(options.branch);
    return this.#run(command, { ...options, timeoutMs: options.timeoutMs ?? 300_000 });
  }

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
