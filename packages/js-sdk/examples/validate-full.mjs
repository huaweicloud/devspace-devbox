import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { DevBox } from "../dist/index.js";

class Validator {
  failures = [];
  tests = 0;

  async verify(name, operation) {
    this.tests += 1;
    const started = performance.now();
    try {
      const result = await operation();
      console.log(`PASS ${name} | ${seconds(started)}s`);
      return result;
    } catch (error) {
      this.failures.push(name);
      console.error(`FAIL ${name} | ${seconds(started)}s | ${error.name}: ${error.message}`);
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
  sandbox = await validator.verify("sandbox.create", () =>
    client.sandboxes.create(template, {
      timeout: 300,
      metadata: { sdk_validation: "javascript" },
      network: { allowInternetAccess: true },
    }),
  );
  if (!sandbox) {
    console.error("STOP sandbox creation failed; runtime validation cannot continue");
  } else {
    await validateSandbox(client, sandbox, validator);
  }
} finally {
  if (sandbox) {
    await validator.verify("sandbox.delete", () => sandbox.kill());
    await sandbox.close();
  }
  await client.close();
}

console.log(
  `SUMMARY tests=${validator.tests} passed=${validator.tests - validator.failures.length} failed=${validator.failures.length}`,
);
if (validator.failures.length) {
  console.error(`FAILED ${validator.failures.join(", ")}`);
  process.exitCode = 1;
}

async function validateSandbox(client, sandbox, validator) {
  console.log(`sandboxId=${sandbox.sandboxId}`);

  await validator.verify("manager.get", async () =>
    equal((await sandbox.getInfo()).sandboxId, sandbox.sandboxId),
  );
  await validator.verify("manager.isRunning", async () => equal(await sandbox.isRunning(), true));
  await validator.verify("manager.list", async () => {
    const page = await client.sandboxes.list({ limit: 100 });
    equal(
      page.items.some((item) => item.sandboxId === sandbox.sandboxId),
      true,
    );
  });
  await validator.verify("manager.setTimeout", () => sandbox.setTimeout(300));
  await validator.verify("manager.refresh", () => sandbox.refresh(300));
  await validator.verify("manager.metrics", () => sandbox.getMetrics());
  await validator.verify("manager.logs", () => sandbox.getLogs({ limit: 20 }));
  await validator.verify("manager.updateNetwork", () =>
    sandbox.updateNetwork({ allowInternetAccess: true }),
  );

  const runtime = await validator.verify("runtime.ready", async () => {
    const result = await sandbox.commands.run("printf runtime-ready");
    equal(result.stdout, "runtime-ready");
    return true;
  });
  if (!runtime) {
    console.log("SKIP commands, filesystem, PTY and Git: runtime gateway is unavailable");
  } else {
    await validateRuntime(sandbox, validator);
  }
}

async function validateRuntime(sandbox, validator) {
  await validator.verify("commands.foreground", async () => {
    const result = await sandbox.commands.run('printf "$DEVBOX_TEST"; printf error-ok >&2', {
      envs: { DEVBOX_TEST: "command-ok" },
      cwd: "/tmp",
    });
    equal(result.stdout, "command-ok");
    equal(result.stderr, "error-ok");
  });

  await validator.verify("commands.background", async () => {
    const process = await sandbox.commands.run("sleep 1; printf background-ok", {
      background: true,
      timeoutMs: 10_000,
    });
    equal((await process.wait()).stdout, "background-ok");
  });

  await validator.verify("commands.stdin", async () => {
    const process = await sandbox.commands.run("cat", {
      background: true,
      stdin: true,
      timeoutMs: 10_000,
    });
    await process.sendStdin("stdin-ok\n");
    await process.closeStdin();
    equal((await process.wait()).stdout, "stdin-ok\n");
  });

  await validator.verify("filesystem", async () => {
    await sandbox.files.makeDir("/tmp/devbox-sdk-test");
    await sandbox.files.write("/tmp/devbox-sdk-test/input.txt", "file-ok");
    equal(await sandbox.files.read("/tmp/devbox-sdk-test/input.txt"), "file-ok");
    equal((await sandbox.files.stat("/tmp/devbox-sdk-test/input.txt")).size, 7);
    equal(
      (await sandbox.files.list("/tmp/devbox-sdk-test")).some((item) => item.name === "input.txt"),
      true,
    );
    await sandbox.files.move("/tmp/devbox-sdk-test/input.txt", "/tmp/devbox-sdk-test/output.txt");
    await sandbox.files.remove("/tmp/devbox-sdk-test");
  });

  await validator.verify("filesystem.uploadDownload", async () => {
    const directory = await mkdtemp(join(tmpdir(), "devbox-js-"));
    try {
      const source = join(directory, "source.txt");
      const target = join(directory, "target.txt");
      await writeFile(source, "transfer-ok");
      await sandbox.files.upload(source, "/tmp/transfer.txt");
      await sandbox.files.download("/tmp/transfer.txt", target);
      equal(await readFile(target, "utf8"), "transfer-ok");
      await sandbox.files.remove("/tmp/transfer.txt");
    } finally {
      await rm(directory, { recursive: true, force: true });
    }
  });

  await validator.verify("pty", async () => {
    const session = await sandbox.pty.start();
    await session.sendStdin("printf pty-ok\nexit\n");
    const result = await session.wait({ check: false });
    equal(result.stdout.includes("pty-ok"), true);
  });

  await validator.verify("git", async () => {
    const repository = "/tmp/devbox-sdk-git";
    await sandbox.commands.run(
      `rm -rf ${repository}; mkdir -p ${repository}; git -C ${repository} init`,
    );
    await sandbox.git.setConfig(repository, "user.name", "DevBox SDK");
    await sandbox.git.setConfig(repository, "user.email", "sdk@devbox.local");
    await sandbox.files.write(`${repository}/README.md`, "test\n");
    await sandbox.git.add(repository);
    await sandbox.git.commit(repository, "test commit");
    equal((await sandbox.git.status(repository)).stdout.includes("README.md"), false);
    await sandbox.files.remove(repository);
  });
}

function equal(actual, expected) {
  if (actual !== expected)
    throw new Error(`expected ${JSON.stringify(expected)}, received ${JSON.stringify(actual)}`);
}

function seconds(started) {
  return ((performance.now() - started) / 1000).toFixed(3);
}
