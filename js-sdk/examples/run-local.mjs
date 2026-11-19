import dns from "node:dns";
import { syncBuiltinESMExports } from "node:module";
import { isIP } from "node:net";

const original = dns.lookup;
const gatewayIp = process.env.DEVBOX_GATEWAY_IP?.trim();
if (gatewayIp) {
  if (!isIP(gatewayIp)) throw new Error("DEVBOX_GATEWAY_IP must be an IP address");
  dns.lookup = function lookup(hostname, ...args) {
    const [label, domain] = hostname.toLowerCase().split(".", 2);
    const matches = domain !== undefined && label.endsWith("-49983");
    return original.call(this, matches ? gatewayIp : hostname, ...args);
  };
  // Change DNS only; preserve the URL, HTTP Host, TLS SNI and certificate verification.
  syncBuiltinESMExports();
  console.log(`Local DNS: *-49983.* -> ${gatewayIp}`);
}

try {
  const { main } = await import("./validate-full.mjs");
  await main({ args: process.argv.slice(2) });
} finally {
  dns.lookup = original;
  syncBuiltinESMExports();
}
