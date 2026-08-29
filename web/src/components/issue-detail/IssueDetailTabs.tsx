import React from 'react';
import { Issue } from '../../types';
import { Language, getTranslation, ThemeStyle } from '../../lib/i18n';
import { THEME_CONFIGS } from '../../lib/theme';
import { MessageSquare, FileText, Terminal, GitPullRequest, CheckCircle2 } from 'lucide-react';

interface IssueDetailTabsProps {
  issue: Issue;
  splitIssue: boolean;
  activeTab: 'chat' | 'spec' | 'console' | 'review';
  setActiveTab: (tab: 'chat' | 'spec' | 'console' | 'review') => void;
  lang: Language;
  themeStyle: ThemeStyle;
}

export const IssueDetailTabs: React.FC<IssueDetailTabsProps> = ({
  issue,
  splitIssue,
  activeTab,
  setActiveTab,
  lang,
  themeStyle,
}) => {
  const themeConfig = THEME_CONFIGS[themeStyle] || THEME_CONFIGS.glass;
  const t = getTranslation(lang);

  return (
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
  );
};
