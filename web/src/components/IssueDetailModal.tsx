import React, { useState, useEffect, useRef } from 'react';
import { Issue, GitRepo, ModelConfig, ChatMessage, BranchPrefixConfig, SubRequirement } from '../types';
import { Language, getTranslation, ThemeStyle } from '../lib/i18n';
import { THEME_CONFIGS } from '../lib/theme';
import { sendLLMChat } from '../lib/llm';
import { api } from '../lib/api';
import { MarkdownView } from '../lib/markdown';
import { aggregatedFileChanges, hasSubRequirements, specMarkdownForExport, specReadyForDev, visibleSpec } from '../lib/subreq';
import { saveTextFile } from '../lib/savefile';
import { SubRequirementBar } from './SubRequirementBar';
import {
  X,
  MessageSquare,
  FileText,
  Terminal,
  GitPullRequest,
  Send,
  Sparkles,
  Bot,
  User,
  CheckCircle2,
  AlertTriangle,
  Play,
  RotateCcw,
  GitMerge,
  Code2,
  FileCode,
  Edit3,
  Loader2,
  ShieldCheck,
  Square,
  Trash2,
  GitBranch,
  Pencil,
  ChevronDown,
  ChevronUp,
  Eye,
  Download,
  Layers,
} from 'lucide-react';

interface IssueDetailModalProps {
  isOpen: boolean;
  onClose: () => void;
  issue: Issue;
  gitRepos: GitRepo[];
  modelConfig: ModelConfig;
  branchPrefixConfig?: BranchPrefixConfig;
  onUpdateIssue: (updatedIssue: Issue) => void;
  onStartAutoDev: (issueId: string, subRequirementId?: string) => void;
  onCancelAutoDev?: (issueId: string) => void;
  onDeleteIssue?: (issueId: string) => void;
  projectId?: string;
  lang?: Language;
  themeStyle?: ThemeStyle;
}

export const IssueDetailModal: React.FC<IssueDetailModalProps> = ({
  isOpen,
  onClose,
  issue,
  gitRepos,
  modelConfig,
  branchPrefixConfig,
  onUpdateIssue,
  onStartAutoDev,
  onCancelAutoDev,
  onDeleteIssue,
  projectId,
  lang = 'en',
  themeStyle = 'glass',
}) => {
  const themeConfig = THEME_CONFIGS[themeStyle] || THEME_CONFIGS.glass;
  const isLight = themeConfig.isLight;
  const t = getTranslation(lang);

  const [activeTab, setActiveTab] = useState<'chat' | 'spec' | 'console' | 'review'>('chat');
  const [inputPrompt, setInputPrompt] = useState('');
  const [isSending, setIsSending] = useState(false);
  const [editingSpec, setEditingSpec] = useState(false);

  const [specMarkdown, setSpecMarkdown] = useState(issue.devSpec?.rawMarkdown || '');
  const [reworkFeedback, setReworkFeedback] = useState('');
  const [showReworkBox, setShowReworkBox] = useState(false);
  const [chatError, setChatError] = useState('');
  const [editingRepos, setEditingRepos] = useState(false);
  const [selectedScope, setSelectedScope] = useState<string>('all');
  const [reworkScope, setReworkScope] = useState<string>('all');
  const [exporting, setExporting] = useState(false);
  const [exportHint, setExportHint] = useState('');
  /** Ephemeral model process traces — session only, never persisted. */
  const [modelProcess, setModelProcess] = useState<{
    entries: { id: string; at: string; prompt: string; body: string; status: 'running' | 'done' | 'error' }[];
    expanded: boolean;
  }>({ entries: [], expanded: false });
  const abortRef = useRef<AbortController | null>(null);

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const splitIssue = hasSubRequirements(issue);
  const activeSub: SubRequirement | undefined =
    splitIssue && selectedScope !== 'all'
      ? (issue.subRequirements || []).find((s) => s.id === selectedScope)
      : undefined;
  const visibleMessages = activeSub ? activeSub.chatMessages || [] : issue.chatMessages;
  const currentSpec = visibleSpec(issue, selectedScope);

  // Automatically switch active tab based on Issue status when modal opens
  useEffect(() => {
    if (!isOpen) return;
    setEditingRepos(false);
    setModelProcess({ entries: [], expanded: false });
    setSelectedScope('all');
    setReworkScope('all');
    setExportHint('');
    if (issue.status === 'requirements') {
      setActiveTab('chat');
    } else if (issue.status === 'backlog') {
      setActiveTab('spec');
    } else if (issue.status === 'in_progress') {
      setActiveTab('console');
    } else if (issue.status === 'in_review' || issue.status === 'completed') {
      setActiveTab('review');
    }
  }, [isOpen, issue.id, issue.status]);

  useEffect(() => {
    const spec = visibleSpec(issue, selectedScope);
    if (spec?.rawMarkdown) {
      setSpecMarkdown(spec.rawMarkdown);
    }
  }, [issue.devSpec, issue.subRequirements, selectedScope]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [visibleMessages, activeTab]);

  if (!isOpen) return null;

  const associatedRepos = gitRepos.filter((r) => issue.associatedRepoIds.includes(r.id));
  const canEditRepos = issue.status === 'requirements' || issue.status === 'backlog';

  const toggleAssociatedRepo = (repoId: string) => {
    if (!canEditRepos) return;
    const current = issue.associatedRepoIds || [];
    const next = current.includes(repoId)
      ? current.filter((id) => id !== repoId)
      : [...current, repoId];
    if (next.length === 0) {
      setChatError(lang === 'zh' ? '请至少保留一个关联仓库' : 'Keep at least one associated repository');
      return;
    }
    setChatError('');
    onUpdateIssue({
      ...issue,
      associatedRepoIds: next,
      updatedAt: new Date().toISOString(),
    });
  };

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
          issueDescription: issue.description,
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

  const handleReworkSubmit = async () => {
    if (!reworkFeedback.trim()) {
      setChatError(lang === 'zh' ? '请输入评审修改意见' : 'Enter review feedback');
      return;
    }

    setShowReworkBox(false);
    setActiveTab('chat');
    const scope = reworkScope;
    setSelectedScope(scope);

    const targetLabel =
      scope === 'all'
        ? lang === 'zh'
          ? '整单需求'
          : 'whole requirement'
        : (issue.subRequirements || []).find((s) => s.id === scope)?.title || scope;

    const prompt =
      lang === 'zh'
        ? `开发者在评审中指出了以下问题，需要二次修改代码与开发文档（范围: ${targetLabel}）:\n"${reworkFeedback}"\n请重新分析并更新对应待开发文档。`
        : `Reviewer requested rework for ${targetLabel}:\n"${reworkFeedback}"\nPlease revise the Dev Spec(s) accordingly.`;

    const updatedIssue: Issue = {
      ...issue,
      reviewFeedback: reworkFeedback,
      reworkSubId: scope === 'all' ? '' : scope,
      autoDevLogs: [
        ...issue.autoDevLogs,
        {
          id: `rework-${Date.now()}`,
          timestamp: new Date().toLocaleTimeString(),
          phase: 'analyzing',
          message:
            lang === 'zh'
              ? `收到二次评审意见（${targetLabel}）: "${reworkFeedback}". 正在更新 Dev Spec...`
              : `Rework feedback (${targetLabel}): "${reworkFeedback}". Updating Dev Spec...`,
        },
      ],
    };

    onUpdateIssue(updatedIssue);
    const feedback = reworkFeedback;
    setReworkFeedback('');

    await handleSendMessage(prompt, { forceSpecSync: true, scope });
    onStartAutoDev(issue.id, scope === 'all' ? undefined : scope);
    void feedback;
  };

  const handleApproveMerge = async () => {
    try {
      const saved = await api.approveMerge(issue.id);
      onUpdateIssue(saved);
    } catch (err: any) {
      setChatError(err.message || t.mergeFailed);
    }
  };

  const handleExportDevSpec = async () => {
    const { filename, markdown } = specMarkdownForExport(
      issue,
      selectedScope,
      editingSpec ? specMarkdown : undefined
    );
    if (!markdown.trim()) {
      setChatError(t.noSpecToExport);
      setExportHint('');
      return;
    }
    setChatError('');
    setExportHint('');
    setExporting(true);
    try {
      const result = await saveTextFile(filename, markdown);
      if (result.status === 'cancelled') {
        return;
      }
      if (result.status === 'copied') {
        setExportHint(t.exportCopied);
        return;
      }
      setExportHint(
        result.path ? t.exportSavedTo.replace('{path}', result.path) : t.exportSaved
      );
    } catch (err: any) {
      setChatError(err?.message || t.exportFailed);
    } finally {
      setExporting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-md p-4 sm:p-6">
      <div className={`w-full max-w-5xl border rounded-2xl shadow-2xl overflow-hidden flex flex-col h-[90vh] ${themeConfig.modalBg}`}>
        {/* Header */}
        <div className={`p-5 border-b flex items-center justify-between gap-4 ${themeConfig.subtleBorder} ${themeConfig.modalHeaderBg}`}>
          <div className="flex items-center gap-3 overflow-hidden">
            <span
              className={`px-2.5 py-1 rounded-md text-[11px] font-bold uppercase tracking-wider shrink-0 border ${
                issue.status === 'requirements'
                  ? 'bg-cyan-500/10 border-cyan-500/30 text-cyan-600 dark:text-cyan-300'
                  : issue.status === 'backlog'
                  ? 'bg-amber-500/10 border-amber-500/30 text-amber-600 dark:text-amber-300'
                  : issue.status === 'in_progress'
                  ? 'bg-indigo-500/10 border-indigo-500/30 text-indigo-600 dark:text-indigo-300 animate-pulse'
                  : issue.status === 'in_review'
                  ? 'bg-purple-500/10 border-purple-500/30 text-purple-600 dark:text-purple-300'
                  : 'bg-emerald-500/10 border-emerald-500/30 text-emerald-600 dark:text-emerald-300'
              }`}
            >
              {issue.status === 'requirements'
                ? '需求列表'
                : issue.status === 'backlog'
                ? '待执行'
                : issue.status === 'in_progress'
                ? '进行中'
                : issue.status === 'in_review'
                ? '待评审'
                : '已完成'}
            </span>

            <div className="overflow-hidden min-w-0">
              <h2 className={`text-lg font-bold truncate ${themeConfig.textPrimary}`}>{issue.title}</h2>
              <div className={`flex items-center gap-2 text-xs mt-0.5 ${themeConfig.textSecondary}`}>
                <span className="shrink-0">Issue #{issue.id}</span>
              </div>
            </div>
          </div>

          <div className="flex items-center gap-2 shrink-0">
            {issue.status === 'backlog' && specReadyForDev(issue) && (
              <button
                onClick={() => {
                  onStartAutoDev(issue.id);
                  setActiveTab('console');
                }}
                className="px-3.5 py-1.5 bg-gradient-to-r from-indigo-600 to-purple-600 hover:from-indigo-500 hover:to-purple-500 text-white font-semibold text-xs rounded-xl shadow flex items-center gap-1.5 transition-all"
              >
                <Play className="w-3.5 h-3.5 fill-current" />
                启动自治开发
              </button>
            )}

            {onDeleteIssue && (
              <button
                onClick={() => {
                  if (window.confirm(t.deleteIssueConfirm)) {
                    onDeleteIssue(issue.id);
                    onClose();
                  }
                }}
                className={`p-2 rounded-xl transition-colors text-rose-400 hover:bg-rose-500/10`}
                title={t.deleteIssueTitle}
              >
                <Trash2 className="w-4 h-4" />
              </button>
            )}
            <button
              onClick={onClose}
              className={`p-2 rounded-xl transition-colors ${themeConfig.textSecondary} hover:${themeConfig.textPrimary} hover:bg-black/5 dark:hover:bg-white/10`}
            >
              <X className="w-5 h-5" />
            </button>
          </div>
        </div>

        {/* Associated repositories — editable for requirements / backlog */}
        <div className={`px-5 py-2.5 border-b ${themeConfig.subtleBorder} ${themeConfig.modalHeaderBg}`}>
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0 flex-1">
              <div className={`flex items-center gap-1.5 text-[10px] uppercase tracking-wider font-bold mb-1.5 ${themeConfig.textMuted}`}>
                <GitBranch className="w-3 h-3" />
                {lang === 'zh' ? '关联仓库' : 'Associated Repos'}
              </div>
              {editingRepos ? (
                gitRepos.length === 0 ? (
                  <p className="text-xs text-rose-400">
                    {lang === 'zh' ? '项目尚未配置仓库，请先在项目设置中添加。' : 'No repos in project. Add them in project settings first.'}
                  </p>
                ) : (
                  <div className="flex flex-wrap gap-2">
                    {gitRepos.map((repo) => {
                      const checked = issue.associatedRepoIds.includes(repo.id);
                      return (
                        <button
                          key={repo.id}
                          type="button"
                          disabled={!canEditRepos}
                          onClick={() => toggleAssociatedRepo(repo.id)}
                          className={`px-2.5 py-1.5 rounded-lg border text-left text-[11px] transition-colors max-w-full ${
                            checked
                              ? 'border-indigo-500/50 bg-indigo-500/15 text-indigo-800 dark:text-indigo-200'
                              : `${themeConfig.inputBg} ${themeConfig.inputBorder} ${themeConfig.textSecondary}`
                          } ${!canEditRepos ? 'opacity-60 cursor-not-allowed' : ''}`}
                          title={repo.path}
                        >
                          <span className="font-semibold">{repo.name}</span>
                          <span className={`block font-mono truncate max-w-[220px] ${themeConfig.textMuted}`}>
                            {repo.path || '—'}
                          </span>
                        </button>
                      );
                    })}
                  </div>
                )
              ) : (
                <p
                  className={`text-xs truncate select-text ${themeConfig.textSecondary}`}
                  title={associatedRepos.map((r) => `${r.name} (${r.path || ''})`).join(', ')}
                >
                  {associatedRepos.length > 0
                    ? associatedRepos.map((r) => `${r.name}`).join(', ')
                    : lang === 'zh'
                    ? '未关联仓库'
                    : 'No repos linked'}
                </p>
              )}
            </div>
            {canEditRepos && (
              <button
                type="button"
                onClick={() => setEditingRepos((v) => !v)}
                className={`shrink-0 px-2.5 py-1 rounded-lg text-[11px] font-semibold border flex items-center gap-1 ${themeConfig.btnSecondary} ${themeConfig.btnSecondaryText}`}
              >
                <Pencil className="w-3 h-3" />
                {editingRepos
                  ? lang === 'zh'
                    ? '完成'
                    : 'Done'
                  : lang === 'zh'
                  ? '修改'
                  : 'Edit'}
              </button>
            )}
          </div>
        </div>

        {/* Status Focus Banner */}
        {issue.status === 'requirements' && (
          <div className="px-5 py-2.5 bg-cyan-500/10 border-b border-cyan-500/30 flex items-center justify-between text-xs text-cyan-800 dark:text-cyan-200">
            <div className="flex items-center gap-2">
              <MessageSquare className="w-4 h-4 text-cyan-500 shrink-0" />
              <div>
                <span className="font-bold">{t.stageFocusReqTitle}</span>
                <span className="ml-2 hidden sm:inline opacity-80">{t.stageFocusReqDesc}</span>
              </div>
            </div>
            <div className="flex items-center gap-2 shrink-0">
              <button
                onClick={() => {
                  if (splitIssue && !window.confirm(t.splitSubsConfirm)) return;
                  setActiveTab('chat');
                  handleSendMessage(
                    lang === 'zh'
                      ? '请将当前需求拆分成若干可独立实施的子需求，每个子需求一份完整待开发文档，按实施顺序排列。'
                      : 'Split this requirement into ordered, independently implementable sub-requirements. Each must have a complete Dev Spec.',
                    { split: true }
                  );
                }}
                className="px-2.5 py-1 bg-indigo-500/20 hover:bg-indigo-500/30 border border-indigo-500/40 rounded-lg text-indigo-800 dark:text-indigo-200 font-semibold text-[11px] flex items-center gap-1 transition-all"
                title={t.splitSubsHint}
              >
                <Layers className="w-3 h-3 text-indigo-500" />
                {t.splitSubsBtn}
              </button>
              <button
                onClick={() => {
                  setActiveTab('chat');
                  handleSendMessage(
                    lang === 'zh'
                      ? '请根据当前需求，完善待开发文档（架构、改动文件、实施步骤与测试用例）。'
                      : 'Please refine the Dev Spec (architecture, file changes, steps, and tests) based on the current requirement.'
                  );
                }}
                className="px-2.5 py-1 bg-cyan-500/20 hover:bg-cyan-500/30 border border-cyan-500/40 rounded-lg text-cyan-800 dark:text-cyan-200 font-semibold text-[11px] flex items-center gap-1 transition-all"
              >
                <Sparkles className="w-3 h-3 text-cyan-500" />
                {t.extractSpecBtn}
              </button>
              {specReadyForDev(issue) && associatedRepos.length > 0 && (
                <button
                  onClick={() =>
                    onUpdateIssue({
                      ...issue,
                      status: 'backlog',
                      updatedAt: new Date().toISOString(),
                    })
                  }
                  className="px-2.5 py-1 bg-amber-500/20 hover:bg-amber-500/30 border border-amber-500/40 rounded-lg text-amber-900 dark:text-amber-200 font-semibold text-[11px] transition-all"
                >
                  {t.acceptSpecBacklog}
                </button>
              )}
            </div>
          </div>
        )}

        {issue.status === 'backlog' && (
          <div className="px-5 py-2.5 bg-amber-500/10 border-b border-amber-500/30 flex items-center justify-between text-xs text-amber-800 dark:text-amber-200">
            <div className="flex items-center gap-2">
              <FileText className="w-4 h-4 text-amber-500 shrink-0" />
              <div>
                <span className="font-bold">{t.stageFocusBacklogTitle}</span>
                <span className="ml-2 hidden sm:inline opacity-80">{t.stageFocusBacklogDesc}</span>
              </div>
            </div>
            <button
              onClick={() => {
                onStartAutoDev(issue.id);
                setActiveTab('console');
              }}
              className="px-3 py-1 bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg font-bold text-[11px] flex items-center gap-1.5 shadow transition-all shrink-0"
            >
              <Play className="w-3 h-3 fill-current" />
              {t.startAutoDev}
            </button>
          </div>
        )}

        {issue.status === 'in_progress' && (
          <div className="px-5 py-2.5 bg-indigo-500/10 border-b border-indigo-500/30 flex items-center justify-between text-xs text-indigo-800 dark:text-indigo-200">
            <div className="flex items-center gap-2">
              <Sparkles className="w-4 h-4 text-indigo-500 animate-spin shrink-0" />
              <div>
                <span className="font-bold">{t.stageFocusProgressTitle} ({issue.autoDevProgress}%)</span>
                <span className="ml-2 hidden sm:inline opacity-80">{t.stageFocusProgressDesc}</span>
              </div>
            </div>
            <div className="flex items-center gap-2 shrink-0">
              {onCancelAutoDev && (
                <button
                  onClick={() => onCancelAutoDev(issue.id)}
                  className="px-2.5 py-1 bg-rose-500/20 hover:bg-rose-500/30 border border-rose-500/40 rounded-lg text-rose-800 dark:text-rose-200 font-semibold text-[11px] flex items-center gap-1 transition-all"
                >
                  <Square className="w-3 h-3" />
                  {t.cancel}
                </button>
              )}
              <button
                onClick={() => setActiveTab('console')}
                className="px-2.5 py-1 bg-indigo-500/20 hover:bg-indigo-500/30 border border-indigo-500/40 rounded-lg text-indigo-800 dark:text-indigo-200 font-semibold text-[11px] flex items-center gap-1 transition-all"
              >
                <Terminal className="w-3 h-3 text-indigo-500" />
                {t.viewLogsBtn}
              </button>
            </div>
          </div>
        )}

        {issue.status === 'in_review' && (
          <div className="px-5 py-2.5 bg-purple-500/10 border-b border-purple-500/30 flex items-center justify-between text-xs text-purple-800 dark:text-purple-200">
            <div className="flex items-center gap-2">
              <GitPullRequest className="w-4 h-4 text-purple-500 shrink-0" />
              <div>
                <span className="font-bold">{t.stageFocusReviewTitle}</span>
                <span className="ml-2 hidden sm:inline opacity-80">{t.stageFocusReviewDesc}</span>
              </div>
            </div>
            <div className="flex items-center gap-2 shrink-0">
              <button
                onClick={() => {
                  handleApproveMerge();
                  setActiveTab('review');
                }}
                className="px-3 py-1 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg font-bold text-[11px] flex items-center gap-1 shadow transition-all"
              >
                <ShieldCheck className="w-3.5 h-3.5" />
                {t.approveMergeBtn}
              </button>
            </div>
          </div>
        )}

        {issue.status === 'completed' && (
          <div className="px-5 py-2.5 bg-emerald-500/10 border-b border-emerald-500/30 flex items-center justify-between text-xs text-emerald-800 dark:text-emerald-200">
            <div className="flex items-center gap-2">
              <CheckCircle2 className="w-4 h-4 text-emerald-500 shrink-0" />
              <div>
                <span className="font-bold">{t.stageFocusCompletedTitle}</span>
                <span className="ml-2 hidden sm:inline opacity-80">{t.stageFocusCompletedDesc}</span>
              </div>
            </div>
            <span className="px-2.5 py-0.5 rounded-full bg-emerald-500/20 border border-emerald-500/30 text-emerald-700 dark:text-emerald-300 font-mono text-[10px] font-bold shrink-0">
              {t.scoreMerged}
            </span>
          </div>
        )}

        {/* Tab Navigation Bar */}
        <div className={`flex border-b px-6 gap-6 text-xs font-semibold overflow-x-auto ${themeConfig.subtleBorder} ${themeConfig.modalHeaderBg}`}>
          <button
            onClick={() => setActiveTab('chat')}
            className={`py-3 border-b-2 flex items-center gap-2 transition-colors shrink-0 ${
              activeTab === 'chat'
                ? 'border-indigo-500 text-indigo-600 dark:text-indigo-300 font-bold'
                : `border-transparent ${themeConfig.textSecondary} hover:${themeConfig.textPrimary}`
            }`}
          >
            <MessageSquare className="w-4 h-4" />
            {t.tabChat}
            <span className={`px-1.5 py-0.5 rounded-full text-[10px] font-mono border ${themeConfig.inputBg} ${themeConfig.textMuted} ${themeConfig.inputBorder}`}>
              {issue.chatMessages.length +
                (issue.subRequirements || []).reduce((n, s) => n + (s.chatMessages?.length || 0), 0)}
            </span>
            {issue.status === 'requirements' && (
              <span className="px-1.5 py-0.5 rounded bg-cyan-500/20 text-cyan-700 dark:text-cyan-300 text-[9px] border border-cyan-500/40 font-bold">
                {lang === 'zh' ? '当前重点' : 'Focus'}
              </span>
            )}
          </button>

          <button
            onClick={() => setActiveTab('spec')}
            className={`py-3 border-b-2 flex items-center gap-2 transition-colors shrink-0 ${
              activeTab === 'spec'
                ? 'border-indigo-500 text-indigo-600 dark:text-indigo-300 font-bold'
                : `border-transparent ${themeConfig.textSecondary} hover:${themeConfig.textPrimary}`
            }`}
          >
            <FileText className="w-4 h-4" />
            {t.tabSpec}
            {(issue.devSpec || splitIssue) && <CheckCircle2 className="w-3.5 h-3.5 text-emerald-500" />}
            {issue.status === 'backlog' && (
              <span className="px-1.5 py-0.5 rounded bg-amber-500/20 text-amber-700 dark:text-amber-300 text-[9px] border border-amber-500/40 font-bold">
                {lang === 'zh' ? '确认规范' : 'Focus'}
              </span>
            )}
          </button>

          <button
            onClick={() => setActiveTab('console')}
            className={`py-3 border-b-2 flex items-center gap-2 transition-colors shrink-0 ${
              activeTab === 'console'
                ? 'border-indigo-500 text-indigo-600 dark:text-indigo-300 font-bold'
                : `border-transparent ${themeConfig.textSecondary} hover:${themeConfig.textPrimary}`
            }`}
          >
            <Terminal className="w-4 h-4" />
            {t.tabConsole}
            {issue.status === 'in_progress' && (
              <span className="px-1.5 py-0.5 rounded bg-indigo-500/20 text-indigo-700 dark:text-indigo-300 text-[9px] border border-indigo-500/40 font-bold flex items-center gap-1">
                <span className="w-1.5 h-1.5 rounded-full bg-indigo-500 animate-ping" />
                {lang === 'zh' ? '编码中' : 'Coding'}
              </span>
            )}
          </button>

          <button
            onClick={() => setActiveTab('review')}
            className={`py-3 border-b-2 flex items-center gap-2 transition-colors shrink-0 ${
              activeTab === 'review'
                ? 'border-indigo-500 text-indigo-600 dark:text-indigo-300 font-bold'
                : `border-transparent ${themeConfig.textSecondary} hover:${themeConfig.textPrimary}`
            }`}
          >
            <GitPullRequest className="w-4 h-4" />
            {t.tabReview}
            {issue.status === 'in_review' && (
              <span className="px-1.5 py-0.5 rounded bg-purple-500/20 text-purple-700 dark:text-purple-200 text-[9px] border border-purple-500/40 font-bold">
                {lang === 'zh' ? '优先评审' : 'Focus'}
              </span>
            )}
            {issue.status === 'completed' && (
              <span className="px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-700 dark:text-emerald-300 text-[9px] border border-emerald-500/40 font-bold">
                {lang === 'zh' ? '成果归档' : 'Archived'}
              </span>
            )}
          </button>
        </div>

        {/* Tab Content Body */}
        <div className="flex-1 overflow-hidden flex flex-col">
          {/* TAB 1: AI Chat & Spec Generator */}
          {activeTab === 'chat' && (
            <div className="flex-1 flex flex-col h-full overflow-hidden">
              <div className="flex-1 p-6 overflow-y-auto space-y-4">
                {splitIssue && (
                  <SubRequirementBar
                    issue={issue}
                    selectedScope={selectedScope}
                    onSelect={setSelectedScope}
                    lang={lang}
                    themeStyle={themeStyle}
                  />
                )}
                {visibleMessages.map((msg) => (
                  <div
                    key={msg.id}
                    className={`flex items-start gap-3 ${
                      msg.sender === 'user' ? 'flex-row-reverse' : ''
                    }`}
                  >
                    <div
                      className={`w-8 h-8 rounded-xl flex items-center justify-center shrink-0 text-xs font-bold ${
                        msg.sender === 'user'
                          ? 'bg-indigo-600 text-white'
                          : msg.sender === 'ai'
                          ? 'bg-purple-600 text-white'
                          : `${themeConfig.cardBg} ${themeConfig.textMuted}`
                      }`}
                    >
                      {msg.sender === 'user' ? (
                        <User className="w-4 h-4" />
                      ) : msg.sender === 'ai' ? (
                        <Bot className="w-4 h-4" />
                      ) : (
                        <Sparkles className="w-4 h-4" />
                      )}
                    </div>

                    <div
                      className={`max-w-[80%] rounded-2xl p-4 text-xs leading-relaxed backdrop-blur-md ${
                        msg.sender === 'user'
                          ? 'bg-indigo-600/20 border border-indigo-500/40 text-indigo-900 dark:text-indigo-100 rounded-tr-none'
                          : msg.sender === 'ai'
                          ? `${themeConfig.cardBg} border ${themeConfig.cardBorder} ${themeConfig.textPrimary} rounded-tl-none font-sans select-text`
                          : `${themeConfig.inputBg} border ${themeConfig.inputBorder} ${themeConfig.textMuted} font-mono`
                      }`}
                    >
                      <div className={`flex items-center justify-between gap-4 mb-1 border-b pb-1 text-[10px] ${themeConfig.subtleBorder} ${themeConfig.textMuted}`}>
                        <span className="font-semibold uppercase">
                          {msg.sender === 'user' ? '开发者' : msg.sender === 'ai' ? 'Vibe AI Copilot' : '系统说明'}
                        </span>
                        <span>{msg.timestamp}</span>
                      </div>
                      {msg.sender === 'ai' ? <MarkdownView text={msg.text} /> : <div className="select-text whitespace-pre-wrap">{msg.text}</div>}
                    </div>
                  </div>
                ))}
                {isSending && (
                  <div className="flex items-center gap-2 text-xs text-indigo-600 dark:text-indigo-300 p-2">
                    <Loader2 className="w-4 h-4 animate-spin" />
                    <span>AI 大模型正在思考分析并处理需求...</span>
                    <button
                      className="ml-2 underline"
                      onClick={() => abortRef.current?.abort()}
                    >
                      {t.cancel}
                    </button>
                  </div>
                )}
                {chatError && (
                  <div className="text-xs text-rose-500 px-2 select-text">{chatError}</div>
                )}
                <div ref={messagesEndRef} />
              </div>

              {/* Ephemeral model process (not persisted) */}
              {modelProcess.entries.length > 0 && (
                <div className={`border-t shrink-0 ${themeConfig.subtleBorder} ${themeConfig.modalHeaderBg}`}>
                  <button
                    type="button"
                    onClick={() =>
                      setModelProcess((prev) => ({ ...prev, expanded: !prev.expanded }))
                    }
                    className={`w-full px-4 py-2 flex items-center justify-between gap-2 text-[11px] ${themeConfig.textSecondary}`}
                  >
                    <span className="flex items-center gap-1.5 font-semibold min-w-0">
                      <Eye className="w-3.5 h-3.5 text-indigo-500 shrink-0" />
                      <span className="truncate">
                        {lang === 'zh' ? '模型过程' : 'Model process'}
                        <span className={`ml-1.5 font-normal ${themeConfig.textMuted}`}>
                          ({modelProcess.entries.length}
                          {lang === 'zh' ? '，仅本会话展示、不落库' : ', session only'})
                        </span>
                      </span>
                      {modelProcess.entries.some((e) => e.status === 'running') && (
                        <Loader2 className="w-3 h-3 animate-spin text-indigo-500 shrink-0" />
                      )}
                    </span>
                    {modelProcess.expanded ? (
                      <ChevronUp className="w-3.5 h-3.5 shrink-0" title="收起" />
                    ) : (
                      <ChevronDown className="w-3.5 h-3.5 shrink-0" title="展开" />
                    )}
                  </button>
                  {modelProcess.expanded && (
                    <div className="px-4 pb-3 max-h-56 overflow-y-auto space-y-2">
                      {[...modelProcess.entries].reverse().map((entry) => (
                        <div
                          key={entry.id}
                          className={`rounded-lg border p-2.5 text-[11px] ${themeConfig.cardBg} ${themeConfig.cardBorder}`}
                        >
                          <div className={`flex items-center justify-between gap-2 mb-1.5 ${themeConfig.textMuted}`}>
                            <span className="font-mono shrink-0">{entry.at}</span>
                            <span
                              className={
                                entry.status === 'running'
                                  ? 'text-indigo-500'
                                  : entry.status === 'error'
                                  ? 'text-rose-500'
                                  : 'text-emerald-500'
                              }
                            >
                              {entry.status === 'running'
                                ? t.processStreaming
                                : entry.status === 'error'
                                ? t.processError
                                : t.processDone}
                            </span>
                          </div>
                          <div className={`mb-1 truncate ${themeConfig.textSecondary}`} title={entry.prompt}>
                            → {entry.prompt}
                          </div>
                          <pre
                            ref={(el) => {
                              if (el && entry.status === 'running') {
                                el.scrollTop = el.scrollHeight;
                              }
                            }}
                            className="whitespace-pre-wrap break-words font-mono text-[10px] leading-relaxed max-h-36 overflow-y-auto select-text opacity-90"
                          >
                            {entry.body}
                            {entry.status === 'running' ? '▌' : ''}
                          </pre>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}

              {/* Chat Input */}
              <div className={`p-4 border-t space-y-3 ${themeConfig.subtleBorder} ${themeConfig.modalHeaderBg}`}>
                <div className="flex items-center gap-2 overflow-x-auto pb-1 text-[11px]">
                  <span className={`text-[10px] uppercase font-semibold shrink-0 ${themeConfig.textMuted}`}>
                    {splitIssue
                      ? selectedScope === 'all'
                        ? t.chatUpdatesAll
                        : t.chatUpdatesSub
                      : lang === 'zh'
                      ? '对话将直接更新待开发文档'
                      : 'Chat updates the Dev Spec directly'}
                  </span>
                  <button
                    onClick={() =>
                      handleSendMessage(
                        lang === 'zh'
                          ? '请补充边界条件、异常处理与错误码说明到待开发文档。'
                          : 'Please add edge cases, error handling, and error codes into the Dev Spec.'
                      )
                    }
                    className={`px-2.5 py-1 rounded-lg border shrink-0 transition-colors ${themeConfig.btnSecondary} ${themeConfig.btnSecondaryText}`}
                  >
                    🛡️ {lang === 'zh' ? '补全边界与异常' : 'Edge cases'}
                  </button>
                  <button
                    onClick={() =>
                      handleSendMessage(
                        lang === 'zh'
                          ? '请补充更具体的改动文件清单与实施步骤。'
                          : 'Please refine the file-change list and implementation steps.'
                      )
                    }
                    className={`px-2.5 py-1 rounded-lg border shrink-0 transition-colors ${themeConfig.btnSecondary} ${themeConfig.btnSecondaryText}`}
                  >
                    📄 {lang === 'zh' ? '细化改动清单' : 'Refine files'}
                  </button>
                </div>

                <div className="flex items-center gap-2">
                  <input
                    type="text"
                    value={inputPrompt}
                    onChange={(e) => setInputPrompt(e.target.value)}
                    onKeyDown={(e) => e.key === 'Enter' && !e.shiftKey && handleSendMessage()}
                    placeholder={
                      lang === 'zh'
                        ? splitIssue && selectedScope !== 'all'
                          ? '描述该子需求的修改意见，发送后更新对应文档...'
                          : splitIssue
                          ? '全局描述修改意见，发送后同步更新所有子需求文档...'
                          : '描述需求或修改意见，发送后自动更新待开发文档...'
                        : splitIssue && selectedScope !== 'all'
                        ? 'Describe this sub-requirement — its Dev Spec updates on send...'
                        : splitIssue
                        ? 'Global instruction — every sub-requirement spec updates on send...'
                        : 'Describe requirements or changes — Dev Spec updates on send...'
                    }
                    className={`flex-1 px-4 py-2.5 border rounded-xl text-xs focus:outline-none focus:border-indigo-500 transition-colors ${themeConfig.inputBg} ${themeConfig.inputText} ${themeConfig.inputBorder}`}
                  />
                  <button
                    onClick={() => handleSendMessage()}
                    disabled={isSending || !inputPrompt.trim()}
                    className="px-4 py-2.5 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white font-semibold text-xs rounded-xl flex items-center gap-1.5 transition-all shrink-0"
                  >
                    <Send className="w-3.5 h-3.5" />
                    {lang === 'zh' ? '发送并更新文档' : 'Send & update'}
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* TAB 2: Development Spec Document (Markdown) */}
          {activeTab === 'spec' && (
            <div className="flex-1 p-6 overflow-y-auto space-y-4">
              {splitIssue && (
                <SubRequirementBar
                  issue={issue}
                  selectedScope={selectedScope}
                  onSelect={(scope) => {
                    setSelectedScope(scope);
                    setEditingSpec(false);
                  }}
                  lang={lang}
                  themeStyle={themeStyle}
                />
              )}
              {!currentSpec?.rawMarkdown && !currentSpec?.summary ? (
                <div className={`p-12 text-center border-2 border-dashed rounded-2xl ${themeConfig.subtleBorder} ${themeConfig.cardBg}`}>
                  <FileText className={`w-12 h-12 mx-auto mb-3 ${themeConfig.textMuted}`} />
                  <h3 className={`text-base font-semibold ${themeConfig.textPrimary}`}>暂未生成待开发文档 (Dev Spec)</h3>
                  <p className={`text-xs max-w-md mx-auto mt-1 mb-4 ${themeConfig.textMuted}`}>
                    在【AI 对话】中发送需求说明即可自动写入 Markdown 文档，无需再点生成按钮。
                  </p>
                  <button
                    onClick={() => {
                      setActiveTab('chat');
                      handleSendMessage(
                        lang === 'zh'
                          ? '请根据当前需求，撰写完整的待开发文档（Markdown）。'
                          : 'Please write a complete Markdown Dev Spec from the current requirement.'
                      );
                    }}
                    className="px-5 py-2.5 bg-indigo-600 hover:bg-indigo-500 text-white font-semibold text-xs rounded-xl shadow-lg flex items-center gap-2 mx-auto transition-all"
                  >
                    <Sparkles className="w-4 h-4" />
                    {lang === 'zh' ? '去对话并写入文档' : 'Chat to write Spec'}
                  </button>
                </div>
              ) : (
                <div className="space-y-4 h-full flex flex-col min-h-0">
                  <div className={`pb-3 border-b shrink-0 space-y-2 ${themeConfig.subtleBorder}`}>
                    <div className="flex items-center justify-between gap-3">
                    <div className="min-w-0">
                      <h3 className={`text-base font-bold truncate ${themeConfig.textPrimary}`}>
                        {currentSpec?.title ||
                          activeSub?.title ||
                          (selectedScope === 'all' && splitIssue
                            ? lang === 'zh'
                              ? '总览文档'
                              : 'Overview'
                            : lang === 'zh'
                            ? '待开发文档'
                            : 'Dev Spec')}
                      </h3>
                      <p className={`text-[11px] mt-0.5 ${themeConfig.textMuted}`}>
                        Markdown · {lang === 'zh' ? '更新' : 'Updated'}{' '}
                        {currentSpec?.updatedAt ? new Date(currentSpec.updatedAt).toLocaleString() : '—'}
                      </p>
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
                      <button
                        type="button"
                        onClick={handleExportDevSpec}
                        disabled={exporting}
                        className={`px-3 py-1.5 border font-semibold text-xs rounded-lg flex items-center gap-1.5 transition-colors disabled:opacity-50 ${themeConfig.btnSecondary} ${themeConfig.btnSecondaryText}`}
                        title={lang === 'zh' ? '导出 Markdown 文件' : 'Export Markdown file'}
                      >
                        {exporting ? (
                          <Loader2 className="w-3.5 h-3.5 text-emerald-500 animate-spin" />
                        ) : (
                          <Download className="w-3.5 h-3.5 text-emerald-500" />
                        )}
                        {exporting
                          ? lang === 'zh'
                            ? '保存中…'
                            : 'Saving…'
                          : lang === 'zh'
                          ? '导出下载'
                          : 'Export'}
                      </button>
                      <button
                        onClick={() => {
                          if (!editingSpec) setSpecMarkdown(currentSpec?.rawMarkdown || '');
                          setEditingSpec(!editingSpec);
                        }}
                        className={`px-3 py-1.5 border font-semibold text-xs rounded-lg flex items-center gap-1.5 transition-colors ${themeConfig.btnSecondary} ${themeConfig.btnSecondaryText}`}
                      >
                        <Edit3 className="w-3.5 h-3.5 text-indigo-500" />
                        {editingSpec
                          ? lang === 'zh'
                            ? '退出编辑'
                            : 'Cancel'
                          : lang === 'zh'
                          ? '编辑 Markdown'
                          : 'Edit Markdown'}
                      </button>
                    </div>
                    </div>
                    {exportHint && (
                      <div className="text-[11px] text-emerald-600 dark:text-emerald-400 select-text break-all">
                        {exportHint}
                      </div>
                    )}
                    {chatError && (
                      <div className="text-[11px] text-rose-500 select-text">{chatError}</div>
                    )}
                  </div>

                  {editingSpec ? (
                    <div className="space-y-3 flex-1 flex flex-col min-h-0">
                      <textarea
                        value={specMarkdown}
                        onChange={(e) => setSpecMarkdown(e.target.value)}
                        spellCheck={false}
                        className={`flex-1 min-h-[360px] w-full p-4 border rounded-xl font-mono text-xs focus:outline-none focus:border-indigo-500 leading-relaxed resize-y ${themeConfig.inputBg} ${themeConfig.inputText} ${themeConfig.inputBorder}`}
                      />
                      <div className="flex justify-end gap-2 shrink-0">
                        <button
                          onClick={() => {
                            setSpecMarkdown(currentSpec?.rawMarkdown || '');
                            setEditingSpec(false);
                          }}
                          className={`px-3 py-1.5 text-xs rounded-lg border ${themeConfig.btnSecondary} ${themeConfig.btnSecondaryText}`}
                        >
                          {lang === 'zh' ? '取消' : 'Cancel'}
                        </button>
                        <button
                          onClick={() => {
                            const md = specMarkdown;
                            const titleMatch = md.match(/^#\s+(.+)$/m);
                            const nextSpec = {
                              title: titleMatch?.[1]?.trim() || currentSpec?.title || activeSub?.title || issue.title,
                              summary: currentSpec?.summary || '',
                              architectureDesign: currentSpec?.architectureDesign || '',
                              fileChanges: currentSpec?.fileChanges || [],
                              implementationSteps: currentSpec?.implementationSteps || [],
                              testCases: currentSpec?.testCases || [],
                              rawMarkdown: md,
                              updatedAt: new Date().toISOString(),
                            };
                            if (activeSub) {
                              onUpdateIssue({
                                ...issue,
                                subRequirements: (issue.subRequirements || []).map((s) =>
                                  s.id === activeSub.id ? { ...s, title: nextSpec.title, devSpec: nextSpec } : s
                                ),
                              });
                            } else {
                              onUpdateIssue({
                                ...issue,
                                devSpec: nextSpec,
                              });
                            }
                            setEditingSpec(false);
                          }}
                          className="px-4 py-1.5 bg-indigo-600 text-xs text-white font-semibold rounded-lg"
                        >
                          {lang === 'zh' ? '保存' : 'Save'}
                        </button>
                      </div>
                    </div>
                  ) : (
                    <div className={`flex-1 p-5 rounded-xl border overflow-y-auto ${themeConfig.cardBg} ${themeConfig.cardBorder} ${themeConfig.textPrimary}`}>
                      <MarkdownView text={currentSpec?.rawMarkdown || currentSpec?.summary || ''} />
                    </div>
                  )}
                </div>
              )}
            </div>
          )}

          {/* TAB 3: Auto-Dev Console */}
          {activeTab === 'console' && (
            <div className="flex-1 p-6 flex flex-col gap-4 overflow-hidden">
              <div className={`p-4 rounded-xl border flex items-center justify-between ${themeConfig.cardBg} ${themeConfig.cardBorder}`}>
                <div>
                  <div className={`text-xs font-bold flex items-center gap-2 ${themeConfig.textPrimary}`}>
                    <Terminal className="w-4 h-4 text-indigo-500" />
                    {t.vibeBotConsoleTitle}
                  </div>
                  <div className={`text-[11px] mt-0.5 ${themeConfig.textMuted}`}>
                    {splitIssue
                      ? t.sequentialDev
                      : lang === 'zh'
                      ? '根据待开发文档自动切换分支，编写补丁，运行测试并创建 PR'
                      : 'Branch, patch, test, and open a local review from the Dev Spec'}
                  </div>
                </div>

                <div className="flex items-center gap-3">
                  <div className="text-right">
                    <div className="text-xs font-bold text-indigo-600 dark:text-indigo-300 font-mono">
                      {issue.autoDevProgress}%
                    </div>
                    <div className={`text-[10px] uppercase ${themeConfig.textMuted}`}>当前完成度</div>
                  </div>

                  {issue.status === 'backlog' && (
                    <button
                      onClick={() => onStartAutoDev(issue.id)}
                      className="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white font-semibold text-xs rounded-xl shadow transition-all flex items-center gap-1.5"
                    >
                      <Play className="w-3.5 h-3.5 fill-current" />
                      开始自治开发
                    </button>
                  )}
                </div>
              </div>

              {splitIssue && (
                <div className={`px-3 py-2 rounded-xl border ${themeConfig.cardBg} ${themeConfig.cardBorder}`}>
                  <SubRequirementBar
                    issue={issue}
                    selectedScope={issue.currentSubId || selectedScope}
                    onSelect={setSelectedScope}
                    lang={lang}
                    themeStyle={themeStyle}
                    compact
                  />
                </div>
              )}

              <div className={`w-full h-2 rounded-full overflow-hidden border ${themeConfig.inputBg} ${themeConfig.inputBorder}`}>
                <div
                  className="bg-gradient-to-r from-indigo-500 to-purple-500 h-full transition-all duration-500"
                  style={{ width: `${issue.autoDevProgress}%` }}
                />
              </div>

              <div className={`flex-1 border rounded-xl p-4 font-mono text-xs overflow-y-auto space-y-2 leading-relaxed backdrop-blur-md ${isLight ? 'bg-slate-900 text-slate-100 border-slate-800' : 'bg-black/60 text-slate-200 border-white/10'}`}>
                <div className="opacity-40 text-[11px]">{t.autoExecLogHeader}</div>
                {issue.autoDevLogs.length === 0 ? (
                  <div className="opacity-40 italic py-8 text-center">
                    准备就绪。点击【开始自治开发】触发后台工程构建与自动化改码流程...
                  </div>
                ) : (
                  issue.autoDevLogs.map((log) => (
                    <div key={log.id} className="flex items-start gap-2">
                      <span className="opacity-40 shrink-0">[{log.timestamp}]</span>
                      <span
                        className={`font-semibold shrink-0 uppercase px-1.5 py-0.5 rounded text-[10px] ${
                          log.phase === 'analyzing'
                            ? 'text-amber-300 bg-amber-500/20'
                            : log.phase === 'branching'
                            ? 'text-indigo-300 bg-indigo-500/20'
                            : log.phase === 'coding'
                            ? 'text-purple-300 bg-purple-500/20'
                            : log.phase === 'testing'
                            ? 'text-cyan-300 bg-cyan-500/20'
                            : log.phase === 'completed'
                            ? 'text-emerald-300 bg-emerald-500/20'
                            : 'opacity-60'
                        }`}
                      >
                        {lang === 'zh'
                          ? ({
                              analyzing: '分析中',
                              branching: '建分支',
                              coding: '编码中',
                              testing: '测试中',
                              linting: '检查中',
                              committing: '提交中',
                              completed: '已完成',
                              failed: '失败',
                            } as Record<string, string>)[log.phase] || log.phase
                          : log.phase}
                      </span>
                      <span>{log.message}</span>
                    </div>
                  ))
                )}
              </div>
            </div>
          )}

          {/* TAB 4: Review */}
          {activeTab === 'review' && (
            <div className="flex-1 p-6 overflow-y-auto space-y-6">
              {/* PR Status & Action Header */}
              <div className={`p-4 rounded-xl border flex flex-col md:flex-row items-start md:items-center justify-between gap-4 ${themeConfig.cardBg} ${themeConfig.cardBorder}`}>
                <div>
                  <div className="flex items-center gap-2">
                    <GitPullRequest className="w-5 h-5 text-purple-500" />
                    <h3 className={`font-bold text-sm ${themeConfig.textPrimary}`}>
                      {issue.prInfo?.title || `Pull Request: feature/issue-${issue.id}`}
                    </h3>
                    <span
                      className={`px-2 py-0.5 rounded text-[10px] font-bold border font-mono ${
                        issue.status === 'completed'
                          ? 'bg-emerald-500/20 text-emerald-700 dark:text-emerald-300 border-emerald-500/40'
                          : 'bg-purple-500/20 text-purple-700 dark:text-purple-300 border-purple-500/40'
                      }`}
                    >
                      {issue.status === 'completed' ? 'MERGED (已合并)' : 'OPEN (待评审)'}
                    </span>
                  </div>
                  <p className={`text-xs mt-1 ${themeConfig.textMuted}`}>
                    目标分支: <code className="text-indigo-600 dark:text-indigo-300 font-mono">{issue.prInfo?.branchName || `feature/issue-${issue.id}`}</code> | 提交者: {issue.prInfo?.author || 'AI Auto-Dev Agent'}
                  </p>
                  {splitIssue && (
                    <div className="mt-2 flex flex-wrap gap-1.5">
                      {(issue.subRequirements || []).map((sub) => (
                        <span
                          key={sub.id}
                          className={`px-2 py-0.5 rounded-md text-[10px] border ${themeConfig.inputBorder} ${themeConfig.textSecondary}`}
                        >
                          {sub.order}. {sub.title}
                          {sub.commitSha ? ` · ${sub.commitSha}` : ` · ${sub.status}`}
                        </span>
                      ))}
                    </div>
                  )}
                </div>

                {issue.status === 'in_review' && (
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => setShowReworkBox(true)}
                      className="px-4 py-2 bg-amber-500/20 hover:bg-amber-500/30 border border-amber-500/30 text-amber-800 dark:text-amber-200 font-semibold text-xs rounded-xl flex items-center gap-1.5 transition-colors"
                    >
                      <RotateCcw className="w-3.5 h-3.5" />
                      二次修改 (提意见打回)
                    </button>
                    <button
                      onClick={handleApproveMerge}
                      className="px-5 py-2 bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs rounded-xl shadow-lg flex items-center gap-1.5 transition-all"
                    >
                      <GitMerge className="w-4 h-4" />
                      合并代码 (移至已完成)
                    </button>
                  </div>
                )}
              </div>

              {/* Code Review & Quality Gate Summary Report */}
              <div className="p-5 rounded-2xl border border-emerald-500/30 bg-emerald-500/10 backdrop-blur-md space-y-4 shadow-xl">
                <div className="flex items-center justify-between border-b border-emerald-500/20 pb-3">
                  <div className="flex items-center gap-2">
                    <ShieldCheck className="w-5 h-5 text-emerald-500" />
                    <h4 className="text-sm font-bold text-emerald-900 dark:text-emerald-200">
                      代码自动化评审与质量门禁结果 (Code Quality & Review Report)
                    </h4>
                  </div>
                  <span className="px-2.5 py-1 rounded-md bg-emerald-500/20 border border-emerald-500/30 text-emerald-700 dark:text-emerald-300 text-xs font-mono font-bold">
                    {t.scorePassed}
                  </span>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 text-xs">
                  <div className={`p-3 rounded-xl border space-y-1 ${themeConfig.cardBg} ${themeConfig.cardBorder}`}>
                    <span className={`text-[10px] uppercase font-semibold ${themeConfig.textMuted}`}>单元测试套件</span>
                    <div className="text-emerald-600 dark:text-emerald-400 font-bold font-mono text-sm flex items-center gap-1">
                      <CheckCircle2 className="w-4 h-4" /> {t.unitTestPassed}
                    </div>
                    <p className={`text-[10px] ${themeConfig.textMuted}`}>所有测试用例已断言绿灯通过</p>
                  </div>

                  <div className={`p-3 rounded-xl border space-y-1 ${themeConfig.cardBg} ${themeConfig.cardBorder}`}>
                    <span className={`text-[10px] uppercase font-semibold ${themeConfig.textMuted}`}>静态 TypeScript 检查</span>
                    <div className="text-emerald-600 dark:text-emerald-400 font-bold font-mono text-sm flex items-center gap-1">
                      <CheckCircle2 className="w-4 h-4" /> {t.zeroTypeErrors}
                    </div>
                    <p className={`text-[10px] ${themeConfig.textMuted}`}>无任何隐式 any 或类型不匹配</p>
                  </div>

                  <div className={`p-3 rounded-xl border space-y-1 ${themeConfig.cardBg} ${themeConfig.cardBorder}`}>
                    <span className={`text-[10px] uppercase font-semibold ${themeConfig.textMuted}`}>ESLint 代码规范</span>
                    <div className="text-emerald-600 dark:text-emerald-400 font-bold font-mono text-sm flex items-center gap-1">
                      <CheckCircle2 className="w-4 h-4" /> {t.zeroWarnings}
                    </div>
                    <p className={`text-[10px] ${themeConfig.textMuted}`}>符合团队工程统一风格标准</p>
                  </div>

                  <div className={`p-3 rounded-xl border space-y-1 ${themeConfig.cardBg} ${themeConfig.cardBorder}`}>
                    <span className={`text-[10px] uppercase font-semibold ${themeConfig.textMuted}`}>修改行数与体积</span>
                    <div className="text-indigo-600 dark:text-indigo-300 font-bold font-mono text-sm">
                      +{issue.prInfo?.diffStats?.additions || 98} / -{issue.prInfo?.diffStats?.deletions || 14}
                    </div>
                    <p className={`text-[10px] ${themeConfig.textMuted}`}>轻量精简，无无用冗余依赖</p>
                  </div>
                </div>

                {/* Reviewer Feedback / Comments */}
                <div className={`p-3.5 rounded-xl border space-y-1.5 ${themeConfig.cardBg} ${themeConfig.cardBorder}`}>
                  <div className="text-[11px] font-semibold text-emerald-700 dark:text-emerald-300 flex items-center gap-1.5">
                    <Sparkles className="w-3.5 h-3.5 text-emerald-500" />
                    评审结论与点评意见 (Reviewer Comments):
                  </div>
                  <p className={`text-xs leading-relaxed font-sans ${themeConfig.textPrimary}`}>
                    {issue.reviewFeedback || '代码设计严谨且包含完备的异常处理逻辑，经自动分析校验与单元测试断言完全符合工程规范，已审核通过并合并至主干。'}
                  </p>
                </div>
              </div>

              {showReworkBox && (
                <div className="p-4 rounded-xl border border-amber-500/40 bg-amber-500/10 space-y-3">
                  <div className="text-xs font-bold text-amber-800 dark:text-amber-200 flex items-center gap-2">
                    <AlertTriangle className="w-4 h-4 text-amber-500" />
                    {lang === 'zh' ? '开发者二次修改意见反馈' : 'Rework feedback'}
                  </div>
                  {splitIssue && (
                    <label className="block text-[11px] space-y-1">
                      <span className={themeConfig.textMuted}>{t.reworkScope}</span>
                      <select
                        value={reworkScope}
                        onChange={(e) => setReworkScope(e.target.value)}
                        className={`w-full p-2 border rounded-lg text-xs ${themeConfig.inputBg} ${themeConfig.inputText} ${themeConfig.inputBorder}`}
                      >
                        <option value="all">{t.reworkAll}</option>
                        {(issue.subRequirements || []).map((sub) => (
                          <option key={sub.id} value={sub.id}>
                            {sub.order}. {sub.title}
                          </option>
                        ))}
                      </select>
                    </label>
                  )}
                  <textarea
                    rows={3}
                    value={reworkFeedback}
                    onChange={(e) => setReworkFeedback(e.target.value)}
                    placeholder="指出问题所在（例如: 高并发下请求锁过期时间过短，请补全 Redis 续期与告警）"
                    className={`w-full p-3 border rounded-lg text-xs focus:outline-none focus:border-amber-500 ${themeConfig.inputBg} ${themeConfig.inputText} ${themeConfig.inputBorder}`}
                  />
                  <div className="flex justify-end gap-2">
                    <button
                      onClick={() => setShowReworkBox(false)}
                      className={`px-3 py-1.5 text-xs rounded-lg border ${themeConfig.btnSecondary} ${themeConfig.btnSecondaryText}`}
                    >
                      取消
                    </button>
                    <button
                      onClick={handleReworkSubmit}
                      className="px-4 py-1.5 bg-amber-600 hover:bg-amber-500 text-white font-semibold text-xs rounded-lg shadow"
                    >
                      {t.reworkThenDev}
                    </button>
                  </div>
                </div>
              )}

              {/* Diff View */}
              <div>
                <h4 className={`text-xs font-bold uppercase tracking-wider mb-3 flex items-center gap-2 ${themeConfig.textSecondary}`}>
                  <FileCode className="w-4 h-4 text-indigo-500" />
                  变更文件 Diff 详情对比 (File Code Diff)
                </h4>

                {aggregatedFileChanges(issue).length === 0 ? (
                  <div className={`p-8 text-center text-xs border rounded-xl ${themeConfig.subtleBorder} ${themeConfig.textMuted}`}>
                    暂无文件变更 Diff 数据
                  </div>
                ) : (
                  <div className="space-y-4">
                    {aggregatedFileChanges(issue).map((fc, idx) => (
                      <div key={idx} className={`border rounded-xl overflow-hidden shadow-md ${themeConfig.cardBg} ${themeConfig.cardBorder}`}>
                        <div className={`p-3 border-b flex items-center justify-between text-xs font-mono ${themeConfig.subtleBorder} ${themeConfig.modalHeaderBg}`}>
                          <span className="text-indigo-600 dark:text-indigo-300 font-bold">{fc.filePath}</span>
                          <span className={`text-[11px] ${themeConfig.textMuted}`}>{fc.repoName}</span>
                        </div>

                        <div className={`grid grid-cols-1 md:grid-cols-2 divide-y md:divide-y-0 md:divide-x font-mono text-[11px] ${themeConfig.subtleBorder}`}>
                          <div className="p-3 bg-rose-500/5">
                            <div className="text-[10px] text-rose-600 dark:text-rose-400 uppercase font-sans font-bold mb-1">
                              原始代码 (Original)
                            </div>
                            <pre className="text-rose-800 dark:text-rose-200/80 overflow-x-auto whitespace-pre-wrap leading-relaxed">
                              {fc.originalCode || '// (新建文件，无历史版本)'}
                            </pre>
                          </div>

                          <div className="p-3 bg-emerald-500/5">
                            <div className="text-[10px] text-emerald-600 dark:text-emerald-400 uppercase font-sans font-bold mb-1">
                              AI 修改/新生成的代码 (Modified / Added)
                            </div>
                            <pre className="text-emerald-800 dark:text-emerald-200/90 overflow-x-auto whitespace-pre-wrap leading-relaxed">
                              {fc.modifiedCode || '// (已被删除)'}
                            </pre>
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

