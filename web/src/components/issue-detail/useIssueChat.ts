import { useState, useRef } from 'react';
import { Issue, GitRepo, ModelConfig, ChatMessage } from '../../types';
import { Language, getTranslation } from '../../lib/i18n';
import { sendLLMChat } from '../../lib/llm';
import { api } from '../../lib/api';
import { promptDescription } from '../../lib/attachments';

export type ModelProcessState = {
  entries: { id: string; at: string; prompt: string; body: string; status: 'running' | 'done' | 'error' }[];
  expanded: boolean;
};

export function useIssueChat(params: {
  issue: Issue;
  splitIssue: boolean;
  selectedScope: string;
  setSelectedScope: (scope: string) => void;
  setSpecMarkdown: (md: string) => void;
  onUpdateIssue: (updatedIssue: Issue) => void;
  modelConfig: ModelConfig;
  projectId?: string;
  lang: Language;
  associatedRepos: GitRepo[];
}) {
  const {
    issue,
    splitIssue,
    selectedScope,
    setSelectedScope,
    setSpecMarkdown,
    onUpdateIssue,
    modelConfig,
    projectId,
    lang,
    associatedRepos,
  } = params;

  const t = getTranslation(lang);

  const [inputPrompt, setInputPrompt] = useState('');
  const [isSending, setIsSending] = useState(false);
  const [chatError, setChatError] = useState('');
  /** Ephemeral model process traces — session only, never persisted. */
  const [modelProcess, setModelProcess] = useState<ModelProcessState>({ entries: [], expanded: false });
  const abortRef = useRef<AbortController | null>(null);

  const handleSendMessage = async (
    customPrompt?: string,
    opts?: { forceSpecSync?: boolean; split?: boolean; scope?: string }
  ) => {
    const promptToUse = customPrompt || inputPrompt;
    if (!promptToUse.trim()) return;

    const scope = opts?.scope || selectedScope;
    const targetingSub = splitIssue && scope !== 'all';
    const targetSub = targetingSub
      ? (issue.subRequirements || []).find((s) => s.id === scope)
      : undefined;
    const priorMessages = targetSub ? targetSub.chatMessages || [] : issue.chatMessages;

    const userMsg: ChatMessage = {
      id: `msg-${Date.now()}`,
      sender: 'user',
      text: promptToUse,
      timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    };

    const updatedMessages = [...priorMessages, userMsg];
    const tempIssue = targetSub
      ? {
          ...issue,
          subRequirements: (issue.subRequirements || []).map((s) =>
            s.id === targetSub.id ? { ...s, chatMessages: updatedMessages } : s
          ),
        }
      : { ...issue, chatMessages: updatedMessages };
    onUpdateIssue(tempIssue);
    setInputPrompt('');
    setIsSending(true);
    setChatError('');
    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;

    const syncSpec =
      opts?.forceSpecSync ||
      opts?.split ||
      issue.status === 'requirements' ||
      issue.status === 'backlog';

    const processId = `proc-${Date.now()}`;
    const processAt = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    setModelProcess((prev) => ({
      expanded: true,
      entries: [
        ...prev.entries.slice(-7),
        {
          id: processId,
          at: processAt,
          prompt: promptToUse.length > 120 ? `${promptToUse.slice(0, 120)}…` : promptToUse,
          body: lang === 'zh' ? '正在请求模型…' : 'Calling model…',
          status: 'running',
        },
      ],
    }));

    const patchProcess = (body: string, status?: 'running' | 'done' | 'error') => {
      setModelProcess((prev) => ({
        ...prev,
        entries: prev.entries.map((e) =>
          e.id === processId ? { ...e, body, ...(status ? { status } : {}) } : e
        ),
      }));
    };

    let streamed = '';
    const appendDelta = (chunk: string) => {
      streamed += chunk;
      patchProcess(streamed, 'running');
    };

    const mergeChat = (base: Issue, messages: ChatMessage[]): Issue => {
      if (targetSub) {
        return {
          ...base,
          subRequirements: (base.subRequirements || []).map((s) =>
            s.id === targetSub.id ? { ...s, chatMessages: messages } : s
          ),
        };
      }
      return { ...base, chatMessages: messages };
    };

    try {
      if (syncSpec) {
        const streamOpts = {
          signal: ac.signal,
          onDelta: appendDelta,
          onStatus: () => {
            if (!streamed) {
              patchProcess(lang === 'zh' ? '正在请求模型…' : 'Calling model…', 'running');
            }
          },
        };
        const result = opts?.split
          ? await api.splitIssueStream(issue.id, { prompt: promptToUse, messages: updatedMessages }, streamOpts)
          : await api.generateSpecStream(
              issue.id,
              {
                prompt: promptToUse,
                messages: updatedMessages,
                scope: targetingSub ? 'sub' : splitIssue ? 'all' : undefined,
                subRequirementId: targetingSub ? scope : undefined,
              },
              streamOpts
            );
        const { spec, text, chatReply, process, subRequirements } = result;
        const reply =
          chatReply ||
          text ||
          (lang === 'zh' ? '已更新待开发文档。' : 'Dev Spec updated.');
        patchProcess(process || streamed || text || reply, 'done');
        const aiMsg: ChatMessage = {
          id: `msg-${Date.now() + 1}`,
          sender: 'ai',
          text: reply,
          timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
        };
        const nextSubs = subRequirements || tempIssue.subRequirements;
        onUpdateIssue({
          ...mergeChat(tempIssue, [...updatedMessages, aiMsg]),
          devSpec: spec || tempIssue.devSpec,
          subRequirements: nextSubs,
          updatedAt: new Date().toISOString(),
        });
        if (spec?.rawMarkdown) setSpecMarkdown(spec.rawMarkdown);
        if (opts?.split) setSelectedScope('all');
      } else {
        const responseText = await sendLLMChat({
          prompt: promptToUse,
          messages: updatedMessages,
          modelConfig,
          issueTitle: issue.title,
          issueDescription: promptDescription(issue),
          associatedRepos: associatedRepos.map((r) => ({
            name: r.name,
            path: r.path,
            defaultBranch: r.defaultBranch,
          })),
          generateSpec: false,
          projectId,
          issueId: issue.id,
          signal: ac.signal,
          onDelta: appendDelta,
        });

        patchProcess(streamed || responseText, 'done');
        const aiMsg: ChatMessage = {
          id: `msg-${Date.now() + 1}`,
          sender: 'ai',
          text: responseText,
          timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
        };
        onUpdateIssue({
          ...mergeChat(tempIssue, [...updatedMessages, aiMsg]),
          updatedAt: new Date().toISOString(),
        });
      }
    } catch (err: any) {
      if (err?.name === 'AbortError') {
        setChatError(t.requestCancelled);
        patchProcess(t.requestCancelled, 'error');
      } else {
        setChatError(err.message || t.llmError);
        patchProcess(err.message || t.llmError, 'error');
        const errMsg: ChatMessage = {
          id: `msg-${Date.now() + 1}`,
          sender: 'system',
          text: `⚠️ ${t.llmError}: ${err.message}`,
          timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
        };
        onUpdateIssue(mergeChat(tempIssue, [...updatedMessages, errMsg]));
      }
    } finally {
      setIsSending(false);
    }
  };

  return {
    inputPrompt,
    setInputPrompt,
    isSending,
    chatError,
    setChatError,
    modelProcess,
    setModelProcess,
    abortRef,
    handleSendMessage,
  };
}
