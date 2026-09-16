import dns from "node:dns";
import { syncBuiltinESMExports } from "node:module";
import { isIP } from "node:net";

const example = process.argv[2] ?? "validate-full";
if (!["basic", "validate-full"].includes(example)) {
  throw new Error("Expected example: basic or validate-full");
}

const original = dns.lookup;
const gatewayIp = process.env.DEVBOX_GATEWAY_IP?.trim();
if (gatewayIp) {
  if (!isIP(gatewayIp)) throw new Error("DEVBOX_GATEWAY_IP must be an IP address");
  const template = new URL(process.env.DEVBOX_GATEWAY_URL).hostname.replaceAll("{port}", "49983");
  const [prefix, suffix] = template.split("{tunnel_id}");
  dns.lookup = function lookup(hostname, ...args) {
    const name = hostname.toLowerCase();
    const matches =
      suffix === undefined
        ? name === prefix
        : name.startsWith(prefix) &&
          name.endsWith(suffix) &&
          name.length > prefix.length + suffix.length;
    return original.call(this, matches ? gatewayIp : hostname, ...args);
  };
  // Change DNS only; preserve the URL, HTTP Host, TLS SNI and certificate verification.
  syncBuiltinESMExports();
  console.log(`Local DNS: ${template} -> ${gatewayIp}`);
}

try {
  await import(`./${example}.mjs`);
} finally {
  dns.lookup = original;
  syncBuiltinESMExports();
}
