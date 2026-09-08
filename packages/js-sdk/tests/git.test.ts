import type { MockAgent } from "undici";
import { afterEach, describe, expect, it } from "vitest";
import { Commands } from "../src/commands.js";
import { Git } from "../src/git.js";
import { Transport } from "../src/internal/transport.js";
import { connectBody, mockAgent } from "./helpers.js";

describe("Git", () => {
  let agent: MockAgent | undefined;
  afterEach(async () => agent?.close());

  it("passes credentials through environment variables instead of command arguments", async () => {
    agent = mockAgent();
    agent
      .get("https://runtime.example.test")
      .intercept({ path: "/process.Process/Start", method: "POST" })
      .reply(({ body }) => {
        const request = JSON.parse(connectRequestBody(body as Uint8Array));
        const process = request.process;
        expect(process.args.join(" ")).not.toContain("secret-token");
        expect(process.envs).toMatchObject({
          DEVBOX_GIT_USERNAME: "git-user",
          DEVBOX_GIT_PASSWORD: "secret-token",
          GIT_TERMINAL_PROMPT: "0",
        });
        return {
          statusCode: 200,
          data: connectBody(
            { value: { event: { start: { pid: 11 } } } },
            { value: { event: { end: { exitCode: 0 } } } },
            { value: {}, trailer: true },
          ),
          responseOptions: { headers: { "Content-Type": "application/connect+json" } },
        };
      });

    const transport = new Transport("https://runtime.example.test", { dispatcher: agent });
    const git = new Git(new Commands(async () => transport));
    await git.clone("https://example.test/repo.git", "/tmp/repo", {
      credentials: { username: "git-user", password: "secret-token" },
    });
  });
});

function connectRequestBody(value: Uint8Array): string {
  const body = Buffer.from(value);
  return body.subarray(5, 5 + body.readUInt32BE(1)).toString();
}
