import { useState } from "react";
import type { ContainerView, IconRef, LinkMode } from "./api";

/** Container icon: proxied image with generated-initials fallback. */
export function AppIcon({ icon, size }: { icon: IconRef; size?: number }) {
  const [broken, setBroken] = useState(false);
  const style = {
    background: `hsl(${icon.hue} 42% 42%)`,
    ...(size ? { width: size, height: size, fontSize: size * 0.36 } : {}),
  };
  return (
    <span className="app-icon" style={style} aria-hidden="true">
      {icon.kind === "proxy" && icon.url && !broken ? (
        <img src={icon.url} alt="" loading="lazy" onError={() => setBroken(true)} />
      ) : (
        icon.initials
      )}
    </span>
  );
}

/** State pill: always a dot AND a text label, never color alone. */
export function StatePill({ c }: { c: ContainerView }) {
  let tone = "neutral";
  let label = c.state;
  if (c.state === "running") {
    tone = "ok";
    label = "Running";
    if (c.health === "unhealthy") {
      tone = "warn";
      label = "Degraded";
    } else if (c.health === "starting") {
      tone = "neutral";
      label = "Starting";
    }
  } else if (c.state === "exited" || c.state === "dead" || c.state === "created") {
    tone = "neutral";
    label = "Stopped";
  } else if (c.state === "restarting") {
    tone = "warn";
    label = "Restarting";
  } else if (c.state === "paused") {
    tone = "neutral";
    label = "Paused";
  }
  return (
    <span className={`pill ${tone}`}>
      <span className="dot" />
      {label}
    </span>
  );
}

const MODES: { value: LinkMode; label: (hostname: string) => string }[] = [
  { value: "hostname", label: (h) => h || "Hostname" },
  { value: "lan-ip", label: () => "LAN IP" },
  { value: "smart", label: () => "Smart" },
];

/** The Hostname / LAN IP / Smart segmented control. */
export function LinkModeSegment({
  mode,
  hostname,
  onChange,
}: {
  mode: LinkMode;
  hostname: string;
  onChange: (m: LinkMode) => void;
}) {
  return (
    <div className="segment" role="group" aria-label="Link address mode">
      {MODES.map((m) => (
        <button
          key={m.value}
          aria-pressed={mode === m.value}
          onClick={() => onChange(m.value)}
        >
          {m.label(hostname)}
        </button>
      ))}
    </div>
  );
}

/** Accessible toggle switch. */
export function Switch({
  checked,
  onChange,
  label,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  label: string;
}) {
  return (
    <button
      role="switch"
      aria-checked={checked}
      aria-label={label}
      className="switch"
      onClick={() => onChange(!checked)}
    >
      <i />
    </button>
  );
}

/** Confidence percentage formatted for display. */
export function confidencePct(v: number | undefined): string {
  return v == null ? "" : `${Math.round(v * 100)}%`;
}

/** Human "ago" string. */
export function ago(iso: string | undefined): string {
  if (!iso) return "—";
  const ms = Date.now() - new Date(iso).getTime();
  if (Number.isNaN(ms) || ms < 0) return "—";
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m`;
  const h = Math.floor(m / 60);
  if (h < 48) return `${h}h`;
  return `${Math.floor(h / 24)}d`;
}

/** Uptime seconds → "31d 4h" style. */
export function uptime(seconds: number | undefined): string {
  if (!seconds || seconds <= 0) return "—";
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  if (d > 0) return `${d}d ${h}h`;
  const m = Math.floor((seconds % 3600) / 60);
  return h > 0 ? `${h}h ${m}m` : `${m}m`;
}
