import React from 'react';
import { Issue, GitRepo, IssueStatus, BranchPrefixConfig } from '../types';
import { Language, ThemeStyle, getTranslation } from '../lib/i18n';
import { THEME_CONFIGS } from '../lib/theme';
import { IssueCard } from './IssueCard';
import { Clock, Play, CheckCircle2, GitPullRequest, Plus, ListTodo } from 'lucide-react';

interface KanbanBoardProps {
  issues: Issue[];
  gitRepos: GitRepo[];
  branchPrefixConfig?: BranchPrefixConfig;
  onSelectIssue: (issue: Issue) => void;
  onStartAutoDev: (issueId: string, subRequirementId?: string) => void;
  onMoveColumn: (issueId: string, newStatus: IssueStatus) => void;
  onOpenCreateIssue: () => void;
  lang?: Language;
  themeStyle?: ThemeStyle;
}

export const KanbanBoard: React.FC<KanbanBoardProps> = ({
  issues,
  gitRepos,
  branchPrefixConfig,
  onSelectIssue,
  onStartAutoDev,
  onMoveColumn,
  onOpenCreateIssue,
  lang = 'en',
  themeStyle = 'glass',
}) => {
  const t = getTranslation(lang);
  const themeConfig = THEME_CONFIGS[themeStyle] || THEME_CONFIGS.glass;
  const isLight = themeConfig.isLight;

  const columns: { id: IssueStatus; title: string; subtitle: string; icon: any; accentColor: string }[] = [
    {
      id: 'requirements',
      title: t.colRequirements,
      subtitle: lang === 'zh' ? '归档与沟通中的原始需求，可随时生成开发 Spec' : 'Idea pool for AI discussion and Spec extraction',
      icon: ListTodo,
      accentColor: isLight
        ? 'border-cyan-300 text-cyan-800 bg-cyan-50'
        : 'border-cyan-500/30 text-cyan-300 bg-cyan-500/10',
    },
    {
      id: 'backlog',
      title: t.colBacklog,
      subtitle: lang === 'zh' ? '已关联 Git 本地工程并挂载规格文档' : 'Ready for execution with target branch spec',
      icon: Clock,
      accentColor: isLight
        ? 'border-amber-300 text-amber-800 bg-amber-50'
        : 'border-amber-500/30 text-amber-300 bg-amber-500/10',
    },
    {
      id: 'in_progress',
      title: t.colInProgress,
      subtitle: lang === 'zh' ? 'Auto-Dev Agent 正在按文档修改源码并自动构建' : 'Autonomous VibeBot modifying code & testing',
      icon: Play,
      accentColor: isLight
        ? 'border-indigo-300 text-indigo-800 bg-indigo-50'
        : 'border-indigo-500/30 text-indigo-300 bg-indigo-500/10',
    },
    {
      id: 'in_review',
      title: t.colInReview,
      subtitle: lang === 'zh' ? '提交 PR 并通过自动化测试套件' : 'Automated test suite passed, pending code review',
      icon: GitPullRequest,
      accentColor: isLight
        ? 'border-purple-300 text-purple-800 bg-purple-50'
        : 'border-purple-500/30 text-purple-300 bg-purple-500/10',
    },
    {
      id: 'completed',
      title: t.colCompleted,
      subtitle: lang === 'zh' ? '开发人员评审批准且代码已合并至 main' : 'Code approved and merged into target repository',
      icon: CheckCircle2,
      accentColor: isLight
        ? 'border-emerald-300 text-emerald-800 bg-emerald-50'
        : 'border-emerald-500/30 text-emerald-300 bg-emerald-500/10',
    },
  ];

  return (
    <div className="flex-1 overflow-x-auto p-6">
      <div className="grid grid-cols-1 md:grid-cols-3 xl:grid-cols-5 gap-5 min-w-[1400px] h-full items-start">
        {columns.map((col) => {
          const colIssues = issues.filter((i) => i.status === col.id);
          const IconComponent = col.icon;

          return (
            <div
              key={col.id}
              className={`${themeConfig.columnBg} backdrop-blur-xl border rounded-2xl p-4 flex flex-col max-h-[82vh] h-full shadow-lg transition-colors`}
            >
              {/* Column Header */}
              <div className={`pb-3 mb-3 border-b ${themeConfig.subtleBorder} flex items-center justify-between gap-2 shrink-0`}>
                <div className="flex items-center gap-2 overflow-hidden">
                  <div className={`p-1.5 rounded-lg border ${col.accentColor}`}>
                    <IconComponent className="w-4 h-4" />
                  </div>
                  <div>
                    <h3 className={`font-bold text-sm ${themeConfig.textPrimary} flex items-center gap-2`}>
                      {col.title}
                      <span className={`px-2 py-0.5 rounded-full text-[11px] font-mono border font-semibold ${
                        isLight
                          ? 'bg-slate-200/80 text-slate-800 border-slate-300'
                          : 'bg-white/10 text-white/80 border-white/10'
                      }`}>
                        {colIssues.length}
                      </span>
                    </h3>
                  </div>
                </div>

                {(col.id === 'requirements' || col.id === 'backlog') && (
                  <button
                    onClick={onOpenCreateIssue}
                    className={`p-1.5 rounded-lg border transition-colors flex items-center gap-1 shrink-0 ${
                      isLight
                        ? 'bg-slate-100 hover:bg-slate-200 border-slate-300 text-slate-700'
                        : 'bg-white/10 hover:bg-white/20 border-white/10 text-white/80'
                    }`}
                    title={t.newIssue}
                  >
                    <Plus className="w-4 h-4" />
                  </button>
                )}
              </div>

              {/* Column Issues List */}
              <div className="flex-1 overflow-y-auto space-y-3 pr-1">
                {colIssues.length === 0 ? (
                  <div className={`py-12 text-center text-xs border border-dashed rounded-xl font-sans ${
                    isLight
                      ? 'text-slate-400 border-slate-300 bg-slate-50'
                      : 'text-white/30 border-white/10 bg-black/10'
                  }`}>
                    {lang === 'zh' ? `暂无 ${col.title} 的 Issue` : `No issues in ${col.title}`}
                  </div>
                ) : (
                  colIssues.map((issue) => (
                    <IssueCard
                      key={issue.id}
                      issue={issue}
                      gitRepos={gitRepos}
                      branchPrefixConfig={branchPrefixConfig}
                      onClick={() => onSelectIssue(issue)}
                      onStartAutoDev={onStartAutoDev}
                      onMoveColumn={onMoveColumn}
                      lang={lang}
                      themeStyle={themeStyle}
                    />
                  ))
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};


