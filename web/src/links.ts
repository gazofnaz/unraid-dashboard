import type { Endpoint, HostIdentity, LinkMode } from "./api";

// Address rendering is separate from endpoint discovery: switching the link
// mode rewrites URLs client-side without any rediscovery.
//
// Policy × mode → host:
//   unraid-host      hostname → server hostname (fallback browser origin)
//   unraid-host      lan-ip   → server LAN address (fallback browser origin)
//   unraid-host      smart    → hostname, then LAN address, then origin
//   container-lan-ip any      → the container's own LAN address
//   explicit-host    any      → the explicit host (reverse proxy / override)

export function hostFor(
  ep: Endpoint,
  mode: LinkMode,
  identity: HostIdentity,
): string {
  const origin = window.location.hostname;
  if (ep.addressPolicy === "explicit-host" || ep.addressPolicy === "container-lan-ip") {
    return ep.explicitHost || origin;
  }
  const hostname = identity.hostname || "";
  const lan = identity.lanAddress || "";
  switch (mode) {
    case "hostname":
      return hostname || origin;
    case "lan-ip":
      return lan || origin;
    case "smart":
      return hostname || lan || origin;
  }
}

export function renderURL(
  ep: Endpoint,
  mode: LinkMode,
  identity: HostIdentity,
): string {
  const host = hostFor(ep, mode, identity);
  const isDefaultPort =
    !ep.hostPort ||
    (ep.scheme === "http" && ep.hostPort === 80) ||
    (ep.scheme === "https" && ep.hostPort === 443);
  const port = isDefaultPort ? "" : `:${ep.hostPort}`;
  return `${ep.scheme}://${host}${port}${ep.path || "/"}`;
}

export function describePolicy(policy: Endpoint["addressPolicy"]): string {
  switch (policy) {
    case "unraid-host":
      return "Unraid host";
    case "container-lan-ip":
      return "container LAN IP";
    case "explicit-host":
      return "explicit host";
  }
}

export function sourceKind(source: string | undefined): string {
  if (!source) return "";
  if (source === "override") return "manual";
  if (source.startsWith("label:")) return "exact";
  if (source.startsWith("template:")) return "template";
  if (source.startsWith("proxy:")) return "proxy";
  if (source.startsWith("catalog:")) return "catalog";
  if (source.startsWith("probe:")) return "inferred";
  return source;
}
