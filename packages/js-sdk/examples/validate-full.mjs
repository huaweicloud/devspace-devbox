import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { CommandExitError, DevBox, ServiceUnavailableError } from "../dist/index.js";

class Validator {
  failures = [];
  tests = 0;

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
      console.log(`FAIL ${name} | ${seconds(started)}s | ${error.name}: ${error.message}`);
      return undefined;
    }
  }
}

const template = process.env.DEVBOX_TEST_TEMPLATE || "default";
const validator = new Validator();
const client = new DevBox();
let sandbox;

console.log(`DevBox full validation | template=${template}`);
try {
  section("Create sandbox", `Start template=${template}, timeout=300s, with an isolated runtime`);
  sandbox = await validator.verify("sandbox.create", () =>
    step("sandboxes.create()", () =>
      client.sandboxes.create(template, {
        timeout: 300,
        metadata: { sdk_validation: "javascript" },
      }),
    ),
  );
  if (!sandbox) {
    console.log("STOP sandbox creation failed; runtime validation cannot continue");
  } else {
    await validateSandbox(client, sandbox, validator);
  }
} finally {
  if (sandbox) {
    section("Cleanup", "Delete the remote sandbox and close local connections");
    await validator.verify("sandbox.delete", () => step("sandbox.kill()", () => sandbox.kill()));
    await sandbox.close();
  }
  await client.close();
}

console.log(
  `SUMMARY tests=${validator.tests} passed=${validator.tests - validator.failures.length} failed=${validator.failures.length}`,
);
if (validator.failures.length) {
  console.log(`FAILED ${validator.failures.join(", ")}`);
  process.exitCode = 1;
}

async function validateSandbox(client, sandbox, validator) {
  section("Sandbox management", "Inspect, reconnect, extend lifetime and read monitoring data");
  console.log(`sandboxId=${sandbox.sandboxId}`);

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
  await validator.verify("manager.setTimeout", () =>
    step("sandbox.setTimeout(300) | remaining lifetime: 300s", () => sandbox.setTimeout(300)),
  );
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
  await validator.verify("manager.refresh", () =>
    step("sandbox.refresh(300) | extend lifetime by 300s", () => sandbox.refresh(300)),
  );
  await validator.verify("manager.metrics", () =>
    step("sandbox.getMetrics()", () => sandbox.getMetrics()),
  );
  await validator.verify("manager.logs", () =>
    step("sandbox.getLogs(limit=20)", () => sandbox.getLogs({ limit: 20 })),
  );
  const runtime = await validator.verify("runtime.ready", () => waitForRuntime(sandbox));
  if (!runtime) {
    console.log("SKIP commands, filesystem, PTY and Git: runtime gateway is unavailable");
  } else {
    await validateRuntime(sandbox, validator);
  }
}

async function waitForRuntime(sandbox) {
  const deadline = performance.now() + 30_000;
  while (true) {
    try {
      const result = await step(
        "commands.run('printf runtime-ready') | verify remote execution",
        () => sandbox.commands.run("printf runtime-ready"),
      );
      equal(result.stdout, "runtime-ready");
      return true;
    } catch (error) {
      if (!(error instanceof ServiceUnavailableError) || performance.now() >= deadline) throw error;
      console.log("    RETRY runtime is not ready; wait 1s (maximum 30s)");
      await new Promise((resolve) => setTimeout(resolve, 1_000));
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

  section("Interactive terminal", "Start Bash, resize the terminal, send input and exit");
  await validator.verify("pty", async () => {
    const session = await step("pty.start(rows=24, cols=80)", () => sandbox.pty.start());
    await step(`pty.resize(pid=${session.pid}, rows=30, cols=100)`, () =>
      sandbox.pty.resize(session.pid, { rows: 30, cols: 100 }),
    );
    await step("session.sendStdin('printf pty-ok\\nexit\\n')", () =>
      session.sendStdin("printf pty-ok\nexit\n"),
    );
    const result = await step("session.wait() | read terminal output until Bash exits", () =>
      session.wait({ check: false }),
    );
    equal(result.exitCode, 0);
    equal(result.stdout.includes("pty-ok"), true);
  });

  section("Git", "Initialize a repository, configure an author, stage and commit a file");
  await validator.verify("git", async () => {
    const repository = "/tmp/devbox-sdk-git";
    const command = `rm -rf ${repository}; mkdir -p ${repository}; git -C ${repository} init`;
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
    equal(
      (
        await step("git.status() | expect a clean working tree", () =>
          sandbox.git.status(repository),
        )
      ).stdout.includes("README.md"),
      false,
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
  else if (Array.isArray(result)) {
    console.log(`    items=${result.length}`);
    const sample = result.at(-1);
    if (typeof sample?.cpuUsedPercent === "number")
      console.log(
        `    sample_time=${sample.timestampUnix} | cpu_used=${sample.cpuUsedPercent}% | memory_used=${sample.memoryUsedBytes} bytes | disk_used=${sample.diskUsedBytes} bytes`,
      );
  }
  return result;
}

function commandOutput(result) {
  console.log(`    pid=${result.pid} | exit_code=${result.exitCode}`);
  for (const name of ["stdout", "stderr"]) {
    const value = result[name];
    console.log(`    ${name}:`);
    console.log(
      `      ${value ? value.slice(0, 4000).trimEnd().replaceAll("\n", "\n      ") : "(empty)"}`,
    );
    if (value.length > 4000) console.log(`      ... truncated (${value.length} characters total)`);
  }
}

function equal(actual, expected) {
  if (actual !== expected)
    throw new Error(`expected ${JSON.stringify(expected)}, received ${JSON.stringify(actual)}`);
}

function seconds(started) {
  return ((performance.now() - started) / 1000).toFixed(3);
}
