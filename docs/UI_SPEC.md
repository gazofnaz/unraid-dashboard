# ArrayDeck UI specification

## 1. Visual direction

The interface should feel like a modern system console rather than a bookmark page:

- Dark neutral foundation
- Restrained green for healthy/running state
- Blue for selection and navigation
- Amber for degraded or inferred states
- Dense enough for many containers, but not table-like
- Minimal decorative imagery; container icons provide recognition

The mockups use a working palette:

```text
Background       #0B0F14
Panel            #121821
Raised panel     #171F2A
Border           #263140
Primary text     #F5F7FB
Muted text       #8F9BAD
Healthy          #65D49A
Selection        #78A8FF
Warning          #F1BD6C
Danger           #F37F86
```

## 2. Information architecture

### Applications

Default landing page. Grouped, link-focused view of every container.

### Links

A linkstack: every container that resolved to a URL as one ordered column of
icon-and-name rows, and nothing else. The order is explicit and user-owned;
newly deployed applications append to the end rather than waiting to be added.
An edit mode exposes reordering (drag handle, arrows), a page-wide domain/IP
choice with a per-link override, and a hide toggle. The arrangement is stored
server-side, so it is the same on every browser and phone.

Links pinned by a reverse-proxy label or their own container LAN address ignore
the domain/IP choice; the edit row says so rather than offering a control that
would do nothing.

### Containers

Denser inventory table with image, state, health, ports, network and endpoint confidence.

### System

Reserved for future Unraid array, disk, VM and host metrics.

### Discovery rules

Shows source health, resolver behavior, low-confidence candidates and optional overrides.

### Settings

Connections, preferred network interface, link mode defaults, display and security status.

## 3. Desktop applications screen

Reference: `mockups/01-dashboard-desktop.png`

### Header

- Page title and one-line explanation
- Search field
- Refresh/reconcile button

### Global address toolbar

A segmented control with:

- `tower.local`
- `LAN IP`
- `Smart`

Also show the currently detected native address and source health.

### Summary cards

- Total containers
- Containers with web interfaces
- Discovery health
- Server identity/version/uptime when available

### Application groups

Groups are generated automatically. Each group header states where the grouping came from when useful.

### Application card

Required fields:

- Icon or generated initials
- Display name
- Image/repository
- State and health
- Resolved URL or no-web-UI message
- Small metadata tags
- Open action
- Optional confidence indicator

Card click opens the application only when an endpoint exists. A separate details action prevents accidental navigation when inspecting metadata.

## 4. Discovery inspector

Reference: `mockups/02-discovery-inspector.png`

This screen is a major differentiator and should be implemented in the MVP.

Left column:

- Container identity
- Network mode
- Container and host ports
- Matched template
- Category
- All input signals and whether they were used

Right column:

- Ordered resolution steps
- Token substitution
- Final normalized endpoint
- Confidence and source
- Preview of Hostname, LAN IP and Smart rendering
- Override controls

## 5. Mobile behavior

Reference: `mockups/03-mobile-dashboard.png`

- Sidebar becomes a bottom navigation bar.
- Search and link mode remain at the top.
- Summary metrics use a two-column grid.
- Application cards become a single column.
- Group source explanations are shortened.
- URLs remain selectable/copyable and truncate visually.
- Minimum tap target: 44px.

## 6. States

### Loading

Use skeleton cards after the shell renders. Do not block the entire page on icon downloads or probes.

### Source unavailable

Show a non-modal banner such as:

```text
Docker connected. Unraid API unavailable; hostname, LAN address and system metrics may be incomplete.
```

### No containers

Differentiate between:

- Docker returned zero containers
- Docker source is unavailable
- Filters exclude all containers

### No web UI

Keep the card visible and replace the URL row with:

```text
No web interface detected
```

Details should still show ports and network information.

### Low confidence

Use an amber line or badge and route the user to the discovery inspector. Never make the card look unhealthy merely because URL confidence is low.

### Stopped

Display the last-known endpoint but clearly mark the container stopped. Opening may still be allowed because a reverse proxy or dependent service could remain reachable; this can be a setting.

## 7. Interaction details

- `Enter` in search focuses the first result; `Cmd/Ctrl+K` focuses search.
- Copy URL action available in the card menu.
- Link mode updates instantly in the browser.
- Group collapse state persists locally.
- Hover is supplemental; all actions work by keyboard and touch.
- Tooltips explain status and confidence but never contain essential-only information.

## 8. Icon policy

Order:

1. User override
2. `net.unraid.docker.icon`
3. dockerMan `<Icon>`
4. OCI/project icon from curated catalog
5. Generated initials

Proxy external icons through the backend and cache them in SQLite or the appdata filesystem. This avoids mixed-content problems and broken browser CORS behavior.
