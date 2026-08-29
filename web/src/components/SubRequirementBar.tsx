import React from 'react';
import { Issue } from '../types';
import { Language, ThemeStyle } from '../lib/i18n';
import { THEME_CONFIGS } from '../lib/theme';
import { Layers } from 'lucide-react';

interface SubRequirementBarProps {
  issue: Issue;
  selectedScope: string;
  onSelect: (scope: string) => void;
  lang: Language;
  themeStyle: ThemeStyle;
  compact?: boolean;
}

const statusDot: Record<string, string> = {
  pending: 'bg-amber-400',
  ready: 'bg-emerald-400',
  in_progress: 'bg-indigo-400 animate-pulse',
  done: 'bg-emerald-500',
  failed: 'bg-rose-500',
};

export const SubRequirementBar: React.FC<SubRequirementBarProps> = ({
  issue,
  selectedScope,
  onSelect,
  lang,
  themeStyle,
  compact,
}) => {
  const subs = issue.subRequirements || [];
  if (subs.length === 0) return null;
  const themeConfig = THEME_CONFIGS[themeStyle] || THEME_CONFIGS.glass;

  return (
    <div className={`rounded-xl border ${themeConfig.cardBg} ${themeConfig.subtleBorder} ${compact ? 'p-2.5' : 'p-3'}`}>
      <div className="flex items-center justify-between gap-2 mb-2">
        <span className={`flex items-center gap-1.5 text-[11px] font-bold ${themeConfig.textPrimary}`}>
          <Layers className={`w-3.5 h-3.5 ${themeConfig.textMuted}`} />
          {lang === 'zh' ? '子需求' : 'Sub-requirements'}
          <span className={`font-mono font-medium ${themeConfig.textMuted}`}>{subs.length}</span>
        </span>
        <button
          type="button"
          onClick={() => onSelect('all')}
          className={`shrink-0 px-2.5 py-1 rounded-lg border text-[11px] font-semibold transition-colors ${
            selectedScope === 'all'
              ? 'border-indigo-500/60 bg-indigo-500/15 text-indigo-800 dark:text-indigo-200'
              : `${themeConfig.inputBg} ${themeConfig.inputBorder} ${themeConfig.textSecondary}`
          }`}
        >
          {lang === 'zh' ? '全部 / 全局' : 'All / Global'}
        </button>
      </div>
      <div className={`grid gap-2 ${compact ? 'grid-cols-2 sm:grid-cols-3' : 'grid-cols-2 sm:grid-cols-3'}`}>
        {subs.map((sub) => {
          const active = selectedScope === sub.id;
          return (
            <button
              type="button"
              key={sub.id}
              onClick={() => onSelect(sub.id)}
              title={sub.description || sub.title}
              className={`text-left rounded-xl border px-2.5 py-2 flex items-start gap-2 transition-colors min-h-[52px] ${
                active
                  ? 'border-indigo-500/70 bg-indigo-500/15 shadow-sm'
                  : `${themeConfig.inputBg} ${themeConfig.inputBorder} hover:border-indigo-400/40`
              }`}
            >
              <span
                className={`w-6 h-6 rounded-md shrink-0 flex items-center justify-center text-[11px] font-bold ${
                  active
                    ? 'bg-indigo-600 text-white'
                    : 'bg-black/5 dark:bg-white/10 ' + themeConfig.textSecondary
                }`}
              >
                {sub.order}
              </span>
              <span className="min-w-0 flex-1">
                <span className={`block text-[11px] font-semibold leading-snug line-clamp-2 ${themeConfig.textPrimary}`}>
                  {sub.title}
                </span>
                <span className="mt-1 flex items-center gap-1">
                  <span className={`w-1.5 h-1.5 rounded-full ${statusDot[sub.status] || 'bg-slate-400'}`} />
                  <span className={`text-[10px] ${themeConfig.textMuted}`}>
                    {sub.status === 'done'
                      ? lang === 'zh'
                        ? '已完成'
                        : 'Done'
                      : sub.status === 'in_progress'
                      ? lang === 'zh'
                        ? '开发中'
                        : 'In progress'
                      : sub.status === 'failed'
                      ? lang === 'zh'
                        ? '失败'
                        : 'Failed'
                      : sub.devSpec?.rawMarkdown
                      ? lang === 'zh'
                        ? '文档就绪'
                        : 'Spec ready'
                      : lang === 'zh'
                      ? '待完善'
                      : 'Pending'}
                  </span>
                </span>
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
};
