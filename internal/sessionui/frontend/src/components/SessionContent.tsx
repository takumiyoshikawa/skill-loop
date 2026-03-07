import { formatCompactDate } from "../utils";
import type { LogPayload, Session } from "../types";

type SessionContentProps = {
  selectedSession: Session | null;
  log: LogPayload | null;
  activeStream: "stdout" | "stderr";
  mutating: boolean;
  onPruneInactive: () => void;
  onRefreshSelected: () => void;
  onStopSelected: () => void;
  onDeleteSelected: () => void;
  onActiveStreamChange: (stream: "stdout" | "stderr") => void;
};

export function SessionContent({
  selectedSession,
  log,
  activeStream,
  mutating,
  onPruneInactive,
  onRefreshSelected,
  onStopSelected,
  onDeleteSelected,
  onActiveStreamChange,
}: SessionContentProps) {
  return (
    <main className="content">
      <header className="content-header">
        <div>
          <p className="eyebrow">Sessions</p>
          <h2>{selectedSession ? selectedSession.workflowName : "No selection"}</h2>
        </div>
        <div className="action-row">
          <button type="button" className="secondary-button" onClick={onPruneInactive} disabled={mutating}>
            Prune inactive
          </button>
          {selectedSession ? (
            <>
              <button type="button" className="secondary-button" onClick={onRefreshSelected}>
                Refresh
              </button>
              <button
                type="button"
                className="secondary-button"
                onClick={onStopSelected}
                disabled={
                  mutating ||
                  !["running", "scheduled", "idle", "pending"].includes(selectedSession.status)
                }
              >
                Stop
              </button>
              <button
                type="button"
                className="danger-button"
                onClick={onDeleteSelected}
                disabled={
                  mutating ||
                  selectedSession.status === "running" ||
                  selectedSession.status === "scheduled"
                }
              >
                Delete
              </button>
            </>
          ) : null}
        </div>
      </header>

      {selectedSession ? (
        <div className="content-body">
          <section className="summary-card">
            <p className="eyebrow">Description</p>
            <div className="session-title">
              <span className={`status-pill status-${selectedSession.status}`}>
                {selectedSession.status}
              </span>
              <h3>{selectedSession.id}</h3>
            </div>
            <div className="meta-grid">
              <Meta label="Workflow" value={selectedSession.workflowName} />
              <Meta label="Config" value={selectedSession.configName} />
              <Meta label="Runtime" value={selectedSession.runtime} />
              <Meta label="Started" value={formatCompactDate(selectedSession.startedAt)} />
              <Meta label="Last output" value={formatCompactDate(selectedSession.lastOutputAt)} />
              <Meta
                label="Iterations"
                value={
                  selectedSession.maxIterations
                    ? `${selectedSession.currentIteration ?? 0}/${selectedSession.maxIterations}`
                    : "-"
                }
              />
            </div>
            <div className="inline-detail-list">
              <InlineDetail label="Working dir" value={selectedSession.workingDir} />
              <InlineDetail label="Session dir" value={selectedSession.sessionDir} />
              <InlineDetail label="stdout" value={selectedSession.stdoutPath} />
              <InlineDetail label="stderr" value={selectedSession.stderrPath} />
              <InlineDetail label="Command" value={selectedSession.command.join(" ")} mono />
            </div>
            {selectedSession.lastError ? (
              <div className="error-banner">{selectedSession.lastError}</div>
            ) : null}
          </section>

          <section className="log-card">
            <div className="log-header">
              <div>
                <p className="eyebrow">Log</p>
                <div className="log-path" title={log?.path || ""}>
                  {log?.path || "No log file"}
                </div>
              </div>
              <div className="tab-row">
                {(["stdout", "stderr"] as const).map((stream) => (
                  <button
                    key={stream}
                    type="button"
                    className={stream === activeStream ? "tab-button active" : "tab-button"}
                    onClick={() => onActiveStreamChange(stream)}
                  >
                    {stream}
                  </button>
                ))}
              </div>
            </div>
            <pre className="log-output">{log?.content || "(empty)"}</pre>
          </section>
        </div>
      ) : (
        <section className="empty-panel">
          <p>Select a session from the sidebar.</p>
        </section>
      )}
    </main>
  );
}

function Meta({ label, value }: { label: string; value: string }) {
  return (
    <div className="meta-item">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function InlineDetail({
  label,
  value,
  mono,
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <div className="inline-detail">
      <span>{label}</span>
      <code className={mono ? "mono" : undefined}>{value}</code>
    </div>
  );
}
