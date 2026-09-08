import type { Transport } from "./internal/transport.js";
import { identifier, objectValue } from "./internal/wire.js";
import { type NodeInfo, NodeStatus, parseNode } from "./models.js";

export class Nodes {
  readonly #transport: Transport;

  constructor(transport: Transport) {
    this.#transport = transport;
  }

  async list(): Promise<NodeInfo[]> {
    const payload = objectValue(await this.#transport.request("GET", "/nodes"));
    return Array.isArray(payload.nodes)
      ? payload.nodes.map((item) => objectValue(item)).map(parseNode)
      : [];
  }

  async get(nodeId: string): Promise<NodeInfo> {
    return parseNode(
      objectValue(await this.#transport.request("GET", `/nodes/${identifier(nodeId)}`)),
    );
  }

  async updateStatus(nodeId: string, status: NodeStatus): Promise<NodeInfo> {
    if (!Object.values(NodeStatus).includes(status)) throw new TypeError("invalid node status");
    return parseNode(
      objectValue(
        await this.#transport.request("POST", `/nodes/${identifier(nodeId)}`, { json: { status } }),
      ),
    );
  }
}
