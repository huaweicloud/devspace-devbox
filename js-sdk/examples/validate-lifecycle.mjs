// Run after deploying a Manager with pause/resume support. Creates one sandbox.
import { randomUUID } from "node:crypto";
import { setTimeout as sleep } from "node:timers/promises";
import { DevBox, SandboxState } from "../dist/index.js";

const client = new DevBox({ requestTimeoutMs: 150_000 });
let sandbox;
try {
  const marker = randomUUID();
  const path = "/tmp/devbox-lifecycle-proof";
  sandbox = await client.sandboxes.create(process.env.DEVBOX_TEST_TEMPLATE ?? "default", {
    timeout: 300,
    envs: { LIFECYCLE_MARKER: marker },
  });
  console.log(`PASS create | sandbox_id=${sandbox.sandboxId}`);
  const check = (condition, message) => {
    if (!condition) throw new Error(message);
  };
  check(
    (await sandbox.commands.run('printf "%s" "$LIFECYCLE_MARKER"')).stdout === marker,
    "environment was not initialized",
  );
  await sandbox.files.write(path, marker);
  const handle = await sandbox.commands.run("sleep 240", { background: true });
  const pid = handle.pid;
  await handle.disconnect();
  console.log(`PASS prepare | env and file marker=${marker}, background pid=${pid}`);

  await sandbox.setTimeout(600);
  const extended = (await sandbox.getInfo()).endAt;
  check(extended, "Manager did not return a deadline");
  await sandbox.refresh(120);
  check(
    (await sandbox.getInfo()).endAt?.getTime() === extended.getTime(),
    "refresh shortened the deadline",
  );
  await sandbox.setTimeout(300);
  const shortened = (await sandbox.getInfo()).endAt;
  check(shortened && shortened < extended, "setTimeout did not shorten");
  console.log(
    `PASS deadlines | extended=${extended.toISOString()}, shortened=${shortened.toISOString()}`,
  );

  const started = performance.now();
  await sandbox.pause();
  check((await sandbox.getInfo()).state === SandboxState.Paused, "sandbox did not pause");
  console.log(`PASS pause | ${((performance.now() - started) / 1000).toFixed(3)}s`);
  await sandbox.resume({ timeout: 300 });
  check((await sandbox.files.read(path)) === marker, "file was not restored");
  await sandbox.commands.run(`kill -0 ${pid}`);
  console.log(`PASS resume | file retained, original pid=${pid} still exists`);

  await sandbox.pause();
  await sandbox.close();
  sandbox = await client.sandboxes.connect(sandbox.sandboxId, { timeout: 300 });
  check((await sandbox.files.read(path)) === marker, "connect did not restore file");
  console.log("PASS connect | paused sandbox resumed through Manager connect");

  await sandbox.setTimeout(5);
  const deadline = performance.now() + 75_000;
  while ((await sandbox.isRunning()) && performance.now() < deadline) await sleep(2000);
  check(!(await sandbox.isRunning()), "expired sandbox was not cleaned up within 75s");
  console.log("PASS expiry | sandbox no longer running");
} finally {
  try {
    if (sandbox) {
      await sandbox.kill();
      console.log("CLEANUP | test sandbox deleted");
    }
  } finally {
    await sandbox?.close();
    await client.close();
  }
}
console.log("Lifecycle validation passed");
