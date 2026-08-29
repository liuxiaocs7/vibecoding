import React from 'react';
import { Issue } from '../../types';
import { Language, getTranslation, ThemeStyle } from '../../lib/i18n';
import { specReadyForDev } from '../../lib/subreq';
import {
  MessageSquare,
  FileText,
  Sparkles,
  Play,
  Terminal,
  Square,
  GitPullRequest,
  ShieldCheck,
  CheckCircle2,
  Layers,
} from 'lucide-react';

interface StatusFocusBannerProps {
  issue: Issue;
  splitIssue: boolean;
  associatedRepos: { id: string }[];
  lang: Language;
  setActiveTab: (tab: 'chat' | 'spec' | 'console' | 'review') => void;
  handleSendMessage: (
    customPrompt?: string,
    opts?: { forceSpecSync?: boolean; split?: boolean; scope?: string }
  ) => Promise<void>;
  onUpdateIssue: (updatedIssue: Issue) => void;
  onStartAutoDev: (issueId: string, subRequirementId?: string) => void;
  onCancelAutoDev?: (issueId: string) => void;
  handleApproveMerge: () => Promise<void>;
}

export const StatusFocusBanner: React.FC<StatusFocusBannerProps> = ({
  issue,
  splitIssue,
  associatedRepos,
  lang,
  setActiveTab,
  handleSendMessage,
  onUpdateIssue,
  onStartAutoDev,
  onCancelAutoDev,
  handleApproveMerge,
}) => {
  const t = getTranslation(lang);

  return (
    <>
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
    </>
  );
};
