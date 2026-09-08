import { Buffer } from "node:buffer";
import { MockAgent } from "undici";

export function mockAgent(): MockAgent {
  const agent = new MockAgent();
  agent.disableNetConnect();
  return agent;
}

export function connectFrame(value: unknown, trailer = false): Buffer {
  const data = Buffer.from(JSON.stringify(value));
  const frame = Buffer.alloc(data.length + 5);
  frame[0] = trailer ? 2 : 0;
  frame.writeUInt32BE(data.length, 1);
  data.copy(frame, 5);
  return frame;
}

export function connectBody(...values: Array<{ value: unknown; trailer?: boolean }>): Buffer {
  return Buffer.concat(values.map(({ value, trailer }) => connectFrame(value, trailer)));
}

export const sandboxResponse = {
  sandbox: {
    sandboxId: "sbx-1",
    templateId: "default",
    state: "running",
    createdAt: "2026-09-07T00:00:00Z",
  },
  connection: {
    gatewayUrl: "https://runtime.example.test",
    accessToken: "runtime-token",
    expiresAt: "2026-09-07T01:00:00Z",
  },
};
