import type { Dispatcher } from "undici";
import { ConfigurationError } from "./errors.js";

export const DEFAULT_API_URL = "https://devbox.developer.myhuaweicloud.com";

export interface DevBoxOptions {
  apiKey?: string;
  apiUrl?: string;
  gatewayUrl?: string;
  requestTimeoutMs?: number;
  headers?: Record<string, string>;
  dispatcher?: Dispatcher;
}

export interface ConnectionConfig {
  apiKey: string;
  apiUrl: string;
  gatewayUrl?: string;
  requestTimeoutMs: number;
  headers: Record<string, string>;
  dispatcher?: Dispatcher;
}

export function resolveConfig(options: DevBoxOptions = {}): ConnectionConfig {
  const apiKey = (
    options.apiKey ??
    process.env.DEVBOX_API_KEY ??
    process.env.E2B_API_KEY ??
    ""
  ).trim();
  if (!apiKey) {
    throw new ConfigurationError(
      "apiKey is required; set DEVBOX_API_KEY, E2B_API_KEY, or pass apiKey",
    );
  }

  const requestTimeoutMs = options.requestTimeoutMs ?? 30_000;
  if (requestTimeoutMs <= 0) {
    throw new ConfigurationError("requestTimeoutMs must be positive");
  }

  const apiUrl = serviceUrl(
    options.apiUrl ?? process.env.DEVBOX_API_URL ?? process.env.E2B_API_URL ?? DEFAULT_API_URL,
    "apiUrl",
  );
  const rawGatewayUrl = options.gatewayUrl ?? process.env.DEVBOX_GATEWAY_URL;

  return {
    apiKey,
    apiUrl,
    gatewayUrl: rawGatewayUrl ? serviceUrl(rawGatewayUrl, "gatewayUrl") : undefined,
    requestTimeoutMs,
    headers: { ...options.headers },
    dispatcher: options.dispatcher,
  };
}

function serviceUrl(value: string, name: string): string {
  const normalized = value.trim().replace(/\/$/, "");
  let parsed: URL;
  try {
    parsed = new URL(normalized);
  } catch {
    throw new ConfigurationError(`${name} is not a valid URL`);
  }

  const local = ["localhost", "127.0.0.1", "[::1]", "::1"].includes(parsed.hostname);
  if (parsed.protocol !== "https:" && !(parsed.protocol === "http:" && local)) {
    throw new ConfigurationError(`${name} must use https (http is allowed only for localhost)`);
  }
  return normalized;
}
