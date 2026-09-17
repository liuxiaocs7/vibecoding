import React, { useEffect, useRef, useState } from 'react';
import { Issue } from '../../types';
import { Language, getTranslation, ThemeStyle } from '../../lib/i18n';
import { THEME_CONFIGS } from '../../lib/theme';
import { specReadyForDev } from '../../lib/subreq';
import { X, Play, Trash2, Minus } from 'lucide-react';

interface IssueDetailHeaderProps {
  issue: Issue;
  lang: Language;
  themeStyle: ThemeStyle;
  onClose: () => void;
  onMinimize?: () => void;
  analyzing?: boolean;
  onStartAutoDev: (issueId: string, subRequirementId?: string) => void;
  onDeleteIssue?: (issueId: string) => void;
  setActiveTab: (tab: 'chat' | 'spec' | 'console' | 'review') => void;
}

export const IssueDetailHeader: React.FC<IssueDetailHeaderProps> = ({
  issue,
  lang,
  themeStyle,
  onClose,
  onMinimize,
  analyzing,
  onStartAutoDev,
  onDeleteIssue,
  setActiveTab,
}) => {
  const themeConfig = THEME_CONFIGS[themeStyle] || THEME_CONFIGS.light;
  const t = getTranslation(lang);
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const deleteWrapRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setConfirmingDelete(false);
  }, [issue.id]);

  useEffect(() => {
    if (!confirmingDelete) return;
    const onPointerDown = (e: PointerEvent) => {
      if (deleteWrapRef.current?.contains(e.target as Node)) return;
      setConfirmingDelete(false);
    };
    document.addEventListener('pointerdown', onPointerDown, true);
    return () => document.removeEventListener('pointerdown', onPointerDown, true);
  }, [confirmingDelete]);

  return (
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
        {issue.status === 'backlog' && specReadyForDev(issue) && !confirmingDelete && (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              onStartAutoDev(issue.id);
              setActiveTab('console');
            }}
            className="px-3.5 py-1.5 bg-gradient-to-r from-indigo-600 to-purple-600 hover:from-indigo-500 hover:to-purple-500 text-white font-semibold text-xs rounded-xl shadow flex items-center gap-1.5 transition-all"
          >
            <Play className="w-3.5 h-3.5 fill-current" />
            启动自治开发
          </button>
        )}

        {onMinimize && (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              onMinimize();
            }}
            aria-label={analyzing ? t.minimizeWhileAnalyzing : t.minimizeIssue}
            className={`p-2 rounded-xl transition-colors ${themeConfig.textSecondary} hover:${themeConfig.textPrimary} hover:bg-black/5 dark:hover:bg-white/10`}
            title={analyzing ? t.minimizeWhileAnalyzing : t.minimizeIssue}
          >
            <Minus className="w-5 h-5" />
          </button>
        )}
        {onDeleteIssue && (
          <div className="flex items-center gap-1.5" ref={deleteWrapRef}>
            {confirmingDelete ? (
              <>
                <span className={`text-[11px] font-medium whitespace-nowrap ${themeConfig.textPrimary}`}>
                  {t.deleteIssueConfirm}
                </span>
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    setConfirmingDelete(false);
                  }}
                  className={`px-2.5 py-1 rounded-lg text-[11px] font-semibold ${themeConfig.textSecondary} hover:bg-black/5 dark:hover:bg-white/10`}
                >
                  {t.cancel}
                </button>
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    setConfirmingDelete(false);
                    onDeleteIssue(issue.id);
                  }}
                  className="px-2.5 py-1 rounded-lg text-[11px] font-bold text-white bg-rose-500 hover:bg-rose-600"
                >
                  {t.confirm}
                </button>
              </>
            ) : (
              <button
                type="button"
                onClick={(e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  setConfirmingDelete(true);
                }}
                className="p-2 rounded-xl transition-colors text-rose-400 hover:bg-rose-500/10"
                title={t.deleteIssueTitle}
              >
                <Trash2 className="w-4 h-4" />
              </button>
            )}
          </div>
        )}
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onClose();
          }}
          aria-label={analyzing ? t.closeWhileAnalyzing : t.closeIssue}
          className={`p-2 rounded-xl transition-colors ${themeConfig.textSecondary} hover:${themeConfig.textPrimary} hover:bg-black/5 dark:hover:bg-white/10`}
          title={analyzing ? t.closeWhileAnalyzing : t.closeIssue}
        >
          <X className="w-5 h-5" />
        </button>
      </div>
    </div>
  );
};
