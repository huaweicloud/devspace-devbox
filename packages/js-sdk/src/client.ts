import { type DevBoxOptions, resolveConfig } from "./config.js";
import type { Transport } from "./internal/transport.js";
import { contextFrom, controlTransport, Sandboxes } from "./sandbox.js";

export class DevBox {
  readonly sandboxes: Sandboxes;
  readonly #transport: Transport;

  constructor(options: DevBoxOptions = {}) {
    const config = resolveConfig(options);
    this.#transport = controlTransport(config);
    this.sandboxes = new Sandboxes(this.#transport, contextFrom(config));
  }

  close(): Promise<void> {
    return this.#transport.close();
  }
}
