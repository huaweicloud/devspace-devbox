import { DevBox } from "../dist/index.js";

const client = new DevBox();
try {
  const page = await client.sandboxes.list({ limit: 100 });
  for (const sandbox of page.items) {
    console.log(`${sandbox.sandboxId}\t${sandbox.state}\t${sandbox.templateId}`);
  }
  console.log(`sandboxes=${page.items.length} totalRunning=${page.total ?? "unknown"}`);
} finally {
  await client.close();
}
