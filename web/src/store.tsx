import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  api,
  subscribe,
  type ContainerView,
  type LinkMode,
  type StatusPayload,
} from "./api";

interface AppState {
  containers: ContainerView[];
  status: StatusPayload | null;
  loading: boolean;
  error: string | null;
  connected: boolean;
  linkMode: LinkMode;
  setLinkMode: (m: LinkMode) => void;
  refresh: () => Promise<void>;
  reload: () => Promise<void>;
}

const Ctx = createContext<AppState | null>(null);

const LINK_MODE_KEY = "arraydeck-link-mode";

export function AppStateProvider({ children }: { children: ReactNode }) {
  const [containers, setContainers] = useState<ContainerView[]>([]);
  const [status, setStatus] = useState<StatusPayload | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [connected, setConnected] = useState(false);
  const adoptedDefault = useRef(false);
  const [linkMode, setLinkModeState] = useState<LinkMode>(() => {
    const stored = localStorage.getItem(LINK_MODE_KEY);
    return stored === "hostname" || stored === "lan-ip" || stored === "smart"
      ? stored
      : "hostname";
  });

  const setLinkMode = useCallback((m: LinkMode) => {
    setLinkModeState(m);
    localStorage.setItem(LINK_MODE_KEY, m);
  }, []);

  const reload = useCallback(async () => {
    try {
      const [statusRes, containersRes] = await Promise.all([
        api.status(),
        api.containers(),
      ]);
      setStatus(statusRes);
      setContainers(sortViews(containersRes.containers ?? []));
      setError(null);
      // First visit in this browser: adopt the server-wide default link mode.
      if (!adoptedDefault.current && !localStorage.getItem(LINK_MODE_KEY)) {
        setLinkModeState(statusRes.settings.linkMode);
      }
      adoptedDefault.current = true;
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  const refresh = useCallback(async () => {
    await api.reconcile();
  }, []);

  useEffect(() => {
    void reload();
    const close = subscribe({
      onPatch: (patch) => {
        setContainers((prev) => {
          const byId = new Map(prev.map((c) => [c.id, c]));
          for (const up of patch.upserts ?? []) byId.set(up.id, up);
          for (const id of patch.removals ?? []) byId.delete(id);
          return sortViews([...byId.values()]);
        });
        setStatus((prev) => (prev ? { ...prev, stats: patch.stats } : prev));
      },
      onStatus: (s) => {
        setStatus(s);
        if (!adoptedDefault.current && !localStorage.getItem(LINK_MODE_KEY)) {
          setLinkModeState(s.settings.linkMode);
          adoptedDefault.current = true;
        }
      },
      onConnectionChange: (ok) => {
        setConnected(ok);
        if (ok) void reload(); // resync after a dropped stream
      },
    });
    return close;
  }, [reload]);

  const value = useMemo<AppState>(
    () => ({
      containers,
      status,
      loading,
      error,
      connected,
      linkMode,
      setLinkMode,
      refresh,
      reload,
    }),
    [containers, status, loading, error, connected, linkMode, setLinkMode, refresh, reload],
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

function sortViews(views: ContainerView[]): ContainerView[] {
  return [...views].sort((a, b) =>
    a.displayName.toLowerCase().localeCompare(b.displayName.toLowerCase()),
  );
}

export function useAppState(): AppState {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error("useAppState outside provider");
  return ctx;
}

export type ThemeChoice = "light" | "dark" | "system";

export function applyTheme(choice: ThemeChoice) {
  localStorage.setItem("arraydeck-theme", choice);
  const dark =
    choice === "dark" ||
    (choice === "system" &&
      window.matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.dataset.theme = dark ? "dark" : "light";
}

export function currentTheme(): ThemeChoice {
  const stored = localStorage.getItem("arraydeck-theme");
  return stored === "dark" || stored === "system" ? stored : "light";
}
