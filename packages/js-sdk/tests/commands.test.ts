import { Buffer } from "node:buffer";
import type { MockAgent } from "undici";
import { afterEach, describe, expect, expectTypeOf, it, vi } from "vitest";
import { type CommandHandle, Commands, type RunOptions } from "../src/commands.js";
import { CommandExitError } from "../src/errors.js";
import { Transport } from "../src/internal/transport.js";
import type { CommandResult } from "../src/models.js";
import { connectBody, mockAgent } from "./helpers.js";

describe("Commands", () => {
  let agent: MockAgent | undefined;
  afterEach(async () => agent?.close());

  it("infers run results for literal and dynamic options", () => {
    const foreground = (commands: Commands) => commands.run("true");
    const background = (commands: Commands) => commands.run("true", { background: true });
    const dynamic = (commands: Commands, options: RunOptions) => commands.run("true", options);

    expectTypeOf(foreground).returns.toEqualTypeOf<Promise<CommandResult>>();
    expectTypeOf(background).returns.toEqualTypeOf<Promise<CommandHandle>>();
    expectTypeOf(dynamic).returns.toEqualTypeOf<Promise<CommandResult | CommandHandle>>();
  });

  it("collects stdout and stderr from the same event", async () => {
    agent = mockAgent();
    agent
      .get("https://runtime.example.test")
      .intercept({ path: "/process.Process/Start", method: "POST" })
      .reply(
        200,
        connectBody(
          { value: { event: { start: { pid: 42 } } } },
          {
            value: {
              event: {
                data: {
                  stdout: Buffer.from("out").toString("base64"),
                  stderr: Buffer.from("err").toString("base64"),
                },
              },
            },
          },
          { value: { event: { end: { exitCode: 0 } } } },
          { value: {}, trailer: true },
        ),
        { headers: { "Content-Type": "application/connect+json" } },
      );
    const transport = new Transport("https://runtime.example.test", { dispatcher: agent });
    const commands = new Commands(async () => transport);
    const stdout = vi.fn();
    const result = await commands.run("echo test", { onStdout: stdout });
    expect(result).toEqual({ exitCode: 0, stdout: "out", stderr: "err", pid: 42 });
    expect(stdout).toHaveBeenCalledWith("out");
  });

  it("raises CommandExitError when check is enabled", async () => {
    agent = mockAgent();
    agent
      .get("https://runtime.example.test")
      .intercept({ path: "/process.Process/Start", method: "POST" })
      .reply(
        200,
        connectBody(
          { value: { event: { start: { pid: 9 } } } },
          { value: { event: { end: { exitCode: 7 } } } },
          { value: {}, trailer: true },
        ),
        { headers: { "Content-Type": "application/connect+json" } },
      );
    const transport = new Transport("https://runtime.example.test", { dispatcher: agent });
    await expect(new Commands(async () => transport).run("false")).rejects.toBeInstanceOf(
      CommandExitError,
    );
  });

  it("rejects process control after completion", async () => {
    agent = mockAgent();
    agent
      .get("https://runtime.example.test")
      .intercept({ path: "/process.Process/Start", method: "POST" })
      .reply(
        200,
        connectBody(
          { value: { event: { start: { pid: 10 } } } },
          { value: { event: { end: { exitCode: 0 } } } },
          { value: {}, trailer: true },
        ),
        { headers: { "Content-Type": "application/connect+json" } },
      );
    const transport = new Transport("https://runtime.example.test", { dispatcher: agent });
    const process = await new Commands(async () => transport).run("true", { background: true });
    await process.wait();
    await expect(process.sendStdin("late")).rejects.toThrow("process has already completed");
  });
});
