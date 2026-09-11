import React from 'react';
import { Language, getTranslation, ThemeStyle } from '../../lib/i18n';
import { THEME_CONFIGS } from '../../lib/theme';
import { Loader2, AlertTriangle, X, Maximize2 } from 'lucide-react';

interface IssueSessionChipProps {
  title: string;
  busy: boolean;
  error?: string;
  lang: Language;
  themeStyle: ThemeStyle;
  onRestore: () => void;
  onDismiss: () => void;
}

export const IssueSessionChip: React.FC<IssueSessionChipProps> = ({
  title,
  busy,
  error,
  lang,
  themeStyle,
  onRestore,
  onDismiss,
}) => {
  const t = getTranslation(lang);
  const themeConfig = THEME_CONFIGS[themeStyle] || THEME_CONFIGS.glass;
  return (
    <div
      className={`pointer-events-auto w-[280px] rounded-2xl border shadow-2xl backdrop-blur-md px-3 py-2.5 flex items-start gap-2 ${themeConfig.modalBg} ${themeConfig.cardBorder}`}
    >
      <button
        type="button"
        onClick={onRestore}
        className="flex-1 min-w-0 text-left flex items-start gap-2"
        title={t.restoreIssue}
      >
        <div className="mt-0.5 shrink-0">
          {busy ? (
            <Loader2 className="w-4 h-4 animate-spin text-indigo-500" />
          ) : error ? (
            <AlertTriangle className="w-4 h-4 text-amber-500" />
          ) : (
            <Maximize2 className="w-4 h-4 text-indigo-400" />
          )}
        </div>
        <div className="min-w-0">
          <div className={`text-xs font-semibold truncate ${themeConfig.textPrimary}`}>{title}</div>
          <div className={`text-[10px] mt-0.5 line-clamp-2 ${themeConfig.textMuted}`}>
            {busy ? t.analysisRunning : error ? error : t.issueMinimized}
          </div>
        </div>
      </button>
      <div className="flex items-center gap-0.5 shrink-0">
        <button
          type="button"
          onClick={onRestore}
          className={`p-1 rounded-lg ${themeConfig.textSecondary} hover:bg-black/5 dark:hover:bg-white/10`}
          title={t.restoreIssue}
        >
          <Maximize2 className="w-3.5 h-3.5" />
        </button>
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onDismiss();
          }}
          className={`p-1 rounded-lg ${themeConfig.textSecondary} hover:bg-rose-500/10 hover:text-rose-400`}
          title={busy ? t.stopAnalysis : t.closeIssue}
        >
          <X className="w-3.5 h-3.5" />
        </button>
      </div>
    </div>
  );
};
