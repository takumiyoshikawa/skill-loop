import { useEffect, useMemo, useRef, useState } from "react";
import { FolderIcon, SearchIcon } from "./Icons";
import { STATUS_OPTIONS, statusToneClass } from "../utils";
import type { SessionGroup, SessionStatus } from "../types";

type SidebarProps = {
  repoName: string;
  repoRoot: string;
  query: string;
  statusFilters: Array<SessionStatus | "all">;
  loading: boolean;
  groupedSessions: SessionGroup[];
  selectedId: string;
  onQueryChange: (value: string) => void;
  onStatusFilterChange: (value: Array<SessionStatus | "all">) => void;
  onRefresh: () => void;
  onSelect: (id: string) => void;
};

export function Sidebar({
  repoName,
  repoRoot,
  query,
  statusFilters,
  loading,
  groupedSessions,
  selectedId,
  onQueryChange,
  onStatusFilterChange,
  onRefresh,
  onSelect,
}: SidebarProps) {
  const [isFilterOpen, setIsFilterOpen] = useState(false);
  const filterRef = useRef<HTMLDivElement | null>(null);

  const filterLabel = useMemo(() => {
    if (statusFilters.length === 0 || statusFilters.includes("all")) {
      return "All statuses";
    }
    return statusFilters.join(", ");
  }, [statusFilters]);

  useEffect(() => {
    function handlePointerDown(event: PointerEvent) {
      if (!filterRef.current?.contains(event.target as Node)) {
        setIsFilterOpen(false);
      }
    }

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        setIsFilterOpen(false);
      }
    }

    window.addEventListener("pointerdown", handlePointerDown);
    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("pointerdown", handlePointerDown);
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, []);

  function toggleStatus(nextStatus: SessionStatus | "all") {
    if (nextStatus === "all") {
      onStatusFilterChange(["all"]);
      setIsFilterOpen(false);
      return;
    }

    const current = statusFilters.includes("all") ? [] : statusFilters;
    const hasStatus = current.includes(nextStatus);
    const next = hasStatus
      ? current.filter((status) => status !== nextStatus)
      : [...current, nextStatus];

    onStatusFilterChange(next.length === 0 ? ["all"] : next);
    setIsFilterOpen(false);
  }

  return (
    <aside className="sidebar">
      <div className="sidebar-header">
        <div>
          <p className="eyebrow">Workspace</p>
          <h1>{repoName}</h1>
        </div>
        <button type="button" className="secondary-button compact" onClick={onRefresh}>
          Refresh
        </button>
      </div>

      <label className="search-field">
        <SearchIcon />
        <input value={query} onChange={(event) => onQueryChange(event.target.value)} placeholder="Search" />
      </label>

      <div ref={filterRef} className="filter-dropdown">
        <button
          type="button"
          className={isFilterOpen ? "filter-dropdown-trigger active" : "filter-dropdown-trigger"}
          onClick={() => setIsFilterOpen((current) => !current)}
          aria-haspopup="menu"
          aria-expanded={isFilterOpen}
        >
          <span className="eyebrow">Status</span>
          <strong>{filterLabel}</strong>
        </button>
        {isFilterOpen ? (
          <div className="filter-dropdown-menu" role="menu">
            {STATUS_OPTIONS.map((status) => {
              const checked =
                status === "all" ? statusFilters.includes("all") : statusFilters.includes(status);
              return (
                <label key={status} className="filter-option">
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={() => toggleStatus(status)}
                  />
                  <span>{status}</span>
                </label>
              );
            })}
          </div>
        ) : null}
      </div>

      <div className="tree-pane">
        {loading ? (
          <div className="empty-state">Loading sessions...</div>
        ) : groupedSessions.length === 0 ? (
          <div className="empty-state">No sessions.</div>
        ) : (
          groupedSessions.map((group) => (
            <section key={group.name} className="tree-group">
              <div className="tree-folder">
                <FolderIcon />
                <span className="tree-folder-name">{group.name}</span>
                <span className="tree-folder-count">{group.items.length}</span>
              </div>
              <div className="tree-items">
                {group.items.map((session) => (
                  <button
                    key={session.id}
                    type="button"
                    className={session.id === selectedId ? "tree-item active" : "tree-item"}
                    onClick={() => onSelect(session.id)}
                  >
                    <span className={`tree-item-dot ${statusToneClass(session.status)}`} />
                    <div className="tree-item-body">
                      <strong>{session.id}</strong>
                      <span>{session.status}</span>
                    </div>
                  </button>
                ))}
              </div>
            </section>
          ))
        )}
      </div>
    </aside>
  );
}
