/**
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2027. All rights reserved.
 */

import { type DevBoxOptions, resolveConfig } from './config.js';
import type { Transport } from './internal/transport.js';
import { contextFrom, controlTransport, Sandboxes } from './sandbox.js';
import { Templates } from './templates.js';

/**
 * Entry point of the SDK. Owns the control-plane transport and exposes the
 * sandbox and template namespaces.
 *
 * ```ts
 * const devbox = new DevBox({ apiKey: process.env.DEVBOX_API_KEY });
 * const sandbox = await devbox.sandboxes.create();
 * await devbox.close();
 * ```
 */
export class DevBox {
  readonly sandboxes: Sandboxes;
  readonly templates: Templates;
  readonly #transport: Transport;

  /**
   * Builds a client. Credentials are resolved from `options.apiKey` or the
   * `DEVBOX_API_KEY` / `E2B_API_KEY` environment variables.
   */
  constructor(options: DevBoxOptions = {}) {
    const config = resolveConfig(options);
    this.#transport = controlTransport(config);
    this.sandboxes = new Sandboxes(this.#transport, contextFrom(config));
    this.templates = new Templates(this.#transport);
  }

  /** Closes the shared control-plane transport and idle connections. */
  close(): Promise<void> {
    return this.#transport.close();
  }
}
