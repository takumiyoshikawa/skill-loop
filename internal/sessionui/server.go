package sessionui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/takumiyoshikawa/skill-loop/internal/session"
	"github.com/takumiyoshikawa/skill-loop/internal/sessionui/static"
)

type sessionStore struct {
	list       func(repoRoot string) ([]*session.Metadata, error)
	load       func(repoRoot, id string) (*session.Metadata, error)
	reconcile  func(meta *session.Metadata) error
	stop       func(meta *session.Metadata) error
	deleteByID func(repoRoot, id string) error
	readFile   func(path string) ([]byte, error)
}

type handler struct {
	repoRoot string
	store    sessionStore
	static   fs.FS
}

type listResponse struct {
	RepoRoot  string       `json:"repoRoot"`
	UpdatedAt time.Time    `json:"updatedAt"`
	Sessions  []sessionDTO `json:"sessions"`
}

type sessionDTO struct {
	ID                 string         `json:"id"`
	WorkflowName       string         `json:"workflowName"`
	Skill              string         `json:"skill"`
	Runtime            string         `json:"runtime"`
	RepoRoot           string         `json:"repoRoot"`
	WorkingDir         string         `json:"workingDir"`
	ConfigPath         string         `json:"configPath,omitempty"`
	ConfigName         string         `json:"configName"`
	Schedule           string         `json:"schedule,omitempty"`
	Command            []string       `json:"command"`
	TmuxSession        string         `json:"tmuxSession"`
	ScriptPath         string         `json:"scriptPath"`
	SessionDir         string         `json:"sessionDir"`
	StdoutPath         string         `json:"stdoutPath"`
	StderrPath         string         `json:"stderrPath"`
	ExitCodePath       string         `json:"exitCodePath"`
	Status             session.Status `json:"status"`
	Detail             string         `json:"detail"`
	PID                int            `json:"pid,omitempty"`
	StartedAt          time.Time      `json:"startedAt"`
	LastOutputAt       time.Time      `json:"lastOutputAt"`
	EndedAt            *time.Time     `json:"endedAt,omitempty"`
	NextRun            *time.Time     `json:"nextRun,omitempty"`
	CurrentIteration   int            `json:"currentIteration,omitempty"`
	MaxIterations      int            `json:"maxIterations,omitempty"`
	CurrentSkill       string         `json:"currentSkill,omitempty"`
	IdleTimeoutSeconds int            `json:"idleTimeoutSeconds"`
	MaxRestarts        int            `json:"maxRestarts"`
	RestartCount       int            `json:"restartCount"`
	LastError          string         `json:"lastError,omitempty"`
}

type logResponse struct {
	SessionID string `json:"sessionId"`
	Stream    string `json:"stream"`
	Path      string `json:"path"`
	Content   string `json:"content"`
}

type pruneRequest struct {
	All bool `json:"all"`
}

type pruneResponse struct {
	Pruned             int      `json:"pruned"`
	PrunedSessionIDs   []string `json:"prunedSessionIds"`
	SkippedRunning     int      `json:"skippedRunning"`
	SkippedNonTerminal int      `json:"skippedNonTerminal"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func NewHandler(repoRoot string) (http.Handler, error) {
	sub, err := fs.Sub(static.Frontend, "dist")
	if err != nil {
		return nil, fmt.Errorf("load embedded frontend: %w", err)
	}

	h := &handler{
		repoRoot: repoRoot,
		store: sessionStore{
			list:       session.List,
			load:       session.LoadByID,
			reconcile:  session.Reconcile,
			stop:       session.Stop,
			deleteByID: session.DeleteByID,
			readFile:   os.ReadFile,
		},
		static: sub,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/sessions", h.handleListSessions)
	mux.HandleFunc("GET /api/sessions/{id}", h.handleGetSession)
	mux.HandleFunc("GET /api/sessions/{id}/logs/{stream}", h.handleGetLog)
	mux.HandleFunc("POST /api/sessions/{id}/stop", h.handleStopSession)
	mux.HandleFunc("DELETE /api/sessions/{id}", h.handleDeleteSession)
	mux.HandleFunc("POST /api/sessions/prune", h.handlePruneSessions)
	mux.Handle("/", h.handleSPA())

	return mux, nil
}

func (h *handler) handleListSessions(w http.ResponseWriter, r *http.Request) {
	metas, err := h.listRunSessions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	sessions := make([]sessionDTO, 0, len(metas))
	for _, meta := range metas {
		sessions = append(sessions, toSessionDTO(meta))
	}

	writeJSON(w, http.StatusOK, listResponse{
		RepoRoot:  h.repoRoot,
		UpdatedAt: time.Now().UTC(),
		Sessions:  sessions,
	})
}

func (h *handler) handleGetSession(w http.ResponseWriter, r *http.Request) {
	meta, err := h.loadRunSession(r.PathValue("id"))
	if err != nil {
		h.writeSessionError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toSessionDTO(meta))
}

func (h *handler) handleGetLog(w http.ResponseWriter, r *http.Request) {
	meta, err := h.loadRunSession(r.PathValue("id"))
	if err != nil {
		h.writeSessionError(w, err)
		return
	}

	stream := r.PathValue("stream")
	logPath, ok := logPathForStream(meta, stream)
	if !ok {
		writeErrorMessage(w, http.StatusNotFound, "unknown log stream")
		return
	}

	content, err := readLogFile(h.store.readFile, logPath, 400)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, logResponse{
		SessionID: meta.ID,
		Stream:    stream,
		Path:      logPath,
		Content:   content,
	})
}

func (h *handler) handleStopSession(w http.ResponseWriter, r *http.Request) {
	meta, err := h.loadRunSession(r.PathValue("id"))
	if err != nil {
		h.writeSessionError(w, err)
		return
	}

	if err := h.store.stop(meta); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, toSessionDTO(meta))
}

func (h *handler) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	meta, err := h.loadRunSession(r.PathValue("id"))
	if err != nil {
		h.writeSessionError(w, err)
		return
	}

	if meta.Status == session.StatusRunning || meta.Status == session.StatusScheduled {
		writeErrorMessage(w, http.StatusConflict, "stop the session before deleting it")
		return
	}

	if err := h.store.deleteByID(h.repoRoot, meta.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) handlePruneSessions(w http.ResponseWriter, r *http.Request) {
	var req pruneRequest
	if r.Body != nil {
		defer func() {
			_ = r.Body.Close()
		}()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeErrorMessage(w, http.StatusBadRequest, "invalid prune request")
			return
		}
	}

	metas, err := h.listRunSessions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	result := pruneResponse{}
	for _, meta := range metas {
		switch {
		case meta.Status == session.StatusRunning || meta.Status == session.StatusScheduled:
			result.SkippedRunning++
		case !req.All && !isTerminalStatus(meta.Status):
			result.SkippedNonTerminal++
		default:
			if err := h.store.deleteByID(h.repoRoot, meta.ID); err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			result.Pruned++
			result.PrunedSessionIDs = append(result.PrunedSessionIDs, meta.ID)
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *handler) handleSPA() http.Handler {
	fileServer := http.FileServerFS(h.static)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		cleanPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if cleanPath == "." || cleanPath == "" {
			cleanPath = "index.html"
		}

		if _, err := fs.Stat(h.static, cleanPath); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		index, err := fs.ReadFile(h.static, "index.html")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	})
}

func (h *handler) listRunSessions() ([]*session.Metadata, error) {
	all, err := h.store.list(h.repoRoot)
	if err != nil {
		return nil, err
	}

	runs := make([]*session.Metadata, 0, len(all))
	for _, meta := range all {
		if meta.Skill != "orchestrator" {
			continue
		}
		if err := h.store.reconcile(meta); err != nil {
			return nil, err
		}
		runs = append(runs, meta)
	}

	return runs, nil
}

func (h *handler) loadRunSession(id string) (*session.Metadata, error) {
	meta, err := h.store.load(h.repoRoot, id)
	if err != nil {
		return nil, err
	}
	if meta.Skill != "orchestrator" {
		return nil, os.ErrNotExist
	}
	if err := h.store.reconcile(meta); err != nil {
		return nil, err
	}
	return meta, nil
}

func (h *handler) writeSessionError(w http.ResponseWriter, err error) {
	if errors.Is(err, os.ErrNotExist) {
		writeErrorMessage(w, http.StatusNotFound, "session not found")
		return
	}
	writeError(w, http.StatusInternalServerError, err)
}

func toSessionDTO(meta *session.Metadata) sessionDTO {
	return sessionDTO{
		ID:                 meta.ID,
		WorkflowName:       meta.WorkflowName,
		Skill:              meta.Skill,
		Runtime:            meta.Runtime,
		RepoRoot:           meta.RepoRoot,
		WorkingDir:         meta.WorkingDir,
		ConfigPath:         meta.ConfigPath,
		ConfigName:         sessionConfigName(meta),
		Schedule:           meta.Schedule,
		Command:            append([]string(nil), meta.Command...),
		TmuxSession:        meta.TmuxSession,
		ScriptPath:         meta.ScriptPath,
		SessionDir:         filepath.Dir(meta.ScriptPath),
		StdoutPath:         meta.StdoutPath,
		StderrPath:         meta.StderrPath,
		ExitCodePath:       meta.ExitCodePath,
		Status:             meta.Status,
		Detail:             formatSessionDetail(meta),
		PID:                meta.PID,
		StartedAt:          meta.StartedAt,
		LastOutputAt:       meta.LastOutputAt,
		EndedAt:            meta.EndedAt,
		NextRun:            meta.NextRun,
		CurrentIteration:   meta.CurrentIteration,
		MaxIterations:      meta.MaxIterations,
		CurrentSkill:       meta.CurrentSkill,
		IdleTimeoutSeconds: meta.IdleTimeoutSeconds,
		MaxRestarts:        meta.MaxRestarts,
		RestartCount:       meta.RestartCount,
		LastError:          meta.LastError,
	}
}

func formatSessionDetail(meta *session.Metadata) string {
	switch meta.Status {
	case session.StatusScheduled:
		if meta.NextRun == nil {
			if meta.LastError != "" {
				return "next: n/a error: " + meta.LastError
			}
			return "next: n/a"
		}
		detail := "next: " + meta.NextRun.Local().Format(time.DateTime)
		if meta.LastError != "" {
			detail += " error: " + meta.LastError
		}
		return detail
	case session.StatusRunning:
		if meta.Schedule != "" && meta.MaxIterations > 0 {
			return fmt.Sprintf("iter: %d/%d", meta.CurrentIteration, meta.MaxIterations)
		}
		return "last_output: " + meta.LastOutputAt.Local().Format(time.DateTime)
	case session.StatusFailed, session.StatusStopped:
		if meta.LastError != "" {
			return meta.LastError
		}
		if meta.EndedAt != nil {
			return "ended: " + meta.EndedAt.Local().Format(time.DateTime)
		}
		return string(meta.Status)
	default:
		if meta.EndedAt != nil {
			return "ended: " + meta.EndedAt.Local().Format(time.DateTime)
		}
		return meta.LastOutputAt.Local().Format(time.DateTime)
	}
}

func sessionConfigName(meta *session.Metadata) string {
	if meta.WorkflowName != "" {
		return meta.WorkflowName
	}
	if meta.ConfigPath == "" {
		return "-"
	}
	return filepath.Base(meta.ConfigPath)
}

func isTerminalStatus(status session.Status) bool {
	return status == session.StatusDone || status == session.StatusFailed || status == session.StatusStopped
}

func logPathForStream(meta *session.Metadata, stream string) (string, bool) {
	switch stream {
	case "stdout":
		return meta.StdoutPath, true
	case "stderr":
		return meta.StderrPath, true
	default:
		return "", false
	}
}

func readLogFile(readFile func(path string) ([]byte, error), path string, tail int) (string, error) {
	data, err := readFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}

	content := string(data)
	if tail > 0 {
		content = tailLines(content, tail)
	}
	return content, nil
}

func tailLines(s string, n int) string {
	if n <= 0 {
		return s
	}

	trimmedTrailingNewline := strings.HasSuffix(s, "\n")
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return ""
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	result := strings.Join(lines, "\n")
	if trimmedTrailingNewline && result != "" {
		result += "\n"
	}
	return result
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeErrorMessage(w, status, err.Error())
}

func writeErrorMessage(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}
