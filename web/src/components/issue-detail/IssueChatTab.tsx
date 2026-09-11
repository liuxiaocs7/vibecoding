import React, { RefObject } from 'react';
import { Issue, ChatMessage } from '../../types';
import { Language, getTranslation, ThemeStyle } from '../../lib/i18n';
import { THEME_CONFIGS } from '../../lib/theme';
import { MarkdownView } from '../../lib/markdown';
import { SubRequirementBar } from '../SubRequirementBar';
import { ModelProcessState } from './useIssueChat';
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
  handleSendMessage: (
    customPrompt?: string,
    opts?: { forceSpecSync?: boolean; split?: boolean; scope?: string }
  ) => Promise<void>;
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
}) => {
  const themeConfig = THEME_CONFIGS[themeStyle] || THEME_CONFIGS.glass;
  const t = getTranslation(lang);

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
  );
};
