import { readFile, writeFile } from "node:fs/promises";
import { NotFoundError, ProtocolError } from "./errors.js";
import type { Transport } from "./internal/transport.js";
import { objectValue, type WireObject } from "./internal/wire.js";
import { type FileInfo, type FileWatchEvent, parseFileInfo } from "./models.js";

const FILESYSTEM = "/filesystem.Filesystem";
type TransportProvider = () => Promise<Transport>;

export class Filesystem {
  readonly #transport: TransportProvider;

  constructor(transport: TransportProvider) {
    this.#transport = transport;
  }

  async readBytes(path: string): Promise<Uint8Array> {
    return (await this.#transport()).requestBytes("GET", "/files", {
      params: { path: sandboxPath(path) },
    });
  }

  async read(path: string, encoding: BufferEncoding = "utf8"): Promise<string> {
    return Buffer.from(await this.readBytes(path)).toString(encoding);
  }

  async write(
    path: string,
    data: string | Uint8Array,
    encoding: BufferEncoding = "utf8",
  ): Promise<FileInfo> {
    const body = typeof data === "string" ? Buffer.from(data, encoding) : data;
    const payload = await (await this.#transport()).request("POST", "/files", {
      body,
      params: { path: sandboxPath(path) },
      headers: { "Content-Type": "application/octet-stream" },
    });
    return parseFileInfo(firstItem(payload));
  }

  async writeBatch(
    files: Record<string, string | Uint8Array>,
    encoding: BufferEncoding = "utf8",
  ): Promise<FileInfo[]> {
    const result: FileInfo[] = [];
    for (const [path, data] of Object.entries(files))
      result.push(await this.write(path, data, encoding));
    return result;
  }

  async list(path: string, depth = 1): Promise<FileInfo[]> {
    if (!Number.isInteger(depth) || depth < 1)
      throw new RangeError("depth must be a positive integer");
    const payload = await (await this.#transport()).connectUnary(`${FILESYSTEM}/ListDir`, {
      path: sandboxPath(path),
      depth,
    });
    return items(payload, "entries").map(parseFileInfo);
  }

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

  async exists(path: string): Promise<boolean> {
    try {
      await this.stat(path);
      return true;
    } catch (error) {
      if (error instanceof NotFoundError) return false;
      throw error;
    }
  }

  async makeDir(path: string): Promise<void> {
    await (await this.#transport()).connectUnary(`${FILESYSTEM}/MakeDir`, {
      path: sandboxPath(path),
    });
  }

  async move(source: string, destination: string): Promise<void> {
    await (await this.#transport()).connectUnary(`${FILESYSTEM}/Move`, {
      source: sandboxPath(source),
      destination: sandboxPath(destination),
    });
  }

  async remove(path: string): Promise<void> {
    await (await this.#transport()).connectUnary(`${FILESYSTEM}/Remove`, {
      path: sandboxPath(path),
    });
  }

  async upload(localPath: string, remotePath: string): Promise<FileInfo> {
    return this.write(remotePath, await readFile(localPath));
  }

  async download(remotePath: string, localPath: string): Promise<string> {
    await writeFile(localPath, await this.readBytes(remotePath));
    return localPath;
  }

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
