import type { ErrorPayload, Session, SessionGroup, SessionStatus } from "./types";

export const STATUS_OPTIONS = [
  "all",
  "running",
  "blocked",
  "scheduled",
  "failed",
  "done",
  "stopped",
  "idle",
] as const;

export function groupSessions(
  sessions: Session[],
  statusFilters: Array<SessionStatus | "all">,
  query: string,
): SessionGroup[] {
  const normalizedQuery = query.trim().toLowerCase();
  const appliesAll = statusFilters.length === 0 || statusFilters.includes("all");
  const filtered = sessions.filter((session) => {
    if (!appliesAll && !statusFilters.includes(session.status)) {
      return false;
    }
    if (normalizedQuery === "") {
      return true;
    }

    const haystack = [
      session.workflowName,
      session.id,
      session.status,
      session.configName,
      session.workingDir,
      session.detail,
      session.command.join(" "),
    ]
      .join(" ")
      .toLowerCase();
    return haystack.includes(normalizedQuery);
  });

  const groups = new Map<string, Session[]>();
  for (const item of filtered) {
    const key = item.workflowName || "default";
    const existing = groups.get(key);
    if (existing) {
      existing.push(item);
    } else {
      groups.set(key, [item]);
    }
  }

  return Array.from(groups.entries())
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([name, items]) => ({ name, items }));
}

export function statusToneClass(status: SessionStatus): string {
  switch (status) {
    case "running":
      return "tone-green";
    case "scheduled":
      return "tone-blue";
    case "blocked":
      return "tone-amber";
    case "done":
      return "tone-green";
    case "failed":
    case "stopped":
      return "tone-red";
    default:
      return "tone-slate";
  }
}

export async function getJSON<T>(url: string): Promise<T> {
  const response = await send(url);
  return (await response.json()) as T;
}

export async function sendJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await send(url, init);
  return (await response.json()) as T;
}

export async function send(url: string, init?: RequestInit): Promise<Response> {
  const response = await fetch(url, {
    headers: {
      "Content-Type": "application/json",
    },
    ...init,
  });

  if (!response.ok) {
    let payload: ErrorPayload | null = null;
    try {
      payload = (await response.json()) as ErrorPayload;
    } catch {
      payload = null;
    }
    throw new Error(payload?.error || `${response.status} ${response.statusText}`);
  }

  return response;
}

export function getErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return "Unexpected error";
}

export function formatCompactDate(value?: string): string {
  if (!value) {
    return "-";
  }
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}
