import { Buffer } from "node:buffer";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { setTimeout as sleep } from "node:timers/promises";
import { pathToFileURL } from "node:url";
import { parseArgs, stripVTControlCharacters } from "node:util";
import {
  CommandExitError,
  DevBox,
  DevBoxError,
  NotFoundError,
  SandboxState,
} from "../dist/index.js";

class Validator {
  failures = [];
  tests = 0;
  timings = new Map();
  started = performance.now();

  async verify(name, operation) {
    this.tests += 1;
    const started = performance.now();
    console.log(`\nTEST ${String(this.tests).padStart(2, "0")} | ${name}`);
    try {
      const result = await operation();
      console.log(`PASS ${name} | ${seconds(started)}s`);
      return result;
    } catch (error) {
      this.failures.push(name);
      const detail =
        error instanceof DevBoxError
          ? ` | http=${error.statusCode} request_id=${error.requestId}`
          : "";
      console.log(`FAIL ${name} | ${seconds(started)}s | ${error.name}: ${error.message}${detail}`);
      return undefined;
    } finally {
      this.timings.set(name, (performance.now() - started) / 1000);
    }
  }
}

export async function main({ args = process.argv.slice(2) } = {}) {
  const { values } = parseArgs({
    args,
    options: {
      mode: { type: "string", default: "full" },
      rounds: { type: "string", default: "1" },
      help: { type: "boolean", short: "h" },
    },
  });
  if (values.help) {
    console.log(
      "Usage: node examples/validate-full.mjs [--mode basic|lifecycle|full] [--rounds N]",
    );
    return;
  }
  const rounds = Number(values.rounds);
  if (!["basic", "lifecycle", "full"].includes(values.mode))
    throw new Error("--mode must be basic, lifecycle or full");
  if (!Number.isSafeInteger(rounds) || rounds < 1)
    throw new Error("--rounds must be a positive integer");
  const runs = [];
  for (let number = 1; number <= rounds; number += 1) {
    console.log(`\nROUND ${number}/${rounds}`);
    const result = await runRound(values.mode);
    runs.push(result);
    if (
      !result.timings.has("sandbox.create") ||
      result.failures.some((name) =>
        ["sandbox.create", "sandbox.delete", "lifecycle.timeoutKill"].includes(name),
      )
    ) {
      console.log("STOP batch: creation or cleanup failed; inspect before continuing");
      break;
    }
  }
  const passed = runs.filter((run) => !run.failures.length).length;
  if (rounds > 1) {
    console.log(`\nBATCH attempted=${runs.length} passed=${passed} failed=${runs.length - passed}`);
    for (const name of new Set(runs.flatMap((run) => [...run.timings.keys()]))) {
      const times = runs.filter((run) => run.timings.has(name)).map((run) => run.timings.get(name));
      const failures = runs.filter((run) => run.failures.includes(name)).length;
      const avg = times.reduce((sum, time) => sum + time, 0) / times.length;
      console.log(
        `  ${name} | n=${times.length} failed=${failures} | min=${Math.min(...times).toFixed(3)}s avg=${avg.toFixed(3)}s max=${Math.max(...times).toFixed(3)}s`,
      );
    }
  }
  if (passed !== rounds) process.exitCode = 1;
}

async function runRound(mode) {
  const template = process.env.DEVBOX_TEST_TEMPLATE || "default";
  const validator = new Validator();
  const client = new DevBox();
  let sandbox;
  let selected;
  console.log(`DevBox ${mode} validation | template=${template}`);
  const gatewayUrl = process.env.DEVBOX_GATEWAY_URL;
  if (gatewayUrl && !gatewayUrl.includes("{tunnel_id}")) {
    await client.close();
    throw new Error(
      "Validation requires the created sandbox: unset DEVBOX_GATEWAY_URL or use {tunnel_id}",
    );
  }
  try {
    section("Templates", "List available templates and select a concrete build");
    selected = await validator.verify("templates.list", () => selectTemplate(client, template));
    if (selected) {
      section(
        "Create sandbox",
        `Start template=${selected.name}, timeout=300s, with an isolated runtime`,
      );
      const lifecycle = mode !== "basic" ? { onTimeout: "pause", autoResume: true } : undefined;
      sandbox = await validator.verify("sandbox.create", () =>
        step(
          `sandboxes.create({timeout:300, envs:..., lifecycle:${JSON.stringify(lifecycle)}})`,
          () =>
            client.sandboxes.create(selected.templateId, {
              buildId: selected.buildId,
              timeout: 300,
              envs: { DEVBOX_SDK_ENV: "created-731" },
              metadata: { sdk_validation: "javascript" },
              lifecycle,
            }),
        ),
      );
      if (sandbox) {
        console.log(`sandboxId=${sandbox.sandboxId}`);
        if (mode !== "lifecycle") await validateSandbox(client, sandbox, validator);
        if (mode !== "basic") await validateLifecycle(client, sandbox, validator);
      } else console.log("STOP sandbox creation failed; runtime validation cannot continue");
    }
  } finally {
    try {
      if (sandbox) {
        section("Cleanup", "Delete the remote sandbox and close local connections");
        await validator.verify("sandbox.delete", () =>
          step("sandbox.kill()", () => sandbox.kill()),
        );
      }
    } finally {
      await sandbox?.close();
      await client.close();
    }
  }
  if (sandbox && mode !== "basic" && !validator.failures.includes("sandbox.delete")) {
    const expiryClient = new DevBox();
    try {
      await validator.verify("lifecycle.timeoutKill", () => validateExpiry(expiryClient, selected));
    } finally {
      await expiryClient.close();
    }
  }
  console.log(
    `SUMMARY tests=${validator.tests} passed=${validator.tests - validator.failures.length} failed=${validator.failures.length} elapsed=${seconds(validator.started)}s`,
  );
  if (validator.failures.length) {
    console.log(`FAILED ${validator.failures.join(", ")}`);
  }
  return validator;
}

async function selectTemplate(client, name) {
  const templates = await step("templates.list()", () => client.templates.list());
  for (const item of templates) {
    console.log(
      `    ${item.name} | id=${item.templateId} | build=${item.buildId} | aliases=${item.aliases.join(",")} | cpu=${item.cpuCount} | memory=${item.memoryMb} MiB`,
    );
  }
  const selected = templates.find((item) =>
    [item.name, item.templateId, ...item.aliases].includes(name),
  );
  if (!selected)
    throw new Error(`template ${JSON.stringify(name)} is not in the available catalog`);
  console.log(`    SELECT ${selected.name} | build=${selected.buildId}`);
  return selected;
}

async function validateSandbox(client, sandbox, validator) {
  section("Sandbox management", "Inspect the sandbox, list it and obtain a new SDK handle");

  await validator.verify("manager.get", async () =>
    equal((await step("sandbox.getInfo()", () => sandbox.getInfo())).sandboxId, sandbox.sandboxId),
  );
  await validator.verify("manager.isRunning", async () =>
    equal(await step("sandbox.isRunning()", () => sandbox.isRunning()), true),
  );
  await validator.verify("manager.list", async () => {
    const page = await step("sandboxes.list(limit=100)", () =>
      client.sandboxes.list({ limit: 100 }),
    );
    console.log(`    sandboxes=${page.items.length} | current sandbox must be present`);
    equal(
      page.items.some((item) => item.sandboxId === sandbox.sandboxId),
      true,
    );
  });
  await validator.verify("manager.connect", async () => {
    const connected = await step(
      `sandboxes.connect(${sandbox.sandboxId}) | open a new SDK handle`,
      () => client.sandboxes.connect(sandbox.sandboxId),
    );
    try {
      equal(connected.sandboxId, sandbox.sandboxId);
    } finally {
      await step("connected.close() | keep the remote sandbox running", () => connected.close());
    }
  });
  const runtime = await validator.verify("runtime.ready", () => waitForRuntime(sandbox));
  if (!runtime) {
    console.log("SKIP EnvD, commands, filesystem, PTY and Git: runtime gateway is unavailable");
  } else {
    await validator.verify("runtime.environment", async () => {
      const command = 'printf "%s|%s" "$DEVBOX_ID" "$DEVBOX_SDK_ENV"';
      const result = await step(
        `commands.run(${JSON.stringify(command)}) | verify sandbox identity and create-time environment`,
        () => sandbox.commands.run(command),
      );
      equal(result.stdout, `${sandbox.sandboxId}|created-731`);
      equal(result.stderr, "");
    });
    section("EnvD runtime", "Probe EnvD and collect realtime metrics and runtime environment");
    await validator.verify("runtime.envd", async () => {
      const health = await step(
        "envd.health() | probe the data plane and read the EnvD version",
        () => sandbox.envd.health(),
      );
      console.log(`    envdVersion=${JSON.stringify(health.envdVersion)}`);
      equal(Boolean(health.envdVersion), true);

      const metrics = await step(
        "envd.getMetrics() | collect one realtime CPU, memory and disk sample",
        () => sandbox.envd.getMetrics(),
      );
      console.log(
        `    cpu=${metrics.cpuCount}x ${metrics.cpuUsedPct.toFixed(2)}%` +
          ` | hotplug possible=${metrics.cpuPossible} target=${metrics.cpuTarget}` +
          ` attempts=${metrics.cpuTargetAttempts} pending=${metrics.cpuWritePendingMs}ms` +
          ` | memory=${metrics.memUsed}/${metrics.memTotal} bytes` +
          ` | disk=${metrics.diskUsed}/${metrics.diskTotal} bytes`,
      );
      equal(metrics.cpuCount > 0, true);
      equal(metrics.cpuPossible !== undefined, true);
      equal(metrics.cpuTarget !== undefined, true);
      equal(metrics.cpuTargetAttempts !== undefined, true);
      equal(metrics.cpuWritePendingMs !== undefined, true);
      equal(metrics.cpuPossible >= metrics.cpuCount, true);
      equal(metrics.cpuTarget >= 0, true);
      equal(metrics.cpuTargetAttempts >= 0, true);
      equal(metrics.cpuWritePendingMs >= 0, true);
      equal(metrics.cpuUsedPct >= 0 && metrics.cpuUsedPct <= 100, true);
      equal(metrics.memUsed >= 0 && metrics.memUsed <= metrics.memTotal, true);
      equal(metrics.memCache >= 0 && metrics.memCache <= metrics.memTotal, true);
      equal(metrics.diskUsed > 0 && metrics.diskUsed <= metrics.diskTotal, true);
      const age = Math.abs(Date.now() - metrics.timestamp.getTime()) / 1000;
      if (age > 60)
        throw new Error(`EnvD metric timestamp is ${age.toFixed(1)}s away from local clock`);
      console.log(`    oomKills=${metrics.oomKills?.length ?? "unknown"}`);
      for (const kill of metrics.oomKills ?? []) {
        equal(kill.seq >= 0, true);
        equal(Boolean(kill.process), true);
      }

      const envs = await step("envd.getEnvs() | read the merged runtime environment", () =>
        sandbox.envd.getEnvs(),
      );
      console.log(`    envKeys=${JSON.stringify(Object.keys(envs).sort())}`);
      equal(envs.DEVBOX_ID, sandbox.sandboxId);
      equal(envs.DEVBOX_TEMPLATE_ID, sandbox.info.templateId);
      equal(envs.DEVBOX_SDK_ENV, "created-731");
    });
    await validateRuntime(sandbox, validator);
  }
}

async function waitForRuntime(sandbox) {
  const result = await step(
    "commands.run('printf runtime-ready') | verify remote execution without readiness retries",
    () => sandbox.commands.run("printf runtime-ready"),
  );
  equal(result.stdout, "runtime-ready");
  equal(result.stderr, "");
  return true;
}

async function validateLifecycle(client, sandbox, validator) {
  section(
    "Lifecycle",
    "Keep files, environment and the original process across pause and recovery",
  );
  const path = `/tmp/devbox-lifecycle-${Date.now()}`;
  const payload = Buffer.alloc(1024 * 1024);
  for (let i = 0; i < payload.length; i++) payload[i] = i % 256;
  let process;
  const resumeTimings = new Map();
  const persistence = async (handle = sandbox) => {
    const content = await step("files.readBytes() | compare all 1 MiB after recovery", () =>
      handle.files.readBytes(path),
    );
    equal(Buffer.from(content).equals(payload), true);
    console.log("    CHECK file content: identical (1 MiB)");
    const running = await step("commands.list() | find the original running PID", () =>
      handle.commands.list(),
    );
    equal(
      running.some((item) => item.pid === process.pid && item.running),
      true,
    );
    console.log(`    CHECK original process: pid=${process.pid}, still running`);
    const result = await step("commands.run() | verify sandbox identity and environment", () =>
      handle.commands.run('printf "%s|%s" "$DEVBOX_ID" "$DEVBOX_SDK_ENV"'),
    );
    equal(result.stdout, `${handle.sandboxId}|created-731`);
    equal(result.stderr, "");
    equal((await step("sandbox.getInfo()", () => handle.getInfo())).state, SandboxState.Running);
  };
  const automaticResume = async (freshConnection) => {
    await step("setTimeout(10) | onTimeout=pause, autoResume=true", () => sandbox.setTimeout(10));
    await waitForState(sandbox, SandboxState.Paused);
    if (freshConnection)
      await step("sandbox.close() | force a fresh data-plane connection", () => sandbox.close());
    else console.log("    Reuse the SDK handle and connection pool without close().");
    console.log("    FLOW SDK request -> Gateway -> Manager gRPC -> restored runtime");
    console.log("    No explicit sandbox.resume() or sandboxes.connect() call follows.");
    const result = await step(
      "commands.run('printf gateway-wake-ok') | access the paused sandbox directly",
      () => sandbox.commands.run("printf gateway-wake-ok", { timeoutMs: 90_000 }),
    );
    equal(result.stdout, "gateway-wake-ok");
    equal(result.stderr, "");
    await persistence();
  };
  const liveUpgrade = async () => {
    const before = await step("envd.health() | read the EnvD version before pause", () =>
      sandbox.envd.health(),
    );

    const pauseStarted = performance.now();
    await step("sandbox.pause() | capture the running EnvD state", () => sandbox.pause());
    const pauseElapsed = (performance.now() - pauseStarted) / 1000;
    await step("sandbox.close() | resume through a fresh SDK handle", () => sandbox.close());

    const resumeStarted = performance.now();
    const connected = await step(
      "sandboxes.connect(id, {timeout:600}) | Manager restores and may live-upgrade EnvD",
      () => client.sandboxes.connect(sandbox.sandboxId, { timeout: 600 }),
    );
    const resumeElapsed = (performance.now() - resumeStarted) / 1000;
    sandbox = connected;

    const after = await step("envd.health() | read the EnvD version after resume", () =>
      sandbox.envd.health(),
    );
    const upgradeWindow = (performance.now() - pauseStarted) / 1000;
    console.log(
      `    before=${JSON.stringify(before.envdVersion)} after=${JSON.stringify(after.envdVersion)} ` +
        `| pause=${pauseElapsed.toFixed(3)}s ` +
        `| resume=${resumeElapsed.toFixed(3)}s ` +
        `| upgradeWindow=${upgradeWindow.toFixed(3)}s`,
    );

    if (versionKey(after.envdVersion) === versionKey(before.envdVersion)) {
      console.log("    SKIP live upgrade | target EnvD is not newer; verify normal resume instead");
    } else if (versionKey(after.envdVersion) < versionKey(before.envdVersion)) {
      throw new Error(`EnvD was downgraded: ${before.envdVersion} -> ${after.envdVersion}`);
    } else {
      console.log("    CHECK EnvD was upgraded during resume");
      resumeTimings.set("upgraded", [pauseElapsed, resumeElapsed]);
    }

    await persistence(sandbox);
    const result = await step(
      "commands.run('printf live-upgrade-probe-ok') | execute a new process",
      () => sandbox.commands.run("printf live-upgrade-probe-ok", { timeoutMs: 30_000 }),
    );
    equal(result.stdout, "live-upgrade-probe-ok");
    equal(result.stderr, "");
  };
  const stages = [
    [
      "lifecycle.prepare",
      async () => {
        await step(`files.write(${path}, <1 MiB>) | persistence proof`, () =>
          sandbox.files.write(path, payload),
        );
        process = await step("commands.run('sleep 900', background=true)", () =>
          sandbox.commands.run("sleep 900", { background: true }),
        );
        await step("process.disconnect() | keep the remote process alive", () =>
          process.disconnect(),
        );
      },
    ],
    ["lifecycle.liveUpgrade", () => liveUpgrade()],
    [
      "lifecycle.deadlines",
      async () => {
        await step("setTimeout(7200) | remaining lifetime becomes 2h (greater than 1h)", () =>
          sandbox.setTimeout(7200),
        );
        const extended = await deadline(sandbox, 7200);
        await step("refresh(120) | must not shorten the existing 2h deadline", () =>
          sandbox.refresh(120),
        );
        equal(
          (await step("getInfo() | compare deadline", () => sandbox.getInfo())).endAt?.getTime(),
          extended.getTime(),
        );
        await step("setTimeout(300) | explicitly shorten remaining lifetime to 5min", () =>
          sandbox.setTimeout(300),
        );
        await deadline(sandbox, 300);
        await step("refresh(600) | extend remaining lifetime to at least 10min", () =>
          sandbox.refresh(600),
        );
        await deadline(sandbox, 600);
      },
    ],
    [
      "lifecycle.manualResume",
      async () => {
        await step("sandbox.pause() | save the running instance", () => sandbox.pause());
        equal(
          (await step("getInfo() | expect paused", () => sandbox.getInfo())).state,
          SandboxState.Paused,
        );
        await step("sandbox.resume({timeout:300}) | restore this sandbox handle", () =>
          sandbox.resume({ timeout: 300 }),
        );
        await deadline(sandbox, 300);
        await persistence();
      },
    ],
    [
      "lifecycle.connectResume",
      async () => {
        const pauseStarted = performance.now();
        await step("sandbox.pause()", () => sandbox.pause());
        const pauseElapsed = (performance.now() - pauseStarted) / 1000;
        await step("sandbox.close() | close local connections, not the sandbox", () =>
          sandbox.close(),
        );
        const resumeStarted = performance.now();
        const connected = await step(
          "sandboxes.connect(id, {timeout:300}) | Manager restores the paused sandbox",
          () => client.sandboxes.connect(sandbox.sandboxId, { timeout: 300 }),
        );
        const resumeElapsed = (performance.now() - resumeStarted) / 1000;
        resumeTimings.set("normal", [pauseElapsed, resumeElapsed]);
        console.log(
          `    TIMING normal resume | pause=${pauseElapsed.toFixed(3)}s ` +
            `| resume=${resumeElapsed.toFixed(3)}s ` +
            `| total=${(pauseElapsed + resumeElapsed).toFixed(3)}s`,
        );
        if (resumeTimings.has("upgraded")) {
          const [upgradedPause, upgradedResume] = resumeTimings.get("upgraded");
          const resumeOverhead = upgradedResume - resumeElapsed;
          const totalOverhead = upgradedPause + upgradedResume - pauseElapsed - resumeElapsed;
          console.log(
            "    TIMING resume comparison | upgraded " +
              `pause=${upgradedPause.toFixed(3)}s resume=${upgradedResume.toFixed(3)}s ` +
              `total=${(upgradedPause + upgradedResume).toFixed(3)}s` +
              ` | normal pause=${pauseElapsed.toFixed(3)}s ` +
              `resume=${resumeElapsed.toFixed(3)}s ` +
              `total=${(pauseElapsed + resumeElapsed).toFixed(3)}s` +
              ` | resume overhead=${resumeOverhead >= 0 ? "+" : ""}${resumeOverhead.toFixed(3)}s ` +
              `(${((resumeOverhead / resumeElapsed) * 100).toFixed(1)}%)` +
              ` | total overhead=${totalOverhead >= 0 ? "+" : ""}${totalOverhead.toFixed(3)}s ` +
              `(${((totalOverhead / (pauseElapsed + resumeElapsed)) * 100).toFixed(1)}%)`,
          );
        }
        try {
          equal(connected.sandboxId, sandbox.sandboxId);
          const content = await step("connected.files.readBytes() | same remote filesystem", () =>
            connected.files.readBytes(path),
          );
          equal(Buffer.from(content).equals(payload), true);
          await deadline(connected, 300);
          console.log("    SWITCH sandbox handle to connected | use the resume response token");
          await persistence(connected);
        } finally {
          await step("connected.close() | release the extra SDK handle", () => connected.close());
        }
        sandbox = connected;
      },
    ],
    ["lifecycle.gatewayAutoResume", () => automaticResume(true)],
    ["lifecycle.gatewayReusedConnection", () => automaticResume(false)],
  ];
  try {
    for (const [name, operation] of stages) {
      const failures = validator.failures.length;
      await validator.verify(name, operation);
      if (validator.failures.length !== failures) {
        console.log("STOP dependent lifecycle steps; cleanup will still run");
        return;
      }
    }
  } finally {
    await sandbox.close();
  }
}

async function deadline(sandbox, seconds) {
  const info = await step("sandbox.getInfo() | read the real Manager deadline", () =>
    sandbox.getInfo(),
  );
  if (!info.endAt) throw new Error("Manager did not return an expiration time");
  const remaining = (info.endAt.getTime() - Date.now()) / 1000;
  if (remaining < seconds - 15 || remaining > seconds + 5)
    throw new Error(`expected about ${seconds}s remaining, received ${remaining.toFixed(1)}s`);
  console.log(
    `    CHECK remaining=${remaining.toFixed(1)}s | expires_at=${info.endAt.toISOString()}`,
  );
  return info.endAt;
}

async function waitForState(sandbox, expected) {
  console.log(`  WAIT Manager state=${expected} | polling every 2s, maximum 90s`);
  const started = performance.now();
  let previous;
  while (performance.now() - started < 90_000) {
    let state;
    try {
      state = (await sandbox.getInfo()).state;
    } catch (error) {
      if (!(error instanceof NotFoundError) || expected !== SandboxState.Stopped) throw error;
      state = SandboxState.Stopped;
    }
    if (state !== previous) console.log(`    STATE ${state} | elapsed=${seconds(started)}s`);
    previous = state;
    if (state === expected) return;
    await sleep(2_000);
  }
  throw new Error(`sandbox did not reach ${expected} within 90s`);
}

async function validateExpiry(client, template) {
  section("Timeout cleanup", "A separate sandbox uses onTimeout=kill, not pause");
  const sandbox = await step("sandboxes.create(timeout=10, lifecycle=kill)", () =>
    client.sandboxes.create(template.templateId, { buildId: template.buildId, timeout: 10 }),
  );
  console.log(`    sandboxId=${sandbox.sandboxId}`);
  try {
    await waitForState(sandbox, SandboxState.Stopped);
    equal(await step("sandbox.isRunning() | expect false", () => sandbox.isRunning()), false);
  } finally {
    try {
      await step("sandbox.kill() | cleanup is safe even after expiration", () => sandbox.kill());
    } finally {
      await sandbox.close();
    }
  }
}

async function validateRuntime(sandbox, validator) {
  section("Commands", "Capture stdout/stderr, run background jobs and send standard input");
  await validator.verify("commands.foreground", async () => {
    const command = 'printf "$DEVBOX_TEST"; printf error-ok >&2';
    const result = await step(
      `commands.run(${JSON.stringify(command)}) | cwd=/tmp, DEVBOX_TEST=command-ok`,
      () =>
        sandbox.commands.run(command, {
          envs: { DEVBOX_TEST: "command-ok" },
          cwd: "/tmp",
        }),
    );
    equal(result.stdout, "command-ok");
    equal(result.stderr, "error-ok");
    const failed = await step(
      "commands.run('printf partial; printf deliberate >&2; exit 7', check=false)",
      () => sandbox.commands.run("printf partial; printf deliberate >&2; exit 7", { check: false }),
    );
    equal(failed.exitCode, 7);
    equal(failed.stdout, "partial");
    equal(failed.stderr, "deliberate");
  });

  await validator.verify("commands.background", async () => {
    const process = await step(
      "commands.run('sleep 1; printf background-ok', background=true, timeoutMs=10000)",
      () =>
        sandbox.commands.run("sleep 1; printf background-ok", {
          background: true,
          timeoutMs: 10_000,
        }),
    );
    console.log("    SDK returned a handle; the remote process can continue in the background");
    equal(
      (
        await step(
          `process.wait(pid=${process.pid}) | wait for completion and collect output`,
          () => process.wait(),
        )
      ).stdout,
      "background-ok",
    );
  });

  await validator.verify("commands.stdin", async () => {
    const process = await step(
      "commands.run('cat', background=true, stdin=true, timeoutMs=10000)",
      () =>
        sandbox.commands.run("cat", {
          background: true,
          stdin: true,
          timeoutMs: 10_000,
        }),
    );
    await step("process.sendStdin('stdin-ok\\n')", () => process.sendStdin("stdin-ok\n"));
    await step("process.closeStdin() | send EOF so cat can finish", () => process.closeStdin());
    equal(
      (await step("process.wait() | collect the echoed input", () => process.wait())).stdout,
      "stdin-ok\n",
    );
  });

  await validator.verify("commands.reconnect", async () => {
    const process = await step(
      "commands.run('read line; printf reconnected:%s \"$line\"', background=true, stdin=true)",
      () =>
        sandbox.commands.run('read line; printf reconnected:%s "$line"', {
          background: true,
          stdin: true,
          timeoutMs: 10_000,
        }),
    );
    await step("process.disconnect() | remote process stays alive", () => process.disconnect());
    const attached = await step(`commands.connect(${process.pid})`, () =>
      sandbox.commands.connect(process.pid, { timeoutMs: 10_000 }),
    );
    await step("attached.sendStdin('proof-731\\n')", () => attached.sendStdin("proof-731\n"));
    const result = await step("attached.wait() | verify output after reconnect", () =>
      attached.wait(),
    );
    equal(result.stdout, "reconnected:proof-731");
    equal(result.stderr, "");
  });

  await validator.verify("commands.streaming", async () => {
    const chunks = [];
    const result = await step(
      "commands.run(..., onStdout) | receive output while the command is running",
      () =>
        sandbox.commands.run("printf 'step-1\\n'; sleep 1; printf 'step-2\\n'", {
          onStdout: (chunk) => {
            chunks.push(chunk);
            console.log(`    STREAM stdout=${JSON.stringify(chunk)}`);
          },
        }),
    );
    equal(result.stdout, "step-1\nstep-2\n");
    equal(result.stderr, "");
    equal(chunks.join(""), result.stdout);
  });
  await validator.verify("commands.signal", async () => {
    const process = await step("commands.run('sleep 60', background=true)", () =>
      sandbox.commands.run("sleep 60", { background: true }),
    );
    try {
      await step("process.sendSignal('SIGTERM') | terminate only this process", () =>
        process.sendSignal("SIGTERM"),
      );
      const result = await step(
        "process.wait({check:false}) | collect the termination status",
        () => process.wait({ check: false }),
      );
      if (result.exitCode === 0) throw new Error("terminated sleep reported a successful exit");
    } finally {
      await process.disconnect();
    }
  });

  section("Files", "Create, read, inspect, move, upload, download and remove files");
  await validator.verify("filesystem", async () => {
    const root = "/tmp/devbox-sdk-test";
    const path = `${root}/input.txt`;
    await step(`files.makeDir(${root})`, () => sandbox.files.makeDir(root));
    await step(`files.write(${path}, 'file-ok')`, () => sandbox.files.write(path, "file-ok"));
    equal(await step(`files.read(${path})`, () => sandbox.files.read(path)), "file-ok");
    equal(
      await step(
        `files.stat(${path}) | size in bytes`,
        async () => (await sandbox.files.stat(path)).size,
      ),
      7,
    );
    const entries = await step(`files.list(${root})`, () => sandbox.files.list(root));
    console.log(`    entries=${JSON.stringify(entries.map((item) => item.name))}`);
    equal(
      entries.some((item) => item.name === "input.txt"),
      true,
    );
    await step(`files.move(${path}, ${root}/output.txt)`, () =>
      sandbox.files.move(path, `${root}/output.txt`),
    );
    await step(`files.remove(${root})`, () => sandbox.files.remove(root));
  });

  await validator.verify("filesystem.uploadDownload", async () => {
    const binary = Buffer.alloc(1024 * 1024);
    for (let i = 0; i < binary.length; i++) binary[i] = i % 256;
    await step("files.write(/tmp/binary.dat, <1 MiB binary data>)", () =>
      sandbox.files.write("/tmp/binary.dat", binary),
    );
    const downloaded = await step("files.readBytes(/tmp/binary.dat)", () =>
      sandbox.files.readBytes("/tmp/binary.dat"),
    );
    equal(Buffer.from(downloaded).equals(binary), true);
    console.log(`    verified ${binary.length} bytes, including NUL and non-UTF-8 bytes`);
    await sandbox.files.remove("/tmp/binary.dat");
    const directory = await mkdtemp(join(tmpdir(), "devbox-js-"));
    try {
      const source = join(directory, "source.txt");
      const target = join(directory, "target.txt");
      await writeFile(source, "transfer-ok");
      await step(`files.upload(${source}, /tmp/transfer.txt) | content='transfer-ok'`, () =>
        sandbox.files.upload(source, "/tmp/transfer.txt"),
      );
      await step(`files.download(/tmp/transfer.txt, ${target})`, () =>
        sandbox.files.download("/tmp/transfer.txt", target),
      );
      equal(
        await step("Read downloaded local file", () => readFile(target, "utf8")),
        "transfer-ok",
      );
      await step("files.remove(/tmp/transfer.txt)", () =>
        sandbox.files.remove("/tmp/transfer.txt"),
      );
    } finally {
      await rm(directory, { recursive: true, force: true });
    }
  });

  await validator.verify("filesystem.watch", async () => {
    const root = `/tmp/devbox-watch-${Date.now()}`;
    await step(`files.makeDir(${root})`, () => sandbox.files.makeDir(root));
    const events = sandbox.files.watch(root, { includeEntry: false });
    let observed = false;
    const guard = setTimeout(() => {
      void sandbox.close();
    }, 10_000);
    let process;
    try {
      process = await step("commands.run(..., background=true) | create a file after 1s", () =>
        sandbox.commands.run(`sleep 1; printf watch-ok > ${root}/watch-proof.txt`, {
          background: true,
        }),
      );
      console.log("  DO files.watch() | observe a real file change, maximum 10s");
      for await (const event of events) {
        console.log(
          `    EVENT name=${JSON.stringify(event.name)} type=${JSON.stringify(event.type)}`,
        );
        if (event.name === "watch-proof.txt") {
          observed = true;
          break;
        }
      }
      equal(observed, true);
      equal(await sandbox.files.read(`${root}/watch-proof.txt`), "watch-ok");
      const result = await step("process.wait() | file writer completed", () => process.wait());
      equal(result.exitCode, 0);
    } finally {
      clearTimeout(guard);
      await events.return();
      await process?.disconnect();
      await sandbox.files.remove(root);
    }
    console.log("    CHECK file change observed; watcher closed");
  });

  section("Interactive terminal", "Start Bash, resize the terminal, send input and exit");
  await validator.verify("pty", async () => {
    const session = await step("pty.start(rows=24, cols=80)", () => sandbox.pty.start());
    await step(`pty.resize(pid=${session.pid}, rows=30, cols=100)`, () =>
      sandbox.pty.resize(session.pid, { rows: 30, cols: 100 }),
    );
    const command = "printf '\\n__PTY_%s__\\n' \"$((19+23))\"; stty size; exit 7\n";
    await step(
      `session.sendStdin(${JSON.stringify(command)}) | compute 42, inspect size, exit with code 7`,
      () => session.sendStdin(command),
    );
    const result = await step("session.wait() | read terminal output until Bash exits", () =>
      session.wait({ check: false }),
    );
    equal(result.exitCode, 7);
    const lines = result.stdout.replaceAll("\r", "").split("\n");
    equal(lines.includes("__PTY_42__"), true);
    equal(lines.includes("30 100"), true);
    let rejected = false;
    try {
      await session.sendStdin("must-not-run\n");
    } catch (error) {
      if (!error.message.includes("already completed")) throw error;
      rejected = true;
    }
    equal(rejected, true);
    console.log("    completed session rejects further input");
  });

  section("Git", "Initialize a repository, configure an author, stage and commit a file");
  await validator.verify("git", async () => {
    const repository = "/tmp/devbox-sdk-git";
    const command = `mkdir -p ${repository} && git -C ${repository} init -b main`;
    await step(`commands.run(${JSON.stringify(command)})`, () => sandbox.commands.run(command));
    await step("git.setConfig(user.name='DevBox SDK')", () =>
      sandbox.git.setConfig(repository, "user.name", "DevBox SDK"),
    );
    await step("git.setConfig(user.email='sdk@devbox.local')", () =>
      sandbox.git.setConfig(repository, "user.email", "sdk@devbox.local"),
    );
    await step("files.write(README.md, 'test\\n')", () =>
      sandbox.files.write(`${repository}/README.md`, "test\n"),
    );
    await step("git.add() | stage changes", () => sandbox.git.add(repository));
    await step("git.commit('test commit')", () => sandbox.git.commit(repository, "test commit"));
    const committed = await step(
      "git show HEAD:README.md | verify committed content, not just exit code",
      () => sandbox.commands.run(`git -C ${repository} show HEAD:README.md`),
    );
    equal(committed.stdout, "test\n");
    equal(committed.stderr, "");
    equal(
      (
        await step("git.status() | expect a clean working tree", () =>
          sandbox.git.status(repository),
        )
      ).stdout.trim(),
      "## main",
    );
    await step(`files.remove(${repository})`, () => sandbox.files.remove(repository));
  });
}

function section(name, description) {
  console.log(`\n=== ${name} ===\n${description}`);
}

async function step(description, operation) {
  console.log(`  DO ${description}`);
  const started = performance.now();
  let result;
  try {
    result = await operation();
  } catch (error) {
    if (error instanceof CommandExitError) commandOutput(error.result);
    throw error;
  }
  console.log(`    DONE | ${seconds(started)}s`);
  if (typeof result?.exitCode === "number") commandOutput(result);
  else if (typeof result?.pid === "number") console.log(`    pid=${result.pid}`);
  else if (typeof result?.state === "string")
    console.log(
      `    state=${result.state} | template=${result.templateId} | end_at=${result.endAt?.toISOString()}`,
    );
  else if (["string", "boolean", "number"].includes(typeof result))
    console.log(`    result=${JSON.stringify(result)}`);
  else if (result instanceof Uint8Array)
    console.log(`    bytes=${result.length} | content checked separately`);
  else if (Array.isArray(result)) {
    console.log(`    items=${result.length}`);
  }
  return result;
}

function commandOutput(result) {
  console.log(`    pid=${result.pid} | exit_code=${result.exitCode}`);
  for (const name of ["stdout", "stderr"]) {
    const value = terminalText(result[name]);
    console.log(`    ${name}:`);
    console.log(
      `      ${value ? value.slice(0, 4000).trimEnd().replaceAll("\n", "\n      ") : "(empty)"}`,
    );
    if (value.length > 4000) console.log(`      ... truncated (${value.length} characters total)`);
  }
}

function terminalText(value) {
  return Array.from(stripVTControlCharacters(value))
    .filter(
      (character) =>
        character === "\n" ||
        character === "\t" ||
        (character.codePointAt(0) >= 32 && character !== "\x7f"),
    )
    .join("");
}

function equal(actual, expected) {
  if (actual !== expected)
    throw new Error(`expected ${JSON.stringify(expected)}, received ${JSON.stringify(actual)}`);
}

function versionKey(version) {
  const match = String(version)
    .trim()
    .match(/^v?(\d+)\.(\d+)\.(\d+)$/);
  if (!match) throw new Error(`invalid EnvD semantic version: ${JSON.stringify(version)}`);
  return match.slice(1).map(Number);
}

function seconds(started) {
  return ((performance.now() - started) / 1000).toFixed(3);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) await main();
