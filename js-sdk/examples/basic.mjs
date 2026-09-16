import { Sandbox } from "../dist/index.js";

const sandbox = await Sandbox.create(process.env.DEVBOX_TEST_TEMPLATE ?? "default", {
  timeout: 300,
});

try {
  console.log(`sandbox created: ${sandbox.sandboxId}`);

  const result = await sandbox.commands.run("printf 'hello from DevBox'");
  console.log(`command output: ${result.stdout}`);

  await sandbox.files.write("/tmp/message.txt", "hello from the SDK");
  console.log(`file content: ${await sandbox.files.read("/tmp/message.txt")}`);
} finally {
  await sandbox.kill();
  await sandbox.close();
}
