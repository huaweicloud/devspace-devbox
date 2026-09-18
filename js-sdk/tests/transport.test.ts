import { ReadableStream } from "node:stream/web";
import { type MockAgent, Response } from "undici";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ProtocolError, RateLimitError, RequestTimeoutError } from "../src/errors.js";
import { ConnectStream, Transport } from "../src/internal/transport.js";
import { connectBody, connectFrame, mockAgent } from "./helpers.js";

describe("Transport", () => {
  let agent: MockAgent | undefined;
  afterEach(async () => {
    vi.restoreAllMocks();
    await agent?.close();
  });

  it("cancels the body when stream consumption stops early", async () => {
    const cancel = vi.fn();
    const response = new Response(
      new ReadableStream({
        start(controller) {
          controller.enqueue(connectFrame({ event: { start: { pid: 7 } } }));
        },
        cancel,
      }),
      { headers: { "Content-Type": "application/connect+json" } },
    );
    const stream = new ConnectStream(async () => response);
    for await (const event of stream) {
      expect(event).toEqual({ event: { start: { pid: 7 } } });
      break;
    }
    expect(cancel).toHaveBeenCalledOnce();
    expect(stream.signal.aborted).toBe(true);
  });

  it("cancels a stream with an invalid content type", async () => {
    const cancel = vi.fn();
    const response = new Response(new ReadableStream({ cancel }), {
      headers: { "Content-Type": "text/html" },
    });
    const stream = new ConnectStream(async () => response);
    await expect(stream[Symbol.asyncIterator]().next()).rejects.toThrow(ProtocolError);
    expect(cancel).toHaveBeenCalledOnce();
    expect(stream.signal.aborted).toBe(true);
  });

  it("closes its streams without closing a borrowed dispatcher", async () => {
    agent = mockAgent();
    const close = vi.spyOn(agent, "close");
    const transport = new Transport("https://runtime.test", { dispatcher: agent });
    const stream = transport.connectStream("/stream", {});
    await transport.close();
    expect(stream.signal.aborted).toBe(true);
    expect(close).not.toHaveBeenCalled();
  });

  it.each(["json", "bytes"])("maps timeouts while reading a %s response body", async (kind) => {
    agent = mockAgent();
    agent.get("https://runtime.test").intercept({ path: "/body", method: "GET" }).reply(200, "{}");
    const cause = new DOMException("body timed out", "TimeoutError");
    vi.spyOn(Response.prototype, kind === "json" ? "text" : "arrayBuffer").mockRejectedValueOnce(
      cause,
    );
    const transport = new Transport("https://runtime.test", { dispatcher: agent });
    const operation =
      kind === "json" ? transport.request("GET", "/body") : transport.requestBytes("GET", "/body");
    await expect(operation).rejects.toBeInstanceOf(RequestTimeoutError);
  });

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
