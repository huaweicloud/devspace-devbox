import type { Transport } from "./internal/transport.js";
import { objectItems } from "./internal/wire.js";
import { parseTemplateInfo, type TemplateInfo } from "./models.js";

export class Templates {
  constructor(private readonly transport: Transport) {}

  async list(): Promise<TemplateInfo[]> {
    const value = await this.transport.request("GET", "/templates");
    return objectItems(value).map(parseTemplateInfo);
  }
}
