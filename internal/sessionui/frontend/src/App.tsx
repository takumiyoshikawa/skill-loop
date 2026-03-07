import { useEffect, useMemo, useState } from "react";
import { SessionContent } from "./components/SessionContent";
import { Sidebar } from "./components/Sidebar";
import type { LogPayload, Session, SessionStatus, SessionsPayload } from "./types";
import { getErrorMessage, getJSON, groupSessions, send, sendJSON } from "./utils";

const POLL_MS = 4000;

export default function App() {
  const [repoRoot, setRepoRoot] = useState("");
  const [sessions, setSessions] = useState<Session[]>([]);
  const [selectedId, setSelectedId] = useState("");
  const [statusFilters, setStatusFilters] = useState<Array<SessionStatus | "all">>(["all"]);
  const [query, setQuery] = useState("");
  const [activeStream, setActiveStream] = useState<"stdout" | "stderr">("stdout");
  const [log, setLog] = useState<LogPayload | null>(null);
  const [loading, setLoading] = useState(true);
  const [mutating, setMutating] = useState(false);
  const [error, setError] = useState("");
  const [flash, setFlash] = useState("");

  useEffect(() => {
    let cancelled = false;

    const refresh = async () => {
      try {
        const payload = await getJSON<SessionsPayload>("/api/sessions");
        if (cancelled) {
          return;
        }
        setRepoRoot(payload.repoRoot);
        setSessions(payload.sessions);
        setSelectedId((current) => {
          if (current && payload.sessions.some((session) => session.id === current)) {
            return current;
          }
          return payload.sessions[0]?.id ?? "";
        });
        setError("");
      } catch (err) {
        if (!cancelled) {
          setError(getErrorMessage(err));
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    refresh();
    const timer = window.setInterval(refresh, POLL_MS);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, []);

  const selectedSession = useMemo(
    () => sessions.find((session) => session.id === selectedId) ?? null,
    [selectedId, sessions],
  );

  useEffect(() => {
    if (!selectedSession) {
      setLog(null);
      return;
    }

    let cancelled = false;
    const refreshLog = async () => {
      try {
        const payload = await getJSON<LogPayload>(
          `/api/sessions/${selectedSession.id}/logs/${activeStream}`,
        );
        if (!cancelled) {
          setLog(payload);
        }
      } catch (err) {
        if (!cancelled) {
          setError(getErrorMessage(err));
        }
      }
    };

    refreshLog();
    const timer = window.setInterval(refreshLog, POLL_MS);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [activeStream, selectedSession]);

  const groupedSessions = useMemo(
    () => groupSessions(sessions, statusFilters, query),
    [query, sessions, statusFilters],
  );

  const repoName = useMemo(() => {
    const parts = repoRoot.split("/").filter(Boolean);
    return parts.at(-1) ?? "skill-loop";
  }, [repoRoot]);

  async function refreshAll() {
    try {
      const payload = await getJSON<SessionsPayload>("/api/sessions");
      setRepoRoot(payload.repoRoot);
      setSessions(payload.sessions);
      setSelectedId((current) => {
        if (current && payload.sessions.some((session) => session.id === current)) {
          return current;
        }
        return payload.sessions[0]?.id ?? "";
      });
      setError("");
    } catch (err) {
      setError(getErrorMessage(err));
    }
  }

  async function refreshSelected() {
    if (!selectedSession) {
      return;
    }
    try {
      const payload = await getJSON<Session>(`/api/sessions/${selectedSession.id}`);
      setSessions((current) =>
        current.map((session) => (session.id === payload.id ? payload : session)),
      );
      setError("");
    } catch (err) {
      setError(getErrorMessage(err));
    }
  }

  async function stopSelected() {
    if (!selectedSession) {
      return;
    }
    setMutating(true);
    try {
      const payload = await sendJSON<Session>(`/api/sessions/${selectedSession.id}/stop`, {
        method: "POST",
      });
      setSessions((current) =>
        current.map((session) => (session.id === payload.id ? payload : session)),
      );
      setFlash(`Stopped ${payload.id}`);
      setError("");
    } catch (err) {
      setError(getErrorMessage(err));
    } finally {
      setMutating(false);
    }
  }

  async function deleteSelected() {
    if (!selectedSession) {
      return;
    }
    if (!window.confirm(`Delete session ${selectedSession.id}?`)) {
      return;
    }
    setMutating(true);
    try {
      await send(`/api/sessions/${selectedSession.id}`, { method: "DELETE" });
      setSessions((current) => current.filter((session) => session.id !== selectedSession.id));
      setFlash(`Deleted ${selectedSession.id}`);
      setError("");
    } catch (err) {
      setError(getErrorMessage(err));
    } finally {
      setMutating(false);
    }
  }

  async function prune(all: boolean) {
    setMutating(true);
    try {
      const result = await sendJSON<{ pruned: number }>(
        "/api/sessions/prune",
        {
          method: "POST",
          body: JSON.stringify({ all }),
        },
      );
      await refreshAll();
      setFlash(
        all
          ? `Pruned ${result.pruned} inactive sessions`
          : `Pruned ${result.pruned} finished sessions`,
      );
    } catch (err) {
      setError(getErrorMessage(err));
    } finally {
      setMutating(false);
    }
  }

  useEffect(() => {
    if (!flash) {
      return;
    }
    const timer = window.setTimeout(() => setFlash(""), 2400);
    return () => window.clearTimeout(timer);
  }, [flash]);

  return (
    <div className="app-shell">
      <Sidebar
        repoName={repoName}
        repoRoot={repoRoot}
        query={query}
        statusFilters={statusFilters}
        loading={loading}
        groupedSessions={groupedSessions}
        selectedId={selectedId}
        onQueryChange={setQuery}
        onStatusFilterChange={setStatusFilters}
        onRefresh={() => void refreshAll()}
        onSelect={setSelectedId}
      />

      <SessionContent
        selectedSession={selectedSession}
        log={log}
        activeStream={activeStream}
        mutating={mutating}
        onPruneInactive={() => void prune(true)}
        onRefreshSelected={() => void refreshSelected()}
        onStopSelected={() => void stopSelected()}
        onDeleteSelected={() => void deleteSelected()}
        onActiveStreamChange={setActiveStream}
      />

      {(error || flash) && (
        <div className={error ? "toast error" : "toast success"}>{error || flash}</div>
      )}
    </div>
  );
}
