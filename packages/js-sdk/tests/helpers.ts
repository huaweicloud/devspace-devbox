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
  sandboxID: "sbx-1",
  templateID: "default",
  clientID: "client-1",
  envdVersion: "1.0.0",
  domain: "runtime.example.test",
  sandboxProxyDomain: "devbox.example.test",
  trafficAccessToken: "traffic-token",
  tunnelId: "aaaadysa",
  connectToken: "connect-token",
  tokenLifetime: 86_400,
  tokenExpiration: 1_789_029_315,
  tunnelLifetime: 86_400,
  tunnelExpiration: 1_788_946_515,
};
