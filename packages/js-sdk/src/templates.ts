import { ProtocolError } from "./errors.js";
import type { Transport } from "./internal/transport.js";
import { identifier, objectValue, optionalString, stringArray } from "./internal/wire.js";
import {
  parseTemplateBuild,
  parseTemplateInfo,
  type TemplateAliasInfo,
  type TemplateBuildInfo,
  type TemplateDetail,
  type TemplateFileInfo,
  type TemplateInfo,
  type TemplateInput,
} from "./models.js";

export class Templates {
  readonly #transport: Transport;

  constructor(transport: Transport) {
    this.#transport = transport;
  }

  async create(template: TemplateInput): Promise<TemplateInfo> {
    return parseTemplateInfo(
      objectValue(
        await this.#transport.request("POST", "/templates", { json: templateWire(template) }),
      ),
    );
  }

  async list(): Promise<TemplateInfo[]> {
    const payload = objectValue(await this.#transport.request("GET", "/templates"));
    return objectsOrEmpty(payload.templates).map(parseTemplateInfo);
  }

  async get(templateId: string): Promise<TemplateDetail> {
    const payload = objectValue(
      await this.#transport.request("GET", `/templates/${identifier(templateId)}`),
    );
    return {
      template: parseTemplateInfo(objectValue(payload.template)),
      builds: objectsOrEmpty(payload.builds).map(parseTemplateBuild),
    };
  }

  async getByAlias(alias: string): Promise<TemplateAliasInfo> {
    const value = objectValue(
      await this.#transport.request("GET", `/templates/aliases/${identifier(alias)}`),
    );
    return {
      alias: String(value.alias),
      templateId: String(value.template_id),
      namespace: String(value.namespace),
    };
  }

  async update(templateId: string, values: { name?: string; public?: boolean }): Promise<void> {
    if (values.name === undefined && values.public === undefined)
      throw new TypeError("name or public is required");
    await this.#transport.request("PATCH", `/templates/${identifier(templateId)}`, {
      json: values,
    });
  }

  async delete(templateId: string): Promise<void> {
    await this.#transport.request("DELETE", `/templates/${identifier(templateId)}`);
  }

  async setTags(target: string, tags: string[]): Promise<void> {
    await this.#transport.request("POST", "/templates/tags", { json: { target, tags } });
  }

  async deleteTags(target: string, tags: string[]): Promise<void> {
    await this.#transport.request("DELETE", "/templates/tags", { json: { target, tags } });
  }

  async listTags(templateId: string): Promise<string[]> {
    const payload = objectValue(
      await this.#transport.request("GET", `/templates/${identifier(templateId)}/tags`),
    );
    return stringArray(payload.tags);
  }

  async startBuild(
    templateId: string,
    buildId: string,
    compatible = true,
  ): Promise<TemplateBuildInfo> {
    const prefix = compatible ? "/v2" : "";
    return parseTemplateBuild(
      objectValue(
        await this.#transport.request(
          "POST",
          `${prefix}/templates/${identifier(templateId)}/builds/${identifier(buildId)}`,
        ),
      ),
    );
  }

  async getBuildStatus(templateId: string, buildId: string): Promise<TemplateBuildInfo> {
    return parseTemplateBuild(
      objectValue(
        await this.#transport.request(
          "GET",
          `/templates/${identifier(templateId)}/builds/${identifier(buildId)}/status`,
        ),
      ),
    );
  }

  async getBuildLogs(templateId: string, buildId: string): Promise<string[]> {
    const value = objectValue(
      await this.#transport.request(
        "GET",
        `/templates/${identifier(templateId)}/builds/${identifier(buildId)}/logs`,
      ),
    );
    return stringArray(value.entries);
  }

  async getFile(templateId: string, contentHash: string): Promise<TemplateFileInfo> {
    const value = objectValue(
      await this.#transport.request(
        "GET",
        `/templates/${identifier(templateId)}/files/${identifier(contentHash)}`,
      ),
    );
    return { exists: Boolean(value.exists ?? false), uploadUrl: optionalString(value.upload_url) };
  }
}

function templateWire(template: TemplateInput): Record<string, unknown> {
  const body: Record<string, unknown> = {
    alias: template.alias,
    name: template.name,
    public: template.public ?? false,
  };
  if (template.vcpu !== undefined) body.vcpu = template.vcpu;
  if (template.ramMb !== undefined) body.ram_mb = template.ramMb;
  if (template.totalDiskMb !== undefined) body.total_disk_mb = template.totalDiskMb;
  if (template.startCommand !== undefined) body.start_command = template.startCommand;
  return body;
}

function objectsOrEmpty(value: unknown): Record<string, unknown>[] {
  if (value === undefined || value === null) return [];
  if (!Array.isArray(value)) throw new ProtocolError("template response is invalid");
  return value.map((item) => objectValue(item));
}
