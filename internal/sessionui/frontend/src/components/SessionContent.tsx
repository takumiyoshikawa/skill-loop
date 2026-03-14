import { FormEvent } from "react";
import type { LogPayload, Session } from "../types";

type SessionContentProps = {
  selectedSession: Session | null;
  log: LogPayload | null;
  activeStream: "stdout" | "stderr";
  mutating: boolean;
  resumeDraft: string;
  onPruneInactive: () => void;
  onRefreshSelected: () => void;
  onStopSelected: () => void;
  onResumeDraftChange: (value: string) => void;
  onResumeSelected: (prompt: string) => void;
  onActiveStreamChange: (stream: "stdout" | "stderr") => void;
};

export function SessionContent({
  selectedSession,
  log,
  activeStream,
  mutating,
  resumeDraft,
  onPruneInactive,
  onRefreshSelected,
  onStopSelected,
  onResumeDraftChange,
  onResumeSelected,
  onActiveStreamChange,
}: SessionContentProps) {
  function handleResumeSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onResumeSelected(resumeDraft);
  }

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
            </>
          ) : null}
        </div>
      </header>

      {selectedSession ? (
        <div className="content-body">
          <section className="summary-card">
            <div className="summary-section summary-section-top">
              <span className="section-label">Previous summary</span>
              <pre className="summary-output">
                {selectedSession.previousSummary || "(empty)"}
              </pre>
            </div>
            {selectedSession.status === "blocked" ? (
              <form className="resume-form" onSubmit={handleResumeSubmit}>
                <div className="resume-header">
                  <div>
                    <span className="section-label">Resume prompt</span>
                  </div>
                  <button type="submit" className="secondary-button" disabled={mutating}>
                    Resume
                  </button>
                </div>
                <textarea
                  value={resumeDraft}
                  onChange={(event) => onResumeDraftChange(event.target.value)}
                  placeholder="Add human guidance, approval, or constraints here."
                  disabled={mutating}
                />
              </form>
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
