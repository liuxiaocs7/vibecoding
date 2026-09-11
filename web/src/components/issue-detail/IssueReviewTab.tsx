import React from 'react';
import { Issue } from '../../types';
import { Language, getTranslation, ThemeStyle } from '../../lib/i18n';
import { THEME_CONFIGS } from '../../lib/theme';
import { DiffReview } from '../DiffReview';
import {
  GitPullRequest,
  RotateCcw,
  GitMerge,
  AlertTriangle,
} from 'lucide-react';
import { ThemedSelect } from '../ThemedSelect';

interface IssueReviewTabProps {
  issue: Issue;
  splitIssue: boolean;
  lang: Language;
  themeStyle: ThemeStyle;
  showReworkBox: boolean;
  setShowReworkBox: React.Dispatch<React.SetStateAction<boolean>>;
  reworkFeedback: string;
  setReworkFeedback: React.Dispatch<React.SetStateAction<string>>;
  reworkScope: string;
  setReworkScope: React.Dispatch<React.SetStateAction<string>>;
  handleReworkSubmit: () => Promise<void>;
  handleApproveMerge: () => Promise<void>;
}

export const IssueReviewTab: React.FC<IssueReviewTabProps> = ({
  issue,
  splitIssue,
  lang,
  themeStyle,
  showReworkBox,
  setShowReworkBox,
  reworkFeedback,
  setReworkFeedback,
  reworkScope,
  setReworkScope,
  handleReworkSubmit,
  handleApproveMerge,
}) => {
  const themeConfig = THEME_CONFIGS[themeStyle] || THEME_CONFIGS.glass;
  const t = getTranslation(lang);

  return (
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

      {/* Real diff review */}
      <DiffReview issueId={issue.id} lang={lang} />

      {showReworkBox && (
        <div className="p-4 rounded-xl border border-amber-500/40 bg-amber-500/10 space-y-3">
          <div className="text-xs font-bold text-amber-800 dark:text-amber-200 flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 text-amber-500" />
            {lang === 'zh' ? '开发者二次修改意见反馈' : 'Rework feedback'}
          </div>
          {splitIssue && (
            <label className="block text-[11px] space-y-1">
              <span className={themeConfig.textMuted}>{t.reworkScope}</span>
              <ThemedSelect
                value={reworkScope}
                onChange={(e) => setReworkScope(e.target.value)}
                isLight={themeConfig.isLight}
                chevronClassName={themeConfig.textSecondary}
                className={`p-2 border rounded-lg text-xs ${themeConfig.inputBg} ${themeConfig.inputText} ${themeConfig.inputBorder}`}
              >
                <option value="all">{t.reworkAll}</option>
                {(issue.subRequirements || []).map((sub) => (
                  <option key={sub.id} value={sub.id}>
                    {sub.order}. {sub.title}
                  </option>
                ))}
              </ThemedSelect>
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
    </div>
  );
};
