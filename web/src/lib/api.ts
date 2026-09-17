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

type SpecStreamBody = {
  prompt?: string;
  messages?: unknown[];
  subRequirementId?: string;
  scope?: 'all' | 'sub';
  resumePartial?: string;
  freshStart?: boolean;
  resume?: boolean;
};

type SpecStreamResult = {
  spec?: DevSpec;
  reqDoc?: import('../types').ReqDoc;
  text: string;
  chatReply?: string;
  process?: string;
  subRequirements?: Issue['subRequirements'];
  docPhase?: import('../types').DocPhase;
  sourceFilesRead?: number;
};

async function streamSpec(
  url: string,
  body: SpecStreamBody,
  opts?: { signal?: AbortSignal; onDelta?: (chunk: string) => void; onStatus?: (msg: string) => void }
): Promise<SpecStreamResult> {
  const res = await fetch(url, {
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

  let result: SpecStreamResult | null = null;
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
        spec: ev.spec as DevSpec | undefined,
        reqDoc: (ev as { reqDoc?: import('../types').ReqDoc }).reqDoc,
        text: (ev.chatReply || ev.text || '') as string,
        chatReply: (ev.chatReply || ev.text || '') as string,
        process: (ev.process || '') as string,
        subRequirements: (ev.subRequirements as Issue['subRequirements']) || undefined,
        docPhase: (ev as { docPhase?: import('../types').DocPhase }).docPhase,
        sourceFilesRead: (ev as { sourceFilesRead?: number }).sourceFilesRead,
      };
    }
  });

  if (streamError) throw new Error(streamError);
  if (!result) throw new Error('Stream ended without result');
  return result;
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
  executor?: string;
  executorConfig?: import('../types').ExecutorConfig;
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
  subRequirementId?: string;
  subTitle?: string;
  subIndex?: number;
  subTotal?: number;
}

export const api = {
  health: () => request<HealthInfo>('/api/health'),

  getModel: () => request<ModelConfig>('/api/settings/model'),
  putModel: (config: ModelConfig) =>
    request<ModelConfig>('/api/settings/model', { method: 'PUT', body: JSON.stringify(config) }),

  getExecutor: () => request<import('../types').ExecutorConfig>('/api/settings/executor'),
  putExecutor: (config: import('../types').ExecutorConfig) =>
    request<import('../types').ExecutorConfig>('/api/settings/executor', {
      method: 'PUT',
      body: JSON.stringify(config),
    }),
  listExecutors: () =>
    request<{ executors: import('../types').ExecutorProbe[]; current: string }>('/api/executors'),
  getIssueDiff: (id: string) => request<import('../types').IssueDiff>(`/api/issues/${id}/diff`),
  rebaseIssue: (id: string) =>
    request<{ ok: boolean; baseBranch?: string; repos?: { repoId: string; ahead: number; behind: number; error?: string }[] }>(
      `/api/issues/${id}/rebase`,
      { method: 'POST', body: '{}' }
    ),
  publishRemote: (id: string) =>
    request<{ ok: boolean; pushed?: number; prUrl?: string; warnings?: string[]; error?: string; issue?: Issue }>(
      `/api/issues/${id}/publish-remote`,
      { method: 'POST', body: '{}' }
    ),
  openEditor: (id: string, app: 'cursor' | 'vscode', repoId?: string) =>
    request<{ ok: boolean; app: string; path: string }>(`/api/issues/${id}/open-editor`, {
      method: 'POST',
      body: JSON.stringify({ app, repoId }),
    }),

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
    request<{
      ok: boolean;
      path?: string;
      currentBranch?: string;
      defaultBranch?: string;
      filesCount?: number;
      hasCommits?: boolean;
      warning?: string;
      error?: string;
    }>('/api/repos/validate', { method: 'POST', body: JSON.stringify({ path }) }),

  generateSpec: (issueId: string, body: SpecStreamBody) =>
    request<SpecStreamResult>(`/api/issues/${issueId}/spec`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  /** Stream Dev Spec generation; onDelta receives raw model text as it arrives. */
  generateSpecStream: (
    issueId: string,
    body: SpecStreamBody,
    opts?: { signal?: AbortSignal; onDelta?: (chunk: string) => void; onStatus?: (msg: string) => void }
  ) => streamSpec(`/api/issues/${issueId}/spec?stream=1`, body, opts),

  generateReqDocStream: (
    issueId: string,
    body: SpecStreamBody,
    opts?: { signal?: AbortSignal; onDelta?: (chunk: string) => void; onStatus?: (msg: string) => void }
  ) => streamSpec(`/api/issues/${issueId}/req-doc?stream=1`, body, opts),

  acceptRequirement: (issueId: string) =>
    request<Issue>(`/api/issues/${issueId}/accept-requirement`, {
      method: 'POST',
      body: '{}',
    }),

  /** Convert issue brief (title/description/attachments) into a Markdown ReqDoc without LLM. */
  reqDocFromBrief: (issueId: string) =>
    request<Issue>(`/api/issues/${issueId}/req-doc/from-brief`, {
      method: 'POST',
      body: '{}',
    }),

  splitIssueStream: (
    issueId: string,
    body: SpecStreamBody,
    opts?: { signal?: AbortSignal; onDelta?: (chunk: string) => void; onStatus?: (msg: string) => void }
  ) => streamSpec(`/api/issues/${issueId}/split?stream=1`, body, opts),

  exportFile: (filename: string, contents: string) =>
    request<{ path: string }>('/api/export-file', {
      method: 'POST',
      body: JSON.stringify({ filename, contents }),
    }),

  startAutoDev: (issueId: string, subRequirementId?: string, executor?: import('../types').ExecutorConfig) =>
    request<AutoDevJob>('/api/auto-dev/start', {
      method: 'POST',
      body: JSON.stringify({
        issueId,
        subRequirementId: subRequirementId || undefined,
        executor: executor || undefined,
      }),
    }),

  cancelAutoDev: (jobId: string) =>
    request<{ ok: boolean; issue?: Issue }>(`/api/auto-dev/jobs/${jobId}/cancel`, {
      method: 'POST',
      body: '{}',
    }),

  /** Cancel by issue id — works after refresh / when in-memory job id is lost. */
  cancelAutoDevByIssue: (issueId: string) =>
    request<{ ok: boolean; issue: Issue }>(`/api/issues/${issueId}/cancel-auto-dev`, {
      method: 'POST',
      body: '{}',
    }),

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
