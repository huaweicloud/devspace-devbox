import type { MockAgent } from "undici";
import { afterEach, describe, expect, it } from "vitest";
import { DevBox } from "../src/client.js";
import { ConflictError, ProtocolError } from "../src/errors.js";
import { parseConnection, SandboxState } from "../src/models.js";
import { connectBody, mockAgent, sandboxResponse } from "./helpers.js";

describe("sandboxes", () => {
  let agent: MockAgent | undefined;
  afterEach(async () => {
    await agent?.close();
    agent = undefined;
  });

  it.each(["", "bad; other=value", "bad\r\nX-Test: value", "bad\n"])(
    "rejects missing or unsafe connect tokens",
    async (connectToken) => {
      agent = mockAgent();
      agent
        .get("https://manager.example.test")
        .intercept({ path: "/sandboxes", method: "POST" })
        .reply(201, { ...sandboxResponse, connectToken, envdAccessToken: "obsolete-token" });
      const client = new DevBox({
        apiKey: "key",
        apiUrl: "https://manager.example.test",
        dispatcher: agent,
      });
      const sandbox = await client.sandboxes.create();
      try {
        await expect(sandbox.files.read("/tmp/proof")).rejects.toBeInstanceOf(ProtocolError);
      } finally {
        await sandbox.close();
        await client.close();
      }
    },
  );

  it("uses the relay cookie for files and streamed commands", async () => {
    agent = mockAgent();
    agent
      .get("https://manager.example.test")
      .intercept({ path: "/sandboxes", method: "POST" })
      .reply(201, sandboxResponse);
    const pool = agent.get("https://runtime.example.test");
    const headers = { cookie: "relay_token=connect-token", "e2b-sandbox-id": "sbx-1" };
    pool
      .intercept({ path: "/files?path=%2Ftmp%2Fproof", method: "GET", headers })
      .reply(200, "proof");
    pool
      .intercept({ path: "/process.Process/Start", method: "POST", headers })
      .reply(
        200,
        connectBody(
          { value: { event: { start: { pid: 7 } } } },
          { value: { event: { end: { exitCode: 0 } } } },
          { value: {}, trailer: true },
        ),
        { headers: { "Content-Type": "application/connect+json" } },
      );
    const client = new DevBox({
      apiKey: "key",
      apiUrl: "https://manager.example.test",
      dispatcher: agent,
    });
    const sandbox = await client.sandboxes.create();
    try {
      expect(await sandbox.files.read("/tmp/proof")).toBe("proof");
      expect((await sandbox.commands.run("true")).exitCode).toBe(0);
      agent.assertNoPendingInterceptors();
    } finally {
      await sandbox.close();
      await client.close();
    }
  });

  it("parses the manager tunnel connection contract", () => {
    expect(parseConnection(sandboxResponse, "sbx-1")).toEqual({
      sandboxId: "sbx-1",
      domain: "https://runtime.example.test",
      tunnelId: "aaaadysa",
      connectToken: "connect-token",
      tokenLifetime: 86_400,
      tokenExpiration: 1_789_029_315,
      tunnelLifetime: 86_400,
      tunnelExpiration: 1_788_946_515,
      protocolVersion: "1.0.0",
    });
  });

  it("does not infer token expiration from tunnel expiration", () => {
    const connection = parseConnection({ tunnelExpiration: 1_788_946_515 }, "sbx-1");
    expect(connection.connectToken).toBe("");
    expect(connection.tokenLifetime).toBeUndefined();
    expect(connection.tokenExpiration).toBeUndefined();
    expect(connection.tunnelExpiration).toBe(1_788_946_515);
  });

  it("creates a sandbox using the manager wire contract", async () => {
    agent = mockAgent();
    const pool = agent.get("https://manager.example.test");
    pool
      .intercept({
        path: "/sandboxes",
        method: "POST",
        headers: { "X-API-Key": "devbridge_test", "Idempotency-Key": "request-1" },
      })
      .reply(({ body }) => {
        const request = JSON.parse(String(body));
        expect(request).toMatchObject({
          templateID: "default",
          timeout: 600,
          secure: true,
          envVars: { MODE: "test" },
        });
        expect(request).not.toHaveProperty("network");
        expect(request).not.toHaveProperty("allow_internet_access");
        return { statusCode: 200, data: sandboxResponse };
      });

    const client = new DevBox({
      apiKey: "devbridge_test",
      apiUrl: "https://manager.example.test",
      dispatcher: agent,
    });
    const sandbox = await client.sandboxes.create("default", {
      timeout: 600,
      envs: { MODE: "test" },
      idempotencyKey: "request-1",
    });

    expect(sandbox.sandboxId).toBe("sbx-1");
    expect(sandbox.info.state).toBe(SandboxState.Running);
    await client.close();
  });

  it("connects to a sandbox with explicit options", async () => {
    agent = mockAgent();
    const pool = agent.get("https://manager.example.test");
    pool.intercept({ path: "/sandboxes/sbx-1/connect", method: "POST" }).reply(({ body }) => {
      expect(JSON.parse(String(body))).toEqual({ timeout: 600 });
      return { statusCode: 200, data: sandboxResponse };
    });

    const client = new DevBox({
      apiKey: "key",
      apiUrl: "https://manager.example.test",
      dispatcher: agent,
    });
    const sandbox = await client.sandboxes.connect("sbx-1", { timeout: 600 });

    expect(sandbox.sandboxId).toBe("sbx-1");
    await client.close();
  });

  it.each(["already_killed", "another_conflict"])("handles kill conflict %s", async (code) => {
    agent = mockAgent();
    const pool = agent.get("https://manager.example.test");
    pool.intercept({ path: "/sandboxes", method: "POST" }).reply(201, sandboxResponse);
    pool
      .intercept({ path: "/sandboxes/sbx-1", method: "DELETE" })
      .reply(409, { error: code, message: "conflict" });
    const client = new DevBox({
      apiKey: "key",
      apiUrl: "https://manager.example.test",
      dispatcher: agent,
    });
    try {
      const sandbox = await client.sandboxes.create();
      if (code === "already_killed") {
        await expect(sandbox.kill()).resolves.toBe(false);
        expect(sandbox.info.state).toBe(SandboxState.Stopped);
      } else await expect(sandbox.kill()).rejects.toBeInstanceOf(ConflictError);
    } finally {
      await client.close();
    }
  });

  it("parses pagination headers", async () => {
    agent = mockAgent();
    agent
      .get("https://manager.example.test")
      .intercept({ path: "/v2/sandboxes?limit=10", method: "GET" })
      .reply(200, [sandboxResponse], {
        headers: { "X-Next-Token": "next", "X-Total-Running": "1" },
      });
    const client = new DevBox({
      apiKey: "key",
      apiUrl: "https://manager.example.test",
      dispatcher: agent,
    });
    await expect(client.sandboxes.list({ limit: 10 })).resolves.toMatchObject({
      items: [{ sandboxId: "sbx-1" }],
      nextToken: "next",
      total: 1,
    });
    await client.close();
  });
});
