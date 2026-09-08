import { type DevBoxOptions, resolveConfig } from "./config.js";
import type { Transport } from "./internal/transport.js";
import { Nodes } from "./nodes.js";
import { contextFrom, controlTransport, Sandboxes, Snapshots } from "./sandbox.js";
import { Templates } from "./templates.js";

export class DevBox {
  readonly sandboxes: Sandboxes;
  readonly snapshots: Snapshots;
  readonly templates: Templates;
  readonly nodes: Nodes;
  readonly #transport: Transport;

  constructor(options: DevBoxOptions = {}) {
    const config = resolveConfig(options);
    this.#transport = controlTransport(config);
    this.sandboxes = new Sandboxes(this.#transport, contextFrom(config));
    this.snapshots = new Snapshots(this.#transport);
    this.templates = new Templates(this.#transport);
    this.nodes = new Nodes(this.#transport);
  }

  close(): Promise<void> {
    return this.#transport.close();
  }
}
