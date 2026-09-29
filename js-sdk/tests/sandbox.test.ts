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

  it("pauses and resumes with fresh runtime credentials", async () => {
    agent = mockAgent();
    const manager = agent.get("https://manager.example.test");
    manager.intercept({ path: "/sandboxes", method: "POST" }).reply(201, sandboxResponse);
    manager.intercept({ path: "/sandboxes/sbx-1/pause", method: "POST" }).reply(204);
    manager.intercept({ path: "/sandboxes/sbx-1/resume", method: "POST" }).reply(({ body }) => {
      expect(JSON.parse(String(body))).toEqual({ timeout: 600 });
      return { statusCode: 201, data: { ...sandboxResponse, connectToken: "new-token" } };
    });
    const runtime = agent.get("https://runtime.example.test");
    for (const token of ["connect-token", "new-token"]) {
      runtime
        .intercept({
          path: "/files?path=%2Ftmp%2Fproof",
          method: "GET",
          headers: { cookie: `relay_token=${token}` },
        })
        .reply(200, "proof");
    }
    const client = new DevBox({
      apiKey: "key",
      apiUrl: "https://manager.example.test",
      dispatcher: agent,
    });
    const sandbox = await client.sandboxes.create();
    try {
      expect(await sandbox.files.read("/tmp/proof")).toBe("proof");
      await sandbox.pause();
      expect(sandbox.info.state).toBe(SandboxState.Paused);
      await expect(sandbox.files.read("/tmp/proof")).rejects.toThrow("sandbox is paused");
      expect(await sandbox.resume({ timeout: 600 })).toBe(sandbox);
      expect(sandbox.info.state).toBe(SandboxState.Running);
      expect(await sandbox.files.read("/tmp/proof")).toBe("proof");
      agent.assertNoPendingInterceptors();
    } finally {
      await sandbox.close();
      await client.close();
    }
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

  it("refreshes an expiring connect token before opening the gateway", async () => {
    agent = mockAgent();
    const manager = agent.get("https://manager.example.test");
    manager.intercept({ path: "/sandboxes", method: "POST" }).reply(201, {
      ...sandboxResponse,
      connectToken: "old-token",
      tokenExpiration: 1,
    });
    manager.intercept({ path: "/sandboxes/sbx-1/connect", method: "POST" }).reply(200, {
      ...sandboxResponse,
      connectToken: "new-token",
      tokenExpiration: 4_000_000_000,
    });
    agent
      .get("https://runtime.example.test")
      .intercept({
        path: "/files?path=%2Ftmp%2Fproof",
        method: "GET",
        headers: { cookie: "relay_token=new-token", "e2b-sandbox-id": "sbx-1" },
      })
      .reply(200, "proof");

    const client = new DevBox({
      apiKey: "key",
      apiUrl: "https://manager.example.test",
      dispatcher: agent,
    });
    const sandbox = await client.sandboxes.create();
    try {
      expect(await sandbox.files.read("/tmp/proof")).toBe("proof");
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
      tokenExpiration: 4_000_000_000,
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

  it("serializes auto-resume lifecycle and lets paused traffic reach the gateway", async () => {
    agent = mockAgent();
    const manager = agent.get("https://manager.example.test");
    manager.intercept({ path: "/sandboxes", method: "POST" }).reply(({ body }) => {
      expect(JSON.parse(String(body))).toMatchObject({
        autoPause: true,
        autoPauseMemory: true,
        autoResume: { enabled: true },
      });
      return {
        statusCode: 201,
        data: {
          ...sandboxResponse,
          state: "paused",
          lifecycle: { onTimeout: "pause", autoResume: true },
        },
      };
    });
    agent
      .get("https://runtime.example.test")
      .intercept({ path: "/files?path=%2Ftmp%2Fproof", method: "GET" })
      .reply(200, "proof");
    const client = new DevBox({
      apiKey: "key",
      apiUrl: "https://manager.example.test",
      dispatcher: agent,
    });
    const sandbox = await client.sandboxes.create("default", {
      lifecycle: { onTimeout: "pause", autoResume: true },
    });
    expect(await sandbox.files.read("/tmp/proof")).toBe("proof");
    agent.assertNoPendingInterceptors();
    await sandbox.close();
    await client.close();
  });

  it("rejects auto-resume without pause", async () => {
    agent = mockAgent();
    const client = new DevBox({
      apiKey: "key",
      apiUrl: "https://manager.example.test",
      dispatcher: agent,
    });
    await expect(
      client.sandboxes.create("default", {
        lifecycle: { onTimeout: "kill", autoResume: true },
      }),
    ).rejects.toThrow("requires");
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

  it("supports lifecycle timeouts up to 24 hours", async () => {
    agent = mockAgent();
    const pool = agent.get("https://manager.example.test");
    pool.intercept({ path: "/sandboxes", method: "POST" }).reply(201, sandboxResponse);
    pool.intercept({ path: "/sandboxes/sbx-1/timeout", method: "POST" }).reply(({ body }) => {
      expect(JSON.parse(String(body))).toEqual({ timeout: 7200 });
      return { statusCode: 204, data: "" };
    });
    pool.intercept({ path: "/sandboxes/sbx-1/refreshes", method: "POST" }).reply(({ body }) => {
      expect(JSON.parse(String(body))).toEqual({ duration: 86400 });
      return { statusCode: 204, data: "" };
    });
    const client = new DevBox({
      apiKey: "key",
      apiUrl: "https://manager.example.test",
      dispatcher: agent,
    });
    const sandbox = await client.sandboxes.create();
    await sandbox.setTimeout(7200);
    await sandbox.refresh(86400);
    await expect(sandbox.setTimeout(90000)).rejects.toThrow("86400");
    agent.assertNoPendingInterceptors();
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
      .intercept({ path: "/sandboxes?limit=10", method: "GET" })
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
