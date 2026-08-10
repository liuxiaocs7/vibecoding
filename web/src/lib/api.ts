import type { Project, Issue, ModelConfig, DevSpec, PRInfo, AutoDevLog } from '../types';
import { readSSE } from './stream';

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers || {}),
    },
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.error || `HTTP ${res.status}`);
  }
  return data as T;
}

export interface UIPrefs {
  language: string;
  themeStyle: string;
  activeProjectId: string;
}

export interface HealthInfo {
  status: string;
  timestamp: string;
  llmConfigured: boolean;
}

export interface AutoDevJob {
  id: string;
  issueId: string;
  status: string;
  progress: number;
  phase?: string;
  error?: string;
  prInfo?: PRInfo;
  createdAt: string;
  updatedAt: string;
}

export interface JobEvent {
  type: string;
  phase?: string;
  message?: string;
  details?: string;
  progress?: number;
  status?: string;
  log?: AutoDevLog;
  prInfo?: PRInfo;
  error?: string;
}

export const api = {
  health: () => request<HealthInfo>('/api/health'),

  getModel: () => request<ModelConfig>('/api/settings/model'),
  putModel: (config: ModelConfig) =>
    request<ModelConfig>('/api/settings/model', { method: 'PUT', body: JSON.stringify(config) }),

  getUIPrefs: () => request<UIPrefs>('/api/ui-prefs'),
  putUIPrefs: (prefs: UIPrefs) =>
    request<UIPrefs>('/api/ui-prefs', { method: 'PUT', body: JSON.stringify(prefs) }),

  listProjects: () => request<Project[]>('/api/projects'),
  createProject: (p: Partial<Project>) =>
    request<Project>('/api/projects', { method: 'POST', body: JSON.stringify(p) }),
  updateProject: (id: string, p: Partial<Project>) =>
    request<Project>(`/api/projects/${id}`, { method: 'PUT', body: JSON.stringify(p) }),
  deleteProject: (id: string) =>
    request<{ ok: boolean }>(`/api/projects/${id}`, { method: 'DELETE' }),

  listIssues: (projectId?: string) =>
    request<Issue[]>(projectId ? `/api/issues?projectId=${encodeURIComponent(projectId)}` : '/api/issues'),
  createIssue: (issue: Partial<Issue>) =>
    request<Issue>('/api/issues', { method: 'POST', body: JSON.stringify(issue) }),
  updateIssue: (id: string, issue: Partial<Issue>) =>
    request<Issue>(`/api/issues/${id}`, { method: 'PUT', body: JSON.stringify(issue) }),
  deleteIssue: (id: string) =>
    request<{ ok: boolean }>(`/api/issues/${id}`, { method: 'DELETE' }),
  approveMerge: (id: string) =>
    request<Issue>(`/api/issues/${id}/approve-merge`, { method: 'POST', body: '{}' }),

  validateRepo: (path: string) =>
    request<{ ok: boolean; path?: string; currentBranch?: string; filesCount?: number; error?: string }>(
      '/api/repos/validate',
      { method: 'POST', body: JSON.stringify({ path }) }
    ),

  generateSpec: (issueId: string, body: { prompt?: string; messages?: unknown[] }) =>
    request<{ spec: DevSpec; text: string; chatReply?: string; process?: string }>(
      `/api/issues/${issueId}/spec`,
      {
        method: 'POST',
        body: JSON.stringify(body),
      }
    ),

  /** Stream Dev Spec generation; onDelta receives raw model text as it arrives. */
  generateSpecStream: async (
    issueId: string,
    body: { prompt?: string; messages?: unknown[] },
    opts?: { signal?: AbortSignal; onDelta?: (chunk: string) => void; onStatus?: (msg: string) => void }
  ): Promise<{ spec: DevSpec; text: string; chatReply?: string; process?: string }> => {
    const res = await fetch(`/api/issues/${issueId}/spec?stream=1`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'text/event-stream',
      },
      body: JSON.stringify(body),
      signal: opts?.signal,
    });
    if (!res.ok) {
      const data = await res.json().catch(() => ({}));
      throw new Error((data as { error?: string }).error || `HTTP ${res.status}`);
    }

    let result: { spec: DevSpec; text: string; chatReply?: string; process?: string } | null = null;
    let streamError: string | null = null;

    await readSSE(res, (ev) => {
      if (ev.type === 'delta' && ev.text) {
        opts?.onDelta?.(ev.text);
      } else if (ev.type === 'status' && ev.message) {
        opts?.onStatus?.(ev.message);
      } else if (ev.type === 'error') {
        streamError = ev.error || 'stream error';
      } else if (ev.type === 'done') {
        result = {
          spec: ev.spec as DevSpec,
          text: (ev.chatReply || ev.text || '') as string,
          chatReply: (ev.chatReply || ev.text || '') as string,
          process: (ev.process || '') as string,
        };
      }
    });

    if (streamError) throw new Error(streamError);
    if (!result) throw new Error('Stream ended without result');
    return result;
  },

  startAutoDev: (issueId: string) =>
    request<AutoDevJob>('/api/auto-dev/start', {
      method: 'POST',
      body: JSON.stringify({ issueId }),
    }),

  cancelAutoDev: (jobId: string) =>
    request<{ ok: boolean }>(`/api/auto-dev/jobs/${jobId}/cancel`, { method: 'POST', body: '{}' }),

  getJob: (jobId: string) =>
    request<{ job: AutoDevJob; logs: AutoDevLog[] }>(`/api/auto-dev/jobs/${jobId}`),
};

export function subscribeJobEvents(
  jobId: string,
  onEvent: (ev: JobEvent) => void,
  onError?: (err: Error) => void
): () => void {
  const es = new EventSource(`/api/auto-dev/jobs/${jobId}/events`);
  es.onmessage = (msg) => {
    try {
      const ev = JSON.parse(msg.data) as JobEvent;
      onEvent(ev);
    } catch (e: any) {
      onError?.(e);
    }
  };
  es.onerror = () => {
    // EventSource retries; ignore transient errors
  };
  return () => es.close();
}
