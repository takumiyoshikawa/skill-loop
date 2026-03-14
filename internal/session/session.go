package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultIdleTimeoutSeconds = 900
	DefaultMaxRestarts        = 2
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusScheduled Status = "scheduled"
	StatusRunning   Status = "running"
	StatusBlocked   Status = "blocked"
	StatusIdle      Status = "idle"
	StatusDone      Status = "done"
	StatusFailed    Status = "failed"
	StatusStopped   Status = "stopped"
)

type Metadata struct {
	ID                 string     `json:"id"`
	WorkflowName       string     `json:"workflow_name,omitempty"`
	StorageName        string     `json:"storage_name,omitempty"`
	Skill              string     `json:"skill"`
	Runtime            string     `json:"runtime"`
	RepoRoot           string     `json:"repo_root"`
	WorkingDir         string     `json:"working_dir"`
	ConfigPath         string     `json:"config_path,omitempty"`
	Schedule           string     `json:"schedule,omitempty"`
	Command            []string   `json:"command"`
	TmuxSession        string     `json:"tmux_session"`
	ScriptPath         string     `json:"script_path"`
	StdoutPath         string     `json:"stdout_path"`
	StderrPath         string     `json:"stderr_path"`
	ExitCodePath       string     `json:"exit_code_path"`
	Status             Status     `json:"status"`
	PID                int        `json:"pid,omitempty"`
	StartedAt          time.Time  `json:"started_at"`
	LastOutputAt       time.Time  `json:"last_output_at"`
	EndedAt            *time.Time `json:"ended_at,omitempty"`
	NextRun            *time.Time `json:"next_run,omitempty"`
	CurrentIteration   int        `json:"current_iteration,omitempty"`
	MaxIterations      int        `json:"max_iterations,omitempty"`
	CurrentSkill       string     `json:"current_skill,omitempty"`
	LastSkillOutput    string     `json:"last_skill_output,omitempty"`
	BlockReason        string     `json:"block_reason,omitempty"`
	ResumeSkill        string     `json:"resume_skill,omitempty"`
	ResumePrompt       string     `json:"resume_prompt,omitempty"`
	IdleTimeoutSeconds int        `json:"idle_timeout_seconds"`
	MaxRestarts        int        `json:"max_restarts"`
	RestartCount       int        `json:"restart_count"`
	LastError          string     `json:"last_error,omitempty"`
}

func ResolveRepoRoot(cwd string) (string, error) {
	if cwd == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
		cwd = wd
	}

	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && strings.Contains(string(out), "not a git repository") {
			return cwd, nil
		}
		return "", fmt.Errorf("resolve repo root: %w", err)
	}

	repoRoot := strings.TrimSpace(string(out))
	if repoRoot == "" {
		return cwd, nil
	}
	return repoRoot, nil
}

func SessionsRoot(repoRoot string) string {
	_ = repoRoot
	root, err := skillLoopDataRoot()
	if err != nil {
		if repoRoot != "" {
			return filepath.Join(repoRoot, ".skill-loop")
		}
		return ".skill-loop"
	}
	return root
}

func New(repoRoot string, workingDir string, workflowName string, skill string, runtime string, command []string, idleTimeout time.Duration, maxRestarts int) (*Metadata, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("command is required")
	}

	if workingDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
		workingDir = wd
	}

	now := time.Now().UTC()
	id, err := newID(now)
	if err != nil {
		return nil, err
	}

	workflowName = sanitizePathSegment(workflowName)
	storageName, err := newStorageName()
	if err != nil {
		return nil, err
	}

	sessionDir := filepath.Join(SessionsRoot(repoRoot), workflowName, storageName)
	if err := os.MkdirAll(sessionDir, 0o750); err != nil {
		return nil, fmt.Errorf("create session directory: %w", err)
	}

	idleTimeoutSeconds := int(idleTimeout.Seconds())
	if idleTimeoutSeconds <= 0 {
		idleTimeoutSeconds = DefaultIdleTimeoutSeconds
	}
	if maxRestarts < 0 {
		maxRestarts = DefaultMaxRestarts
	}

	meta := &Metadata{
		ID:                 id,
		WorkflowName:       workflowName,
		StorageName:        storageName,
		Skill:              skill,
		Runtime:            runtime,
		RepoRoot:           repoRoot,
		WorkingDir:         workingDir,
		Command:            append([]string(nil), command...),
		TmuxSession:        fmt.Sprintf("skill-loop-%s", id),
		ScriptPath:         filepath.Join(sessionDir, "run.sh"),
		StdoutPath:         filepath.Join(sessionDir, "stdout.log"),
		StderrPath:         filepath.Join(sessionDir, "stderr.log"),
		ExitCodePath:       filepath.Join(sessionDir, "exit.code"),
		Status:             StatusPending,
		StartedAt:          now,
		LastOutputAt:       now,
		IdleTimeoutSeconds: idleTimeoutSeconds,
		MaxRestarts:        maxRestarts,
	}

	if err := writeScript(meta); err != nil {
		return nil, err
	}

	if err := Save(meta); err != nil {
		return nil, err
	}

	return meta, nil
}

func Save(meta *Metadata) error {
	sessionFilePath := filepath.Join(filepath.Dir(meta.ScriptPath), "session.json")

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session metadata: %w", err)
	}

	if err := os.WriteFile(sessionFilePath, data, 0o600); err != nil {
		return fmt.Errorf("write session metadata: %w", err)
	}
	return nil
}

func UpdateCommand(meta *Metadata, command []string) error {
	if len(command) == 0 {
		return fmt.Errorf("command is required")
	}
	meta.Command = append([]string(nil), command...)
	meta.WorkingDir = preferredWorkingDir(meta.ConfigPath, meta.WorkingDir)
	if err := writeScript(meta); err != nil {
		return err
	}
	return Save(meta)
}

func LoadByID(repoRoot string, id string) (*Metadata, error) {
	sessionFilePath, err := sessionFileByID(repoRoot, id)
	if err != nil {
		return nil, err
	}
	meta, err := LoadFromPath(sessionFilePath)
	if err != nil {
		return nil, err
	}
	if repoRoot != "" && filepath.Clean(meta.RepoRoot) != filepath.Clean(repoRoot) {
		return nil, os.ErrNotExist
	}
	return meta, nil
}

func LoadFromPath(path string) (*Metadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read session metadata: %w", err)
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse session metadata: %w", err)
	}

	return &meta, nil
}

func List(repoRoot string) ([]*Metadata, error) {
	root := SessionsRoot(repoRoot)
	entries, err := sessionFilePaths(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read sessions directory: %w", err)
	}

	metas := make([]*Metadata, 0, len(entries))
	for _, sessionFile := range entries {
		meta, err := LoadFromPath(sessionFile)
		if err != nil {
			continue
		}
		if repoRoot != "" && filepath.Clean(meta.RepoRoot) != filepath.Clean(repoRoot) {
			continue
		}
		metas = append(metas, meta)
	}

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].StartedAt.After(metas[j].StartedAt)
	})

	return metas, nil
}

func DeleteByID(repoRoot string, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("session id is required")
	}
	meta, err := LoadByID(repoRoot, id)
	if err != nil {
		return err
	}
	if meta.TmuxSession != "" {
		if err := killTMuxSession(meta.TmuxSession); err != nil && !errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("kill tmux session %s: %w", meta.TmuxSession, err)
		}
	}
	sessionDir := filepath.Dir(meta.ScriptPath)
	if err := os.RemoveAll(sessionDir); err != nil {
		return fmt.Errorf("delete session directory %s: %w", sessionDir, err)
	}
	return nil
}

func Start(meta *Metadata) error {
	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("tmux is required but not found on PATH")
	}

	if err := os.MkdirAll(filepath.Dir(meta.StdoutPath), 0o750); err != nil {
		return fmt.Errorf("ensure session directory: %w", err)
	}

	if err := os.Remove(meta.ExitCodePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove previous exit code: %w", err)
	}

	_ = killTMuxSession(meta.TmuxSession)

	cmd := exec.Command("tmux", "new-session", "-d", "-s", meta.TmuxSession, "exec bash "+shellQuote(meta.ScriptPath))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("start tmux session: %w", err)
	}
	// Keep pane content after exit so users can inspect run output with attach/capture.
	_ = exec.Command("tmux", "set-option", "-t", meta.TmuxSession, "remain-on-exit", "on").Run()
	if meta.StartedAt.IsZero() {
		meta.StartedAt = time.Now().UTC()
	}
	meta.Status = StatusRunning
	meta.EndedAt = nil
	meta.BlockReason = ""
	meta.ResumeSkill = ""
	meta.ResumePrompt = ""
	meta.LastError = ""
	if meta.LastOutputAt.IsZero() {
		meta.LastOutputAt = meta.StartedAt
	}

	pid, err := panePID(meta.TmuxSession)
	if err == nil {
		meta.PID = pid
	}

	if err := Save(meta); err != nil {
		killErr := killTMuxSession(meta.TmuxSession)
		if killErr != nil {
			return fmt.Errorf("persist started session: %w (rollback failed: %v)", err, killErr)
		}
		return fmt.Errorf("persist started session: %w", err)
	}
	return nil
}

func Stop(meta *Metadata) error {
	if err := killTMuxSession(meta.TmuxSession); err != nil {
		return fmt.Errorf("stop tmux session: %w", err)
	}
	now := time.Now().UTC()
	meta.Status = StatusStopped
	meta.EndedAt = &now
	meta.BlockReason = ""
	meta.ResumeSkill = ""
	meta.ResumePrompt = ""
	meta.LastError = ""
	return Save(meta)
}

func Attach(meta *Metadata) error {
	cmd := exec.Command("tmux", "attach-session", "-t", meta.TmuxSession)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Reconcile(meta *Metadata) error {
	updateLastOutputAt(meta)

	exitCode, hasExitCode, err := ReadExitCode(meta.ExitCodePath)
	if err != nil {
		return err
	}

	if hasExitCode {
		cleanupErr := killTMuxSession(meta.TmuxSession)
		if cleanupErr != nil && !errors.Is(cleanupErr, exec.ErrNotFound) {
			return fmt.Errorf("cleanup tmux session %s: %w", meta.TmuxSession, cleanupErr)
		}

		now := time.Now().UTC()
		if meta.Status == StatusBlocked {
			if meta.EndedAt == nil {
				meta.EndedAt = &now
			}
			return Save(meta)
		}
		if exitCode == 0 {
			meta.Status = StatusDone
			meta.BlockReason = ""
			meta.ResumeSkill = ""
			meta.ResumePrompt = ""
			meta.LastError = ""
		} else {
			meta.Status = StatusFailed
			meta.BlockReason = ""
			meta.ResumeSkill = ""
			meta.ResumePrompt = ""
			meta.LastError = fmt.Sprintf("agent exited with code %d", exitCode)
		}
		if meta.EndedAt == nil {
			meta.EndedAt = &now
		}
		return Save(meta)
	}

	hasSession, err := HasTMuxSession(meta.TmuxSession)
	if err != nil {
		return err
	}
	if hasSession {
		if meta.Status == StatusPending {
			meta.Status = StatusRunning
			meta.EndedAt = nil
			return Save(meta)
		}
		return nil
	}

	if meta.Status == StatusRunning || meta.Status == StatusIdle || meta.Status == StatusPending {
		now := time.Now().UTC()
		meta.Status = StatusFailed
		if meta.LastError == "" {
			meta.LastError = "tmux session disappeared before exit code was written"
		}
		if meta.EndedAt == nil {
			meta.EndedAt = &now
		}
		return Save(meta)
	}

	return nil
}

func BuildResumeCommand(meta *Metadata, humanInput string) ([]string, error) {
	if meta.Status != StatusBlocked {
		return nil, fmt.Errorf("session %s is not blocked", meta.ID)
	}
	if meta.Schedule != "" {
		return nil, fmt.Errorf("resume is not supported for scheduled sessions")
	}
	if meta.ConfigPath == "" {
		return nil, fmt.Errorf("session %s is missing config_path", meta.ID)
	}
	if meta.ResumeSkill == "" {
		return nil, fmt.Errorf("session %s is missing resume_skill", meta.ID)
	}
	if len(meta.Command) == 0 {
		return nil, fmt.Errorf("session %s is missing command template", meta.ID)
	}

	runIdx := -1
	for i := len(meta.Command) - 1; i >= 0; i-- {
		arg := meta.Command[i]
		if arg == "run" {
			runIdx = i
			break
		}
	}
	if runIdx == -1 {
		return nil, fmt.Errorf("session %s command does not contain run subcommand", meta.ID)
	}
	if runIdx+1 >= len(meta.Command) {
		return nil, fmt.Errorf("session %s command is missing config path", meta.ID)
	}

	command := append([]string(nil), meta.Command[:runIdx+2]...)
	command[runIdx+1] = meta.ConfigPath

	skipNext := false
	for _, arg := range meta.Command[runIdx+2:] {
		if skipNext {
			skipNext = false
			continue
		}
		switch arg {
		case "--prompt", "-p", "--entrypoint", "-e":
			skipNext = true
		default:
			command = append(command, arg)
		}
	}

	prompt := strings.TrimSpace(meta.ResumePrompt)
	humanInput = strings.TrimSpace(humanInput)
	switch {
	case prompt != "" && humanInput != "":
		prompt += "\n\nHuman input:\n" + humanInput
	case humanInput != "":
		prompt = humanInput
	}
	if prompt != "" {
		command = append(command, "--prompt", prompt)
	}
	command = append(command, "--entrypoint", meta.ResumeSkill)

	return command, nil
}

func preferredWorkingDir(configPath string, fallback string) string {
	if strings.TrimSpace(configPath) == "" {
		return fallback
	}
	dir := filepath.Dir(configPath)
	if dir == "" || dir == "." {
		return fallback
	}
	return dir
}

func HasTMuxSession(name string) (bool, error) {
	cmd := exec.Command("tmux", "has-session", "-t", name)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		// Exit code 1 means "session not found".
		if exitErr.ExitCode() == 1 {
			return false, nil
		}
	}

	if errors.Is(err, exec.ErrNotFound) {
		return false, fmt.Errorf("tmux is required but not found on PATH: %w", exec.ErrNotFound)
	}
	return false, err
}

func ReadExitCode(path string) (int, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("read exit code file: %w", err)
	}

	s := strings.TrimSpace(string(data))
	if s == "" {
		return 0, false, nil
	}

	code, err := strconv.Atoi(s)
	if err != nil {
		return 0, false, fmt.Errorf("parse exit code %q: %w", s, err)
	}
	return code, true, nil
}

func writeScript(meta *Metadata) error {
	var cmdLine strings.Builder
	for i, arg := range meta.Command {
		if i > 0 {
			cmdLine.WriteByte(' ')
		}
		cmdLine.WriteString(shellQuote(arg))
	}

	content := strings.Join([]string{
		"#!/bin/bash",
		"set +euo pipefail",
		"cd " + shellQuote(meta.WorkingDir),
		"export SKILL_LOOP_SESSION_ID=" + shellQuote(meta.ID),
		"export SKILL_LOOP_SESSION_REPO_ROOT=" + shellQuote(meta.RepoRoot),
		"{ " + cmdLine.String() + " 2> >(tee -a " + shellQuote(meta.StderrPath) + " >&2); } | tee -a " + shellQuote(meta.StdoutPath),
		"code=${PIPESTATUS[0]}",
		"echo \"$code\" > " + shellQuote(meta.ExitCodePath),
		"exit \"$code\"",
		"",
	}, "\n")

	if err := os.WriteFile(meta.ScriptPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write run script: %w", err)
	}
	return nil
}

func panePID(sessionName string) (int, error) {
	cmd := exec.Command("tmux", "list-panes", "-t", sessionName, "-F", "#{pane_pid}")
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	line := strings.TrimSpace(string(out))
	if line == "" {
		return 0, fmt.Errorf("empty pane PID")
	}
	firstLine := strings.Split(line, "\n")[0]
	pid, err := strconv.Atoi(strings.TrimSpace(firstLine))
	if err != nil {
		return 0, fmt.Errorf("parse pane PID %q: %w", firstLine, err)
	}
	return pid, nil
}

func killTMuxSession(name string) error {
	hasSession, err := HasTMuxSession(name)
	if err != nil {
		return err
	}
	if !hasSession {
		return nil
	}
	return exec.Command("tmux", "kill-session", "-t", name).Run()
}

func updateLastOutputAt(meta *Metadata) {
	last := meta.LastOutputAt
	for _, path := range []string{meta.StdoutPath, meta.StderrPath} {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		mod := info.ModTime().UTC()
		if mod.After(last) {
			last = mod
		}
	}
	meta.LastOutputAt = last
}

func newID(now time.Time) (string, error) {
	random, err := randomHex(4)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s", now.Format("20060102T150405Z"), random), nil
}

func newStorageName() (string, error) {
	return randomHex(8)
}

func randomHex(size int) (string, error) {
	buf := make([]byte, 4)
	if size > 0 {
		buf = make([]byte, size)
	}
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func skillLoopDataRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "skill-loop"), nil
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func sessionFileByID(repoRoot string, id string) (string, error) {
	files, err := sessionFilePaths(SessionsRoot(repoRoot))
	if err != nil {
		return "", err
	}

	for _, sessionFile := range files {
		meta, err := LoadFromPath(sessionFile)
		if err != nil {
			continue
		}
		if meta.ID == id {
			return sessionFile, nil
		}
	}

	return "", os.ErrNotExist
}

func sessionFilePaths(root string) ([]string, error) {
	files := make([]string, 0)
	if _, err := os.Stat(root); err != nil {
		return nil, err
	}

	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == "session.json" {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return files, nil
}

func sanitizePathSegment(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "default"
	}

	var b strings.Builder
	lastDash := false
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-', r == '_':
			if b.Len() == 0 || lastDash {
				continue
			}
			b.WriteByte('-')
			lastDash = true
		default:
			if b.Len() == 0 || lastDash {
				continue
			}
			b.WriteByte('-')
			lastDash = true
		}
	}

	name := strings.Trim(strings.ToLower(b.String()), "-")
	if name == "" {
		return "default"
	}
	return name
}
