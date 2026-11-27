/**
 * Copyright (c) Huawei Technologies Co., Ltd. 2026-2027. All rights reserved.
 */

import type { Transport } from './internal/transport.js';
import { objectItems } from './internal/wire.js';
import { parseTemplateInfo, type TemplateInfo } from './models.js';

/** Template operations against the Manager API. Accessed via {@link DevBox.templates}. */
export class Templates {
  constructor(private readonly transport: Transport) {}

  /** Lists the latest usable build of each public template. */
  async list(): Promise<TemplateInfo[]> {
    const value = await this.transport.request('GET', '/templates');
    return objectItems(value).map(parseTemplateInfo);
  }
}
