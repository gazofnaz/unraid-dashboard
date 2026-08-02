# ArrayDeck discovery engine

## 1. Goal

Derive a useful browser endpoint for each container without requiring a dashboard-specific configuration file.

The resolver must favor precision over aggressive guessing. A missing link is preferable to a confident-looking wrong link.

## 2. Inputs

For each container collect:

- Container name and image reference
- State and health
- Docker labels
- Exposed and published ports
- Network mode and network attachments
- Container IP addresses
- Unraid dockerMan template metadata
- Reverse-proxy labels
- Optional application catalog match
- Probe results
- User override

## 3. Candidate priority

### Priority 100 — user override

A user override always wins, but the current discovered candidates are still retained and shown.

### Priority 90 — explicit application URL labels

Recognize:

- ArrayDeck-native future label: `io.arraydeck.url`
- Unraid label: `net.unraid.docker.webui`
- Dashboard ecosystem labels only when they clearly contain a URL
- Explicit reverse-proxy router host rules

### Priority 85 — dockerMan `<WebUI>`

Use the matching user template’s `<WebUI>` value. This is especially valuable because Unraid templates commonly contain `[IP]` and `[PORT:n]` placeholders.

### Priority 75 — reverse-proxy routing labels

Parse common proxy metadata into explicit hosts and paths. A configured host such as `photos.home.example` should be preserved in Smart mode.

### Priority 60 — known application catalog

A bundled catalog may map a normalized image or application name to:

- Default internal web port
- Default path
- Scheme
- Category
- Icon hint

The catalog is a fallback, not a replacement for live port mappings.

### Priority 40 — published HTTP-port inference

Consider published TCP ports that are plausible HTTP interfaces. Confirm with an HTTP/HTTPS probe.

### Priority 0 — no endpoint

Return no winner and render “No web interface detected.”

## 4. Token resolution

Given:

```text
http://[IP]:[PORT:8080]/
```

Resolve in this order:

1. Parse scheme, host token, port token and path.
2. Map container port `8080/tcp` to the published host port.
3. Choose an address policy from the network mode.
4. Keep the endpoint normalized rather than substituting a final hostname immediately.

Example normalized endpoint:

```json
{
  "scheme": "http",
  "addressPolicy": "unraid-host",
  "containerPort": 8080,
  "hostPort": 8080,
  "path": "/"
}
```

## 5. Network-mode rules

### Bridge or user-defined bridge

When a port is published, use the Unraid host address and published host port.

When no port is published, do not point the user at the internal Docker IP unless an explicit route or reverse proxy is known.

### Host network

Use the Unraid host address. Port mappings are not meaningful in host mode; use the explicit WebUI/catalog port.

### Custom macvlan/ipvlan network with LAN address

If the container has a LAN-routable address and the WebUI metadata points to the container itself, use the container LAN IP. Preserve a fixed host port if present in metadata.

Provide an option to prefer the Unraid host when the user routes custom-network traffic through a proxy.

### Container network namespace sharing

For `network_mode: container:<name>`, inherit the target container’s network identity but retain the current application’s WebUI metadata and path.

## 6. Port mapping logic

- Match protocol as well as port number.
- Prefer bindings reachable from the LAN (`0.0.0.0`, `::`, or the selected host address).
- Treat `127.0.0.1` bindings as local-only and do not offer them to remote browsers unless the UI is itself on the host.
- When multiple host bindings exist, prefer the selected LAN address, then wildcard IPv4, then wildcard IPv6.
- Do not infer HTTP from UDP ports.

## 7. HTTP probing

Probes are confirmation and fallback, not the primary discovery method.

Rules:

- Probe only local addresses derived from trusted container metadata.
- Use short timeouts, for example 750ms connect and 1.5s total.
- Limit concurrency.
- Try `HEAD`, then a small `GET` when required.
- Accept redirect responses as evidence of a web UI.
- Do not follow redirects to public or unrelated networks during discovery.
- Never send credentials.
- Store only response class, redirect target, content type and timing.

Likely web ports can include 80, 443, 3000, 5000, 8000, 8080, 8081, 8096, 8123, 8181, 8443, 8989 and application-catalog ports. This list should influence ranking, not create a link by itself.

## 8. Non-web service exclusions

Avoid auto-linking ports strongly associated with non-HTTP services unless a probe confirms HTTP. Examples include:

- PostgreSQL 5432
- Redis 6379
- MySQL/MariaDB 3306
- SMTP 25/465/587
- DNS 53
- MQTT 1883
- SSH 22

## 9. Confidence model

Suggested scoring:

```text
1.00 user override
0.98 explicit Unraid WebUI label + valid port mapping + successful probe
0.95 explicit WebUI label + valid port mapping
0.92 dockerMan WebUI + valid mapping
0.88 reverse-proxy host rule
0.75 known app catalog + matching published port + probe
0.60 plausible published port + probe
<0.50 do not select automatically
```

Confidence must be accompanied by source text. Do not show a naked percentage without an explanation.

## 10. Automatic category rules

Normalize Unraid category strings such as `MediaApp:Video` or `Downloaders:Other` into top-level groups. Suggested UI groups:

- Media
- Downloads
- Automation
- Home
- Network
- Security
- Development
- Storage
- Monitoring
- Databases
- Infrastructure
- Other

A container may have several tags but one primary visual group. Users can switch to a flat view.

## 11. Reconciliation pseudocode

```text
on reconcile:
  docker_records = docker.list_and_inspect_all()
  unraid_records = unraid.fetch_if_available()
  templates = templates.parse_all()

  for docker_record in docker_records:
    record = normalize(docker_record, unraid_records, templates)
    candidates = resolver.generate(record)
    winner = resolver.rank(candidates, stored_override)
    category = classifier.classify(record)
    publish(record, winner, category)

  remove records whose containers no longer exist
  emit a compact diff to connected browsers
```
