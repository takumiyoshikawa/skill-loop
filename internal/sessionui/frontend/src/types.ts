export type SessionStatus =
  | "pending"
  | "scheduled"
  | "running"
  | "idle"
  | "done"
  | "failed"
  | "stopped";

export type Session = {
  id: string;
  workflowName: string;
  skill: string;
  runtime: string;
  repoRoot: string;
  workingDir: string;
  configPath?: string;
  configName: string;
  schedule?: string;
  command: string[];
  tmuxSession: string;
  scriptPath: string;
  sessionDir: string;
  stdoutPath: string;
  stderrPath: string;
  exitCodePath: string;
  status: SessionStatus;
  detail: string;
  pid?: number;
  startedAt: string;
  lastOutputAt: string;
  endedAt?: string;
  nextRun?: string;
  currentIteration?: number;
  maxIterations?: number;
  currentSkill?: string;
  idleTimeoutSeconds: number;
  maxRestarts: number;
  restartCount: number;
  lastError?: string;
};

export type SessionsPayload = {
  repoRoot: string;
  updatedAt: string;
  sessions: Session[];
};

export type LogPayload = {
  sessionId: string;
  stream: "stdout" | "stderr";
  path: string;
  content: string;
};

export type ErrorPayload = {
  error: string;
};

export type SessionGroup = {
  name: string;
  items: Session[];
};
