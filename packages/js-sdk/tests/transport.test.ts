import type { MockAgent } from "undici";
import { afterEach, describe, expect, it } from "vitest";
import { ProtocolError, RateLimitError } from "../src/errors.js";
import { Transport } from "../src/internal/transport.js";
import { connectBody, mockAgent } from "./helpers.js";

describe("Transport", () => {
  let agent: MockAgent | undefined;
  afterEach(async () => agent?.close());

  it("maps structured API errors", async () => {
    agent = mockAgent();
    agent
      .get("https://manager.example.test")
      .intercept({ path: "/limited", method: "GET" })
      .reply(
        429,
        { error: { code: "rate_limited", message: "slow down", target: "request" } },
        { headers: { "Retry-After": "2", "X-Request-Id": "req-1" } },
      );
    const transport = new Transport("https://manager.example.test", { dispatcher: agent });

    const error = await transport.request("GET", "/limited").catch((value) => value);
    expect(error).toBeInstanceOf(RateLimitError);
    expect(error).toMatchObject({
      code: "rate_limited",
      statusCode: 429,
      retryAfter: 2,
      requestId: "req-1",
    });
  });

  it("rejects redirects instead of forwarding credentials", async () => {
    agent = mockAgent();
    agent
      .get("https://manager.example.test")
      .intercept({ path: "/redirect", method: "GET" })
      .reply(302, "", { headers: { Location: "https://other.test" } });
    const transport = new Transport("https://manager.example.test", { dispatcher: agent });
    await expect(transport.request("GET", "/redirect")).rejects.toThrow(ProtocolError);
  });

  it("decodes a complete Connect stream", async () => {
    agent = mockAgent();
    agent
      .get("https://runtime.example.test")
      .intercept({ path: "/stream", method: "POST" })
      .reply(
        200,
        connectBody({ value: { event: { start: { pid: 7 } } } }, { value: {}, trailer: true }),
        { headers: { "Content-Type": "application/connect+json" } },
      );
    const transport = new Transport("https://runtime.example.test", { dispatcher: agent });
    const events = [];
    for await (const event of transport.connectStream("/stream", {})) events.push(event);
    expect(events).toEqual([{ event: { start: { pid: 7 } } }]);
  });
});
