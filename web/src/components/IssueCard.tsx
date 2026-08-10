import React from 'react';
import { Issue, GitRepo, BranchPrefixConfig } from '../types';
import { Language, ThemeStyle, getTranslation } from '../lib/i18n';
import { THEME_CONFIGS } from '../lib/theme';
import { GitBranch, FileText, CheckCircle2, Play, Sparkles, Tag, GitPullRequest, ShieldCheck, ArrowRight } from 'lucide-react';

interface IssueCardProps {
  issue: Issue;
  onClick: () => void;
  onStartAutoDev: (issueId: string) => void;
  onMoveColumn: (issueId: string, newStatus: Issue['status']) => void;
  gitRepos: GitRepo[];
  branchPrefixConfig?: BranchPrefixConfig;
  lang?: Language;
  themeStyle?: ThemeStyle;
}

export const IssueCard: React.FC<IssueCardProps> = ({
  issue,
  onClick,
  onStartAutoDev,
  onMoveColumn,
  gitRepos,
  branchPrefixConfig,
  lang = 'en',
  themeStyle = 'glass',
}) => {
  const t = getTranslation(lang);
  const themeConfig = THEME_CONFIGS[themeStyle] || THEME_CONFIGS.glass;
  const isLight = themeConfig.isLight;

  const associatedRepos = gitRepos.filter((r) => issue.associatedRepoIds.includes(r.id));

  const priorityBadge = {
    low: {
      label: t.priorityLow,
      color: isLight
        ? 'bg-slate-100 text-slate-800 border-slate-300 font-medium'
        : 'bg-slate-800/90 text-slate-200 border-slate-700 font-medium',
    },
    medium: {
      label: t.priorityMedium,
      color: isLight
        ? 'bg-indigo-100 text-indigo-900 border-indigo-300 font-semibold'
        : 'bg-indigo-950/90 text-indigo-200 border-indigo-700/80 font-semibold',
    },
    high: {
      label: t.priorityHigh,
      color: isLight
        ? 'bg-amber-100 text-amber-900 border-amber-300 font-semibold'
        : 'bg-amber-950/90 text-amber-200 border-amber-700/80 font-semibold',
    },
    urgent: {
      label: t.priorityUrgent,
      color: isLight
        ? 'bg-rose-100 text-rose-900 border-rose-300 font-bold animate-pulse'
        : 'bg-rose-950/90 text-rose-200 border-rose-700/80 font-bold animate-pulse',
    },
  }[issue.priority];

  // Derive target branch preview name based on status & prefix config
  const autoDevPrefix = branchPrefixConfig?.autoDevPrefix || 'ai-dev/';
  const featurePrefix = branchPrefixConfig?.featurePrefix || 'feature/';
  const activeBranchName = issue.prInfo?.branchName || `${issue.status === 'in_progress' ? autoDevPrefix : featurePrefix}issue-${issue.id.slice(-4)}`;

  const isCompleted = issue.status === 'completed';
  const isInReview = issue.status === 'in_review';

  return (
    <div
      onClick={onClick}
      className={`p-4 rounded-xl border backdrop-blur-md transition-all shadow-sm hover:shadow-md group cursor-pointer flex flex-col justify-between gap-3 relative overflow-hidden ${
        isCompleted
          ? isLight
            ? 'border-emerald-300 bg-emerald-50/90 hover:bg-emerald-100/60 hover:border-emerald-400'
            : 'border-emerald-500/40 bg-emerald-950/30 hover:bg-emerald-950/40 hover:border-emerald-500/60'
          : isInReview
          ? isLight
            ? 'border-purple-300 bg-purple-50/90 hover:bg-purple-100/60 hover:border-purple-400'
            : 'border-purple-500/40 bg-purple-950/30 hover:bg-purple-950/40 hover:border-purple-500/60'
          : `${themeConfig.cardBg} ${themeConfig.cardHoverBg}`
      }`}
    >
      {/* Accent top bar */}
      <div
        className={`absolute top-0 left-0 right-0 h-1 ${
          isCompleted
            ? 'bg-gradient-to-r from-emerald-500 via-teal-500 to-emerald-400'
            : isInReview
            ? 'bg-gradient-to-r from-purple-500 to-indigo-500'
            : issue.status === 'in_progress'
            ? 'bg-gradient-to-r from-indigo-500 to-purple-500'
            : 'bg-transparent'
        }`}
      />

      {/* Top Header */}
      <div>
        <div className="flex items-center justify-between gap-2 mb-2">
          <div className="flex items-center gap-1.5">
            <span className={`px-2 py-0.5 rounded-md text-[10px] border ${priorityBadge.color}`}>
              {priorityBadge.label}
            </span>
            {isCompleted && (
              <span className={`px-2 py-0.5 rounded-md text-[10px] font-bold border flex items-center gap-1 ${
                isLight
                  ? 'bg-emerald-100 text-emerald-900 border-emerald-300'
                  : 'bg-emerald-950/90 text-emerald-200 border-emerald-700/80'
              }`}>
                <CheckCircle2 className="w-3 h-3 text-emerald-500 shrink-0" />
                {t.approvedMerged}
              </span>
            )}
            {isInReview && (
              <span className={`px-2 py-0.5 rounded-md text-[10px] font-bold border flex items-center gap-1 ${
                isLight
                  ? 'bg-purple-100 text-purple-900 border-purple-300'
                  : 'bg-purple-950/90 text-purple-200 border-purple-700/80'
              }`}>
                <GitPullRequest className="w-3 h-3 text-purple-400 shrink-0" />
                {t.reviewPending}
              </span>
            )}
          </div>

          <span className={`text-[10px] font-mono tracking-wide ${themeConfig.textMuted}`}>#{issue.id}</span>
        </div>

        {/* Title */}
        <h4 className={`font-semibold text-xs transition-colors line-clamp-2 leading-snug ${themeConfig.textPrimary} group-hover:text-indigo-500 dark:group-hover:text-indigo-300`}>
          {issue.title}
        </h4>

        {/* Repos & Branch Tags */}
        <div className="flex items-center gap-1.5 flex-wrap mt-2.5">
          {associatedRepos.map((repo) => (
            <span
              key={repo.id}
              title={`${t.repoTooltip}: ${repo.name}`}
              className={`px-2 py-0.5 rounded-md text-[10px] font-mono border flex items-center gap-1 shrink-0 transition-colors ${themeConfig.badgeRepoBg} ${themeConfig.badgeRepoText}`}
            >
              <GitBranch className="w-3 h-3 text-indigo-400 shrink-0" />
              {repo.name}
            </span>
          ))}

          {/* Target Branch Spec Tag */}
          <span
            title={`${t.branchTooltip}: ${activeBranchName}`}
            className={`px-1.5 py-0.5 rounded-md border text-[9px] font-mono flex items-center gap-1 shrink-0 ${themeConfig.badgeBranchBg} ${themeConfig.badgeBranchText}`}
          >
            <Tag className="w-2.5 h-2.5 text-indigo-400 shrink-0" />
            {activeBranchName}
          </span>
        </div>
      </div>

      {/* Middle Spec & Progress Status */}
      <div className={`space-y-2 pt-2 border-t ${themeConfig.subtleBorder}`}>
        <div className="flex items-center justify-between text-[11px]">
          <span className={`flex items-center gap-1 text-[10px] font-medium ${themeConfig.textSecondary}`}>
            <FileText className="w-3 h-3 opacity-80" />
            {t.devSpecLabel}:
          </span>
          {issue.devSpec ? (
            <span className={`font-semibold flex items-center gap-1 text-[10px] ${
              isLight ? 'text-emerald-700' : 'text-emerald-300'
            }`}>
              <CheckCircle2 className="w-3 h-3" />
              {t.specGenerated}
            </span>
          ) : (
            <span className={`italic text-[10px] ${
              isLight ? 'text-amber-800' : 'text-amber-300/90'
            }`}>{t.pendingSpec}</span>
          )}
        </div>

        {/* Status badge for completed card */}
        {isCompleted && (
          <div className={`p-1.5 rounded-lg border flex items-center justify-between text-[10px] ${
            isLight
              ? 'bg-emerald-50 border-emerald-200 text-emerald-900'
              : 'bg-emerald-950/80 border-emerald-700/80 text-emerald-200'
          }`}>
            <span className="flex items-center gap-1 font-semibold">
              <ShieldCheck className="w-3.5 h-3.5 text-emerald-400 shrink-0" />
              {t.approvedMerged}
            </span>
            <span className="font-mono text-[9px] opacity-90">PR #{issue.prInfo?.id || '104'}</span>
          </div>
        )}

        {/* Progress Bar if in progress */}
        {issue.status === 'in_progress' && (
          <div className="space-y-1">
            <div className={`flex justify-between text-[10px] font-mono font-bold ${
              isLight ? 'text-indigo-900' : 'text-indigo-200'
            }`}>
              <span className="flex items-center gap-1">
                <Sparkles className="w-3 h-3 text-indigo-400 animate-spin" />
                {t.autoDevCoding}
              </span>
              <span>{issue.autoDevProgress}%</span>
            </div>
            <div className={`w-full h-1.5 rounded-full overflow-hidden border ${isLight ? 'bg-slate-200 border-slate-300' : 'bg-black/60 border-white/20'}`}>
              <div
                className="bg-gradient-to-r from-indigo-500 via-purple-500 to-pink-500 h-full transition-all duration-300"
                style={{ width: `${issue.autoDevProgress}%` }}
              />
            </div>
          </div>
        )}
      </div>

      {/* Card Footer Actions */}
      <div className={`flex items-center justify-between text-[11px] pt-1 ${themeConfig.textSecondary}`}>
        <div className="flex items-center gap-1.5 text-[10px]">
          <div className={`w-4 h-4 rounded-full flex items-center justify-center text-[9px] font-bold ${
            isLight
              ? 'bg-indigo-100 border border-indigo-300 text-indigo-900'
              : 'bg-indigo-950/90 border border-indigo-700/80 text-indigo-200'
          }`}>
            {issue.assignee ? issue.assignee[0].toUpperCase() : 'U'}
          </div>
          <span className={`truncate max-w-[80px] font-medium ${
            isLight ? 'text-slate-700' : 'text-slate-200'
          }`}>{issue.assignee}</span>
        </div>

        {/* Quick Column Move Actions */}
        <div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
          {issue.status === 'requirements' && (
            <button
              onClick={() => onMoveColumn(issue.id, 'backlog')}
              className={`px-2.5 py-1 border rounded-lg text-[10px] font-bold flex items-center gap-1 transition-all ${
                isLight
                  ? 'bg-cyan-100 hover:bg-cyan-200 border-cyan-300 text-cyan-950'
                  : 'bg-cyan-950/90 hover:bg-cyan-900/90 border-cyan-700/80 text-cyan-200'
              }`}
            >
              <Sparkles className="w-3 h-3 text-cyan-400" />
              {t.scheduleToBacklog}
            </button>
          )}

          {issue.status === 'backlog' && (
            <button
              onClick={() => onStartAutoDev(issue.id)}
              className="px-2.5 py-1 bg-indigo-600 hover:bg-indigo-500 border border-indigo-400/50 rounded-lg text-[10px] font-bold text-white flex items-center gap-1 shadow-md transition-all active:scale-95"
            >
              <Play className="w-3 h-3 fill-current" />
              {t.startAutoDev}
            </button>
          )}

          {issue.status === 'in_review' && (
            <button
              onClick={onClick}
              className="px-2.5 py-1 bg-purple-600 hover:bg-purple-500 border border-purple-400/50 text-white rounded-lg text-[10px] font-bold flex items-center gap-1 transition-all shadow-md animate-pulse"
            >
              <GitPullRequest className="w-3 h-3" />
              {t.reviewPR}
            </button>
          )}

          {isCompleted && (
            <button
              onClick={onClick}
              className={`px-2 py-1 border rounded-lg text-[10px] font-bold flex items-center gap-1 transition-all ${
                isLight
                  ? 'bg-emerald-100 hover:bg-emerald-200 border-emerald-300 text-emerald-950'
                  : 'bg-emerald-950/90 hover:bg-emerald-900/90 border-emerald-700/80 text-emerald-200'
              }`}
            >
              <span>{t.viewSpecDiff}</span>
              <ArrowRight className="w-3 h-3 text-emerald-400" />
            </button>
          )}
        </div>
      </div>
    </div>
  );
};



