import type { Transport } from "./internal/transport.js";
import { objectValue, stringRecord } from "./internal/wire.js";
import { type EnvdHealth, type EnvdMetrics, parseEnvdMetrics } from "./models.js";

type TransportProvider = () => Promise<Transport>;

/**
 * Data-plane access to the sandbox-local envd service: metrics, environment
 * variables and health. Accessed via {@link Sandbox.envd}.
 */
export class Envd {
  readonly #transport: TransportProvider;

  constructor(transport: TransportProvider) {
    this.#transport = transport;
  }

  /** Returns current CPU, memory and disk usage metrics of the sandbox. */
  async getMetrics(): Promise<EnvdMetrics> {
    return parseEnvdMetrics(
      objectValue(await (await this.#transport()).request("GET", "/metrics")),
    );
  }

  /** Returns the environment variables visible to envd. */
  async getEnvs(): Promise<Record<string, string>> {
    return stringRecord(
      objectValue(
        await (await this.#transport()).request("GET", "/envs"),
        "EnvD returned an invalid envs response",
      ),
    );
  }

  /** Returns the envd health info, including the guest envd version. */
  async health(): Promise<EnvdHealth> {
    const { headers } = await (await this.#transport()).requestWithHeaders("GET", "/health");
    return { envdVersion: headers.get("x-envd-version") ?? "" };
  }
}
