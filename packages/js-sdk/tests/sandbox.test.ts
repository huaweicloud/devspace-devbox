import type { MockAgent } from "undici";
import { afterEach, describe, expect, it } from "vitest";
import { DevBox } from "../src/client.js";
import { SandboxState } from "../src/models.js";
import { mockAgent, sandboxResponse } from "./helpers.js";

describe("sandboxes", () => {
  let agent: MockAgent | undefined;
  afterEach(async () => agent?.close());

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
        expect(JSON.parse(String(body))).toMatchObject({
          templateID: "default",
          timeout: 600,
          secure: true,
          allow_internet_access: true,
          envVars: { MODE: "test" },
          network: { allowPublicTraffic: false },
        });
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

  it("parses pagination headers", async () => {
    agent = mockAgent();
    agent
      .get("https://manager.example.test")
      .intercept({ path: "/v2/sandboxes?limit=10", method: "GET" })
      .reply(200, [sandboxResponse.sandbox], {
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
