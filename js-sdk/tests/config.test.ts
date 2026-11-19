import { describe, expect, it } from "vitest";
import { resolveConfig } from "../src/config.js";
import { ConfigurationError } from "../src/errors.js";

describe("resolveConfig", () => {
  it("allows slow Manager operations without extending Gateway requests", () => {
    const config = resolveConfig({ apiKey: "key" });
    expect(config.controlTimeoutMs).toBe(150_000);
    expect(config.requestTimeoutMs).toBe(30_000);
  });

  it("honors an explicit timeout for both services", () => {
    const config = resolveConfig({ apiKey: "key", requestTimeoutMs: 12_000 });
    expect(config.controlTimeoutMs).toBe(12_000);
    expect(config.requestTimeoutMs).toBe(12_000);
  });
  it("uses explicit values", () => {
    const config = resolveConfig({ apiKey: " key ", apiUrl: "https://manager.example.test/" });
    expect(config.apiKey).toBe("key");
    expect(config.apiUrl).toBe("https://manager.example.test");
  });

  it("rejects missing credentials and insecure remote URLs", () => {
    expect(() => resolveConfig({ apiKey: "", apiUrl: "https://manager.example.test" })).toThrow(
      ConfigurationError,
    );
    expect(() => resolveConfig({ apiKey: "key", apiUrl: "http://manager.example.test" })).toThrow(
      ConfigurationError,
    );
    expect(resolveConfig({ apiKey: "key", apiUrl: "http://localhost:8080" }).apiUrl).toBe(
      "http://localhost:8080",
    );
  });
});
