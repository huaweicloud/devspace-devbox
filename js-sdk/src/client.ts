import { type DevBoxOptions, resolveConfig } from "./config.js";
import type { Transport } from "./internal/transport.js";
import { contextFrom, controlTransport, Sandboxes } from "./sandbox.js";
import { Templates } from "./templates.js";

export class DevBox {
  readonly sandboxes: Sandboxes;
  readonly templates: Templates;
  readonly #transport: Transport;

  constructor(options: DevBoxOptions = {}) {
    const config = resolveConfig(options);
    this.#transport = controlTransport(config);
    this.sandboxes = new Sandboxes(this.#transport, contextFrom(config));
    this.templates = new Templates(this.#transport);
  }

  close(): Promise<void> {
    return this.#transport.close();
  }
}
