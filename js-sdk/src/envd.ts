import type { Transport } from "./internal/transport.js";
import { objectValue, stringRecord } from "./internal/wire.js";
import { type EnvdHealth, type EnvdMetrics, parseEnvdMetrics } from "./models.js";

type TransportProvider = () => Promise<Transport>;

export class Envd {
  readonly #transport: TransportProvider;

  constructor(transport: TransportProvider) {
    this.#transport = transport;
  }

  async getMetrics(): Promise<EnvdMetrics> {
    return parseEnvdMetrics(
      objectValue(await (await this.#transport()).request("GET", "/metrics")),
    );
  }

  async getEnvs(): Promise<Record<string, string>> {
    return stringRecord(
      objectValue(
        await (await this.#transport()).request("GET", "/envs"),
        "EnvD returned an invalid envs response",
      ),
    );
  }

  async health(): Promise<EnvdHealth> {
    const { headers } = await (await this.#transport()).requestWithHeaders("GET", "/health");
    return { envdVersion: headers.get("x-envd-version") ?? "" };
  }
}
