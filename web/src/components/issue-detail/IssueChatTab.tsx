import React, { RefObject } from 'react';
import { Issue, ChatMessage, PendingLLMSession } from '../../types';
import { Language, getTranslation, ThemeStyle } from '../../lib/i18n';
import { THEME_CONFIGS } from '../../lib/theme';
import { MarkdownView } from '../../lib/markdown';
import { SubRequirementBar } from '../SubRequirementBar';
import { ModelProcessState, SendMessageOpts } from './useIssueChat';
import { LLMRecoveryBar } from './LLMRecoveryBar';
import { requirementAccepted, legacySpecOnly } from '../../lib/subreq';
import {
  Sparkles,
  Bot,
  User,
  Loader2,
  Eye,
  ChevronDown,
  ChevronUp,
  Send,
  FileText,
} from 'lucide-react';
import { formatFileSize } from '../../lib/attachments';

interface IssueChatTabProps {
  issue: Issue;
  splitIssue: boolean;
  visibleMessages: ChatMessage[];
  selectedScope: string;
  setSelectedScope: (scope: string) => void;
  lang: Language;
  themeStyle: ThemeStyle;
  isSending: boolean;
  chatError: string;
  inputPrompt: string;
  setInputPrompt: (value: string) => void;
  modelProcess: ModelProcessState;
  setModelProcess: React.Dispatch<React.SetStateAction<ModelProcessState>>;
  abortRef: RefObject<AbortController | null>;
  messagesEndRef: RefObject<HTMLDivElement | null>;
  handleSendMessage: (customPrompt?: string, opts?: SendMessageOpts) => Promise<void>;
  pendingLlm?: PendingLLMSession | null;
  onRetrySession?: () => void;
  onRegenerate?: () => void;
}

export const IssueChatTab: React.FC<IssueChatTabProps> = ({
  issue,
  splitIssue,
  visibleMessages,
  selectedScope,
  setSelectedScope,
  lang,
  themeStyle,
  isSending,
  chatError,
  inputPrompt,
  setInputPrompt,
  modelProcess,
  setModelProcess,
  abortRef,
  messagesEndRef,
  handleSendMessage,
  pendingLlm,
  onRetrySession,
  onRegenerate,
}) => {
  const themeConfig = THEME_CONFIGS[themeStyle] || THEME_CONFIGS.light;
  const t = getTranslation(lang);
  const reqOk = requirementAccepted(issue) || legacySpecOnly(issue);
  const canDesign = reqOk && (issue.associatedRepoIds || []).length > 0;
  const hasDesign = !!(
    issue.devSpec?.rawMarkdown?.trim() ||
    (issue.subRequirements || []).some((s) => !!s.devSpec?.rawMarkdown?.trim())
  );

  return (
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

        {(issue.description || (issue.attachments && issue.attachments.length > 0)) && (
          <div className={`rounded-2xl border p-4 space-y-3 ${themeConfig.cardBg}`}>
            {issue.description && (
              <div className={`text-sm leading-relaxed whitespace-pre-wrap ${themeConfig.textSecondary}`}>
                {issue.description}
              </div>
            )}
            {(issue.attachments || []).length > 0 && (
              <div className="flex flex-wrap gap-2">
                {(issue.attachments || []).map((att) => (
                  <div
                    key={att.id}
                    className={`flex items-center gap-2 px-2.5 py-1.5 rounded-xl border text-[11px] ${themeConfig.inputBg} ${themeConfig.inputBorder} ${themeConfig.textSecondary}`}
                  >
                    {att.kind === 'image' && att.dataUrl ? (
                      <img src={att.dataUrl} alt="" className="w-7 h-7 rounded-md object-cover" />
                    ) : (
                      <FileText className="w-3.5 h-3.5 text-indigo-400" />
                    )}
                    <span className="max-w-[160px] truncate">{att.name}</span>
                    <span className={themeConfig.textMuted}>{formatFileSize(att.size)}</span>
                  </div>
                ))}
              </div>
            )}
          </div>
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
                  ? `${themeConfig.cardBg} border ${themeConfig.subtleBorder} ${themeConfig.textPrimary} rounded-tl-none font-sans select-text`
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
        {chatError && !pendingLlm && (
          <div className="text-xs text-rose-500 px-2 select-text">{chatError}</div>
        )}
        <LLMRecoveryBar
          session={pendingLlm || null}
          lang={lang}
          themeStyle={themeStyle}
          busy={isSending}
          onRetrySession={() => onRetrySession?.()}
          onRegenerate={() => onRegenerate?.()}
        />
        <div ref={messagesEndRef} />
      </div>

      {/* Ephemeral model process: compact bar + upward overlay (does not crush chat scroll). */}
      {modelProcess.entries.length > 0 && (
        <div className={`relative shrink-0 border-t z-20 ${themeConfig.subtleBorder} ${themeConfig.modalHeaderBg}`}>
          {modelProcess.expanded && (
            <div
              className={`absolute bottom-full left-0 right-0 max-h-[42vh] overflow-y-auto border-t shadow-[0_-8px_24px_rgba(0,0,0,0.12)] ${themeConfig.subtleBorder} ${themeConfig.modalBg}`}
            >
              <div className="px-4 py-3 space-y-2">
                {[...modelProcess.entries].reverse().map((entry) => (
                  <div
                    key={entry.id}
                    className={`rounded-lg border p-2.5 text-[11px] ${themeConfig.cardBg} ${themeConfig.subtleBorder}`}
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
                      className="whitespace-pre-wrap break-words font-mono text-[10px] leading-relaxed max-h-40 overflow-y-auto select-text opacity-90"
                    >
                      {entry.body}
                      {entry.status === 'running' ? '▌' : ''}
                    </pre>
                  </div>
                ))}
              </div>
            </div>
          )}
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
                  {lang === 'zh' ? '，仅本会话' : ', session only'})
                </span>
              </span>
              {modelProcess.entries.some((e) => e.status === 'running') && (
                <Loader2 className="w-3 h-3 animate-spin text-indigo-500 shrink-0" />
              )}
            </span>
            <span className="flex items-center gap-1 shrink-0 text-[10px] font-medium">
              <span className={themeConfig.textMuted}>
                {modelProcess.expanded
                  ? lang === 'zh'
                    ? '收起'
                    : 'Hide'
                  : lang === 'zh'
                    ? '查看'
                    : 'Show'}
              </span>
              {modelProcess.expanded ? (
                <ChevronDown className="w-3.5 h-3.5" />
              ) : (
                <ChevronUp className="w-3.5 h-3.5" />
              )}
            </span>
          </button>
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
              ? '闲聊只分析；改设计请点「修订开发设计」或直接说「更新开发设计」'
              : 'Chat is analysis only — use “Revise design” or say “update Dev Spec”'}
          </span>
          <button
            onClick={() =>
              handleSendMessage(
                lang === 'zh'
                  ? '请帮我澄清验收标准与边界条件（只分析，先不要改文档）。'
                  : 'Help clarify acceptance criteria and edge cases (analysis only, do not rewrite docs).'
              )
            }
            className={`px-2.5 py-1 rounded-lg border shrink-0 transition-colors ${themeConfig.btnSecondary} ${themeConfig.btnSecondaryText}`}
          >
            🛡️ {lang === 'zh' ? '澄清验收与边界' : 'Clarify acceptance'}
          </button>
          <button
            onClick={() =>
              handleSendMessage(
                lang === 'zh'
                  ? '请根据当前讨论提炼完整需求文档（目标、范围、非目标、验收、约束）。'
                  : 'Extract a complete requirement document from this discussion.',
                { forceReqDoc: true }
              )
            }
            className={`px-2.5 py-1 rounded-lg border shrink-0 transition-colors ${themeConfig.btnSecondary} ${themeConfig.btnSecondaryText}`}
          >
            📄 {t.extractReqBtn}
          </button>
          {canDesign && (
            <button
              type="button"
              disabled={!!isSending}
              onClick={() => {
                const custom =
                  inputPrompt.trim() ||
                  (lang === 'zh'
                    ? hasDesign
                      ? '请结合当前对话与关联仓库源码摘要，修订并写回完整开发设计文档。'
                      : '请基于关联仓库源码摘要与当前讨论，撰写完整开发设计。'
                    : hasDesign
                      ? 'Revise the Dev Spec from this discussion and associated repo excerpts; write back the full document.'
                      : 'Write a complete Dev Spec from associated repo excerpts and this discussion.');
                void handleSendMessage(custom, { forceSpecSync: true });
              }}
              className={`px-2.5 py-1 rounded-lg border shrink-0 transition-colors ${themeConfig.btnSecondary} ${themeConfig.btnSecondaryText} disabled:opacity-50`}
              title={
                lang === 'zh'
                  ? '会读取源码摘要并覆盖更新「开发设计」页文档'
                  : 'Reads source excerpts and overwrites the Dev Spec document'
              }
            >
              <span className="inline-flex items-center gap-1">
                <FileText className="w-3 h-3" />
                {lang === 'zh'
                  ? hasDesign
                    ? '修订开发设计'
                    : '生成开发设计'
                  : hasDesign
                    ? 'Revise design'
                    : 'Generate design'}
              </span>
            </button>
          )}
        </div>

        <div className="flex items-center gap-2">
          <input
            type="text"
            value={inputPrompt}
            onChange={(e) => setInputPrompt(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && !e.shiftKey && handleSendMessage()}
            placeholder={
              lang === 'zh'
                ? canDesign
                  ? '讨论点直接发；若要写回设计，说「更新开发设计」或点「修订开发设计」…'
                  : '输入问题或讨论点（不会自动改文档）…'
                : canDesign
                  ? 'Discuss freely; to write the design say “update Dev Spec” or tap Revise…'
                  : 'Ask or discuss (will not auto-update documents)…'
            }
            className={`flex-1 px-4 py-2.5 border rounded-xl text-xs focus:outline-none focus:border-indigo-500 transition-colors ${themeConfig.inputBg} ${themeConfig.inputText} ${themeConfig.inputBorder}`}
          />
          <button
            onClick={() => handleSendMessage()}
            disabled={isSending || !inputPrompt.trim()}
            className="px-4 py-2.5 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white font-semibold text-xs rounded-xl flex items-center gap-1.5 transition-all shrink-0"
          >
            <Send className="w-3.5 h-3.5" />
            {lang === 'zh' ? '发送' : 'Send'}
          </button>
        </div>
      </div>
    </div>
  );
};
