import { readFile, writeFile } from "node:fs/promises";
import { NotFoundError, ProtocolError } from "./errors.js";
import type { Transport } from "./internal/transport.js";
import { objectValue, type WireObject } from "./internal/wire.js";
import { type FileInfo, type FileWatchEvent, parseFileInfo } from "./models.js";

const FILESYSTEM = "/filesystem.Filesystem";
type TransportProvider = () => Promise<Transport>;

/** Options for {@link Filesystem.write}. */
export interface WriteFileOptions {
  encoding?: BufferEncoding;
}

/** Options for {@link Filesystem.list}. */
export interface ListFilesOptions {
  depth?: number;
}

/**
 * File operations inside the sandbox: read/write files, list directories,
 * watch changes and upload/download. Paths must be absolute. Accessed via
 * {@link Sandbox.files}.
 */
export class Filesystem {
  readonly #transport: TransportProvider;

  constructor(transport: TransportProvider) {
    this.#transport = transport;
  }

  /** Reads a file as raw bytes. */
  async readBytes(path: string): Promise<Uint8Array> {
    return (await this.#transport()).requestBytes("GET", "/files", {
      params: { path: sandboxPath(path) },
    });
  }

  /** Reads a file and decodes it with the given encoding (default utf8). */
  async read(path: string, encoding: BufferEncoding = "utf8"): Promise<string> {
    return Buffer.from(await this.readBytes(path)).toString(encoding);
  }

  /** Writes a file; string data is encoded with `options.encoding` (default utf8). */
  async write(
    path: string,
    data: string | Uint8Array,
    options: WriteFileOptions = {},
  ): Promise<FileInfo> {
    const body = typeof data === "string" ? Buffer.from(data, options.encoding ?? "utf8") : data;
    const payload = await (await this.#transport()).request("POST", "/files", {
      body,
      params: { path: sandboxPath(path) },
      headers: { "Content-Type": "application/octet-stream" },
    });
    return parseFileInfo(firstItem(payload));
  }

  /** Writes multiple files in one call; keys are sandbox paths, values their content. */
  async writeBatch(
    files: Readonly<Record<string, string | Uint8Array>>,
    options: WriteFileOptions = {},
  ): Promise<FileInfo[]> {
    const result: FileInfo[] = [];
    for (const [path, data] of Object.entries(files))
      result.push(await this.write(path, data, options));
    return result;
  }

  /** Lists a directory; `options.depth` controls recursion (default 1). */
  async list(path: string, options: ListFilesOptions = {}): Promise<FileInfo[]> {
    const depth = options.depth ?? 1;
    if (!Number.isInteger(depth) || depth < 1)
      throw new RangeError("depth must be a positive integer");
    const payload = await (await this.#transport()).connectUnary(`${FILESYSTEM}/ListDir`, {
      path: sandboxPath(path),
      depth,
    });
    return items(payload, "entries").map(parseFileInfo);
  }

  /** Returns file/directory metadata for a single path. */
  async stat(path: string): Promise<FileInfo> {
    const payload = objectValue(
      await (await this.#transport()).connectUnary(`${FILESYSTEM}/Stat`, {
        path: sandboxPath(path),
      }),
    );
    if (!payload.entry || typeof payload.entry !== "object")
      throw new ProtocolError("filesystem response does not contain an entry");
    return parseFileInfo(objectValue(payload.entry));
  }

  /** Reports whether a path exists (false instead of throwing on 404). */
  async exists(path: string): Promise<boolean> {
    try {
      await this.stat(path);
      return true;
    } catch (error) {
      if (error instanceof NotFoundError) return false;
      throw error;
    }
  }

  /** Creates a directory (and parents) at the given path. */
  async makeDir(path: string): Promise<void> {
    await (await this.#transport()).connectUnary(`${FILESYSTEM}/MakeDir`, {
      path: sandboxPath(path),
    });
  }

  /** Moves (renames) a file or directory. */
  async move(source: string, destination: string): Promise<void> {
    await (await this.#transport()).connectUnary(`${FILESYSTEM}/Move`, {
      source: sandboxPath(source),
      destination: sandboxPath(destination),
    });
  }

  /** Removes a file or directory. */
  async remove(path: string): Promise<void> {
    await (await this.#transport()).connectUnary(`${FILESYSTEM}/Remove`, {
      path: sandboxPath(path),
    });
  }

  /** Uploads a local file into the sandbox. */
  async upload(localPath: string, remotePath: string): Promise<FileInfo> {
    return this.write(remotePath, await readFile(localPath));
  }

  /** Downloads a sandbox file to a local path; returns the local path. */
  async download(remotePath: string, localPath: string): Promise<string> {
    await writeFile(localPath, await this.readBytes(remotePath));
    return localPath;
  }

  /**
   * Watches a directory and yields change events. `options.recursive`
   * includes subdirectories; `options.includeEntry` toggles the resolved
   * `entry` metadata on each event. The stream ends when the generator is
   * closed (return/break).
   */
  async *watch(
    path: string,
    options: { recursive?: boolean; includeEntry?: boolean } = {},
  ): AsyncGenerator<FileWatchEvent> {
    const stream = (await this.#transport()).connectStream(`${FILESYSTEM}/WatchDir`, {
      path: sandboxPath(path),
      recursive: options.recursive ?? false,
      includeEntry: options.includeEntry ?? true,
    });
    try {
      for await (const response of stream) {
        const event = filesystemEvent(response);
        if (event) yield event;
      }
    } finally {
      stream.close();
    }
  }
}

function sandboxPath(value: string): string {
  if (!value?.startsWith("/")) throw new TypeError("sandbox paths must be absolute");
  return value;
}

function items(value: unknown, key: string): WireObject[] {
  const source = Array.isArray(value)
    ? value
    : value && typeof value === "object"
      ? objectValue(value)[key]
      : [];
  if (!Array.isArray(source)) throw new ProtocolError("filesystem response is invalid");
  return source.map((item) => objectValue(item));
}

function firstItem(value: unknown): WireObject {
  const values = items(value, "files");
  if (!values[0]) throw new ProtocolError("file upload response is empty");
  return values[0];
}

function filesystemEvent(response: WireObject): FileWatchEvent | undefined {
  let value = response.filesystem;
  if (!value && response.event && typeof response.event === "object")
    value = objectValue(response.event).filesystem;
  if (!value || typeof value !== "object") return undefined;
  const event = objectValue(value);
  const entry =
    event.entry && typeof event.entry === "object"
      ? parseFileInfo(objectValue(event.entry))
      : undefined;
  return {
    name: String(event.name ?? ""),
    type: String(event.type ?? event.operation ?? ""),
    path: event.path === undefined ? undefined : String(event.path),
    entry,
  };
}
