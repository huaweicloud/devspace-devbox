import type { MockAgent } from "undici";
import { afterEach, describe, expect, it } from "vitest";
import { Filesystem } from "../src/filesystem.js";
import { Transport } from "../src/internal/transport.js";
import { mockAgent } from "./helpers.js";

describe("Filesystem", () => {
  let agent: MockAgent | undefined;
  afterEach(async () => agent?.close());

  it("reads and writes files", async () => {
    agent = mockAgent();
    const pool = agent.get("https://runtime.example.test");
    pool.intercept({ path: "/files?path=%2Ftmp%2Fhello.txt", method: "GET" }).reply(200, "hello");
    pool.intercept({ path: "/files?path=%2Ftmp%2Fhello.txt", method: "POST" }).reply(({ body }) => {
      expect(Buffer.from(body as Uint8Array).toString()).toBe("updated");
      return {
        statusCode: 200,
        data: [{ name: "hello.txt", path: "/tmp/hello.txt", type: "file", size: 7 }],
      };
    });
    const files = new Filesystem(
      async () => new Transport("https://runtime.example.test", { dispatcher: agent }),
    );
    await expect(files.read("/tmp/hello.txt")).resolves.toBe("hello");
    await expect(files.write("/tmp/hello.txt", "updated")).resolves.toMatchObject({
      path: "/tmp/hello.txt",
      size: 7,
    });
  });

  it("requires absolute sandbox paths", async () => {
    agent = mockAgent();
    const files = new Filesystem(
      async () => new Transport("https://runtime.example.test", { dispatcher: agent }),
    );
    await expect(files.read("relative.txt")).rejects.toThrow("sandbox paths must be absolute");
  });
});
