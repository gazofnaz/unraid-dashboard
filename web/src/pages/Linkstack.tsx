import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import type {
  ContainerView,
  Linkstack,
  LinkstackAddress,
  LinkstackEntry,
} from "../api";
import { AppIcon } from "../components";
import { describePolicy, renderURL } from "../links";
import { useAppState } from "../store";

// The launcher is presentation over endpoints discovery has already resolved:
// an explicit order, and the address form each link renders with. Nothing here
// changes how a URL is discovered — only how it is shown and opened.

interface Row {
  container: ContainerView;
  entry: LinkstackEntry;
}

/** Applies the stored order to the live inventory. */
function arrange(
  containers: ContainerView[],
  entries: LinkstackEntry[],
  showSelf: boolean,
): Row[] {
  const linkable = containers.filter(
    (c) => c.endpoint && !c.hidden && (showSelf || !c.isSelf),
  );
  const byKey = new Map(linkable.map((c) => [c.key, c]));
  const rows: Row[] = [];
  const placed = new Set<string>();
  for (const entry of entries) {
    const container = byKey.get(entry.containerKey);
    if (!container || placed.has(entry.containerKey)) continue;
    placed.add(entry.containerKey);
    rows.push({ container, entry });
  }
  // A container the launcher has never seen — a freshly deployed app — joins
  // the end of the list instead of waiting to be added by hand.
  const fresh = linkable
    .filter((c) => !placed.has(c.key))
    .sort((a, b) =>
      a.displayName.toLowerCase().localeCompare(b.displayName.toLowerCase()),
    );
  for (const container of fresh) {
    rows.push({ container, entry: { containerKey: container.key } });
  }
  return rows;
}

export default function LinkstackPage() {
  const { containers, status, loading, linkstack, saveLinkstack } =
    useAppState();
  const identity = status?.identity ?? {};
  const showSelf = status?.settings.showSelf ?? false;

  const [draft, setDraft] = useState<Linkstack | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [dragging, setDragging] = useState<string | null>(null);

  const editing = draft !== null;
  const stack = draft ?? linkstack;

  const rows = useMemo(
    () => arrange(containers, stack.entries, showSelf),
    [containers, stack.entries, showSelf],
  );
  const shown = rows.filter((r) => !r.entry.hidden);

  // Entries for containers that exist but currently resolve to no endpoint are
  // parked rather than dropped, so a link that flaps out of discovery keeps its
  // place and its address choice.
  const compose = (base: Linkstack, nextRows: Row[]): Linkstack => {
    const listed = new Set(nextRows.map((r) => r.entry.containerKey));
    const known = new Set(containers.map((c) => c.key));
    const parked = base.entries.filter(
      (e) => !listed.has(e.containerKey) && known.has(e.containerKey),
    );
    return { ...base, entries: [...nextRows.map((r) => r.entry), ...parked] };
  };

  // Edits re-derive the arrangement from the previous draft and address rows by
  // container key, so a burst of clicks or drag-over events that React batches
  // into one render still applies every step in order.
  const update = (edit: (rows: Row[]) => Row[]) =>
    setDraft((prev) => {
      const base = prev ?? linkstack;
      return compose(base, edit(arrange(containers, base.entries, showSelf)));
    });

  const reorder = (rows: Row[], from: number, to: number): Row[] => {
    if (from < 0 || to < 0 || to >= rows.length || from === to) return rows;
    const next = [...rows];
    const [moved] = next.splice(from, 1);
    next.splice(to, 0, moved);
    return next;
  };

  const indexOf = (rows: Row[], key: string) =>
    rows.findIndex((r) => r.entry.containerKey === key);

  const nudge = (key: string, delta: number) =>
    update((rows) => {
      const from = indexOf(rows, key);
      return reorder(rows, from, from + delta);
    });

  const dropOnto = (key: string, targetKey: string) =>
    update((rows) => reorder(rows, indexOf(rows, key), indexOf(rows, targetKey)));

  const patchEntry = (key: string, patch: Partial<LinkstackEntry>) =>
    update((rows) =>
      rows.map((r) =>
        r.entry.containerKey === key
          ? { ...r, entry: { ...r.entry, ...patch } }
          : r,
      ),
    );

  const save = async () => {
    setSaving(true);
    setError(null);
    try {
      await saveLinkstack(compose(stack, rows));
      setDraft(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  const urlFor = (row: Row) =>
    renderURL(row.container.endpoint!, row.entry.address ?? stack.address, identity);

  const addressName =
    stack.address === "lan-ip"
      ? identity.lanAddress
        ? `IP address (${identity.lanAddress})`
        : "IP address"
      : identity.hostname
        ? `domain (${identity.hostname})`
        : "domain";

  return (
    <>
      <header className="page-head">
        <div>
          <h1>Links</h1>
          <p className="sub">
            {editing
              ? "Reorder with the handles or arrows, pick the address each link uses, and hide anything that doesn’t belong on the page."
              : `Every application as one ordered list. Links open by ${addressName}.`}
          </p>
        </div>
        <div className="actions">
          {editing ? (
            <>
              <button
                className="btn"
                onClick={() => {
                  setDraft(null);
                  setError(null);
                }}
                disabled={saving}
              >
                Cancel
              </button>
              <button
                className="btn primary"
                onClick={() => void save()}
                disabled={saving}
              >
                {saving ? "Saving…" : "Save order"}
              </button>
            </>
          ) : (
            <button className="btn" onClick={() => setDraft(linkstack)}>
              Edit
            </button>
          )}
        </div>
      </header>

      {error && (
        <div className="banner danger" role="alert">
          <strong>Save failed.</strong> {error}
        </div>
      )}

      {editing && (
        <div className="toolbar linkstack-toolbar">
          <span className="muted">Links open by</span>
          <div className="segment" role="group" aria-label="Link address">
            <button
              aria-pressed={stack.address === "hostname"}
              onClick={() => setDraft({ ...stack, address: "hostname" })}
            >
              {identity.hostname || "Domain"}
            </button>
            <button
              aria-pressed={stack.address === "lan-ip"}
              onClick={() => setDraft({ ...stack, address: "lan-ip" })}
            >
              {identity.lanAddress || "IP address"}
            </button>
          </div>
          <span className="hint">
            Saved for every browser. Individual links can override it.
          </span>
        </div>
      )}

      {loading ? (
        <div className="linkstack" aria-busy="true" aria-label="Loading links">
          {Array.from({ length: 6 }).map((_, i) => (
            <div className="skeleton" key={i} style={{ height: 62 }} />
          ))}
        </div>
      ) : rows.length === 0 ? (
        <div className="empty">
          <h3>No links yet</h3>
          <p>
            The launcher lists containers that resolved to a URL. None have so
            far — <Link to="/rules">discovery rules</Link> shows why.
          </p>
        </div>
      ) : !editing && shown.length === 0 ? (
        <div className="empty">
          <h3>Every link is hidden</h3>
          <p>Choose Edit to bring applications back onto the page.</p>
        </div>
      ) : editing ? (
        <div className="linkstack editing">
          {rows.map((row, i) => {
            const ep = row.container.endpoint!;
            const pinned = ep.addressPolicy !== "unraid-host";
            return (
              <div
                key={row.container.key}
                className={`linkstack-row${row.entry.hidden ? " off" : ""}${
                  dragging === row.container.key ? " dragging" : ""
                }`}
                onDragOver={(e) => {
                  e.preventDefault();
                  if (dragging && dragging !== row.container.key) {
                    dropOnto(dragging, row.container.key);
                  }
                }}
                onDrop={(e) => e.preventDefault()}
              >
                <span
                  className="handle"
                  draggable
                  onDragStart={() => setDragging(row.container.key)}
                  onDragEnd={() => setDragging(null)}
                  title="Drag to reorder"
                  aria-hidden="true"
                >
                  ⠿
                </span>
                <div className="arrows">
                  <button
                    onClick={() => nudge(row.container.key, -1)}
                    disabled={i === 0}
                    aria-label={`Move ${row.container.displayName} up`}
                  >
                    ↑
                  </button>
                  <button
                    onClick={() => nudge(row.container.key, 1)}
                    disabled={i === rows.length - 1}
                    aria-label={`Move ${row.container.displayName} down`}
                  >
                    ↓
                  </button>
                </div>
                <AppIcon icon={row.container.icon} size={32} />
                <div className="row-title">
                  <div className="name">{row.container.displayName}</div>
                  <div className="url mono">{urlFor(row)}</div>
                </div>
                <div className="row-controls">
                  <select
                    aria-label={`Address for ${row.container.displayName}`}
                    value={row.entry.address ?? ""}
                    disabled={pinned}
                    title={
                      pinned
                        ? `This link is pinned to its ${describePolicy(ep.addressPolicy)}, so the domain/IP choice does not apply.`
                        : "Address used by this link"
                    }
                    onChange={(e) =>
                      patchEntry(row.container.key, {
                        address: (e.target.value || undefined) as
                          | LinkstackAddress
                          | undefined,
                      })
                    }
                  >
                    <option value="">Page default</option>
                    <option value="hostname">Domain</option>
                    <option value="lan-ip">IP address</option>
                  </select>
                  <button
                    className="btn small"
                    onClick={() =>
                      patchEntry(row.container.key, { hidden: !row.entry.hidden })
                    }
                  >
                    {row.entry.hidden ? "Show" : "Hide"}
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      ) : (
        <div className="linkstack">
          {shown.map((row) => {
            const url = urlFor(row);
            return (
              <a
                key={row.container.key}
                className="linkstack-link"
                href={url}
                target="_blank"
                rel="noreferrer noopener"
                title={url}
              >
                <AppIcon icon={row.container.icon} size={40} />
                <span className="name">{row.container.displayName}</span>
                <span className="go" aria-hidden="true">
                  ↗
                </span>
              </a>
            );
          })}
        </div>
      )}
    </>
  );
}
