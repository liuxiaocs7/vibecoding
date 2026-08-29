import React from 'react';
import { Issue, SubRequirement, DevSpec } from '../../types';
import { Language, ThemeStyle } from '../../lib/i18n';
import { THEME_CONFIGS } from '../../lib/theme';
import { MarkdownView } from '../../lib/markdown';
import { SubRequirementBar } from '../SubRequirementBar';
import { FileText, Sparkles, Loader2, Download, Edit3 } from 'lucide-react';

interface IssueSpecTabProps {
  issue: Issue;
  splitIssue: boolean;
  activeSub: SubRequirement | undefined;
  currentSpec: DevSpec | undefined;
  selectedScope: string;
  setSelectedScope: (scope: string) => void;
  lang: Language;
  themeStyle: ThemeStyle;
  editingSpec: boolean;
  setEditingSpec: React.Dispatch<React.SetStateAction<boolean>>;
  specMarkdown: string;
  setSpecMarkdown: (value: string) => void;
  exporting: boolean;
  exportHint: string;
  chatError: string;
  setActiveTab: (tab: 'chat' | 'spec' | 'console' | 'review') => void;
  handleSendMessage: (
    customPrompt?: string,
    opts?: { forceSpecSync?: boolean; split?: boolean; scope?: string }
  ) => Promise<void>;
  handleExportDevSpec: () => Promise<void>;
  onUpdateIssue: (updatedIssue: Issue) => void;
}

export const IssueSpecTab: React.FC<IssueSpecTabProps> = ({
  issue,
  splitIssue,
  activeSub,
  currentSpec,
  selectedScope,
  setSelectedScope,
  lang,
  themeStyle,
  editingSpec,
  setEditingSpec,
  specMarkdown,
  setSpecMarkdown,
  exporting,
  exportHint,
  chatError,
  setActiveTab,
  handleSendMessage,
  handleExportDevSpec,
  onUpdateIssue,
}) => {
  const themeConfig = THEME_CONFIGS[themeStyle] || THEME_CONFIGS.glass;

  return (
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
  );
};
