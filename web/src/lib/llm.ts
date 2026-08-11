import { ChatMessage, ModelConfig } from '../types';
import { readSSE } from './stream';

export interface RepoRef {
  name: string;
  path: string;
  defaultBranch: string;
}

export interface ChatRequestParams {
  prompt: string;
  messages: ChatMessage[];
  modelConfig?: ModelConfig;
  issueTitle?: string;
  issueDescription?: string;
  associatedRepos?: RepoRef[];
  generateSpec?: boolean;
  projectId?: string;
  issueId?: string;
  signal?: AbortSignal;
  onDelta?: (chunk: string) => void;
}

export async function sendLLMChat(params: ChatRequestParams): Promise<string> {
  const res = await fetch('/api/chat?stream=1', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream',
    },
    body: JSON.stringify({
      prompt: params.prompt,
      messages: params.messages,
      issueTitle: params.issueTitle,
      issueDescription: params.issueDescription,
      associatedRepos: params.associatedRepos || [],
      generateSpec: params.generateSpec,
      projectId: params.projectId,
      issueId: params.issueId,
    }),
    signal: params.signal,
  });

  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(
      (data as { error?: string }).error || `HTTP ${res.status}: Failed to receive response from AI model.`
    );
  }

  let text = '';
  let streamError: string | null = null;

  await readSSE(res, (ev) => {
    if (ev.type === 'delta' && ev.text) {
      text += ev.text;
      params.onDelta?.(ev.text);
    } else if (ev.type === 'error') {
      streamError = ev.error || 'stream error';
    } else if (ev.type === 'done' && typeof ev.text === 'string') {
      text = ev.text;
    }
  });

  if (streamError) throw new Error(streamError);
  if (!text) throw new Error('Empty model response');
  return text;
}

export async function testOpenAPIConnection(config: {
  openAIBaseUrl: string;
  openAIApiKey: string;
  openAIModel: string;
  /** When key is blank, server uses this project's stored custom key (if any). */
  projectId?: string;
}): Promise<{ success: boolean; message?: string; error?: string }> {
  try {
    const res = await fetch('/api/test-openapi', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(config),
    });
    const data = await res.json();
    if (!res.ok || !data.success) {
      return { success: false, error: data.error || `HTTP ${res.status}: OpenAPI connection failed.` };
    }
    return { success: true, message: data.message || 'Connection successful!' };
  } catch (err: any) {
    return { success: false, error: err.message || 'Network error connecting to backend.' };
  }
}
