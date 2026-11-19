import type { MockAgent } from "undici";
import { afterEach, describe, expect, it } from "vitest";
import { DevBox } from "../src/client.js";
import { mockAgent } from "./helpers.js";

describe("templates", () => {
  let agent: MockAgent | undefined;

  afterEach(async () => {
    await agent?.close();
    agent = undefined;
  });

  it("lists templates available for sandbox creation", async () => {
    agent = mockAgent();
    agent
      .get("https://manager.example.test")
      .intercept({ path: "/templates", method: "GET" })
      .reply(200, [
        {
          templateID: "tpl-devbox-base-test",
          name: "devbox-base-test",
          aliases: ["default"],
          buildID: "devbox-base-test-aarch64-20261012032308",
          cpuCount: 2,
          memoryMB: 2500,
          diskSizeMB: 2040,
        },
      ]);

    const client = new DevBox({
      apiKey: "key",
      apiUrl: "https://manager.example.test",
      dispatcher: agent,
    });
    try {
      await expect(client.templates.list()).resolves.toEqual([
        {
          templateId: "tpl-devbox-base-test",
          name: "devbox-base-test",
          aliases: ["default"],
          buildId: "devbox-base-test-aarch64-20261012032308",
          cpuCount: 2,
          memoryMb: 2500,
          diskSizeMb: 2040,
        },
      ]);
      agent.assertNoPendingInterceptors();
    } finally {
      await client.close();
    }
  });
});
