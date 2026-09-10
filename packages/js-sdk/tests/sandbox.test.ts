import type { MockAgent } from "undici";
import { afterEach, describe, expect, it } from "vitest";
import { DevBox } from "../src/client.js";
import { parseConnection, SandboxState } from "../src/models.js";
import { mockAgent, sandboxResponse } from "./helpers.js";

describe("sandboxes", () => {
  let agent: MockAgent | undefined;
  afterEach(async () => agent?.close());

  it("parses the manager tunnel connection contract", () => {
    expect(parseConnection(sandboxResponse, "sbx-1")).toEqual({
      sandboxId: "sbx-1",
      domain: "https://runtime.example.test",
      envdAccessToken: "envd-token",
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
