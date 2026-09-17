import React from 'react';
import { PendingLLMSession } from '../../types';
import { Language, getTranslation, ThemeStyle } from '../../lib/i18n';
import { THEME_CONFIGS } from '../../lib/theme';
import { RotateCcw, RefreshCw, AlertTriangle } from 'lucide-react';

interface LLMRecoveryBarProps {
  session: PendingLLMSession | null;
  lang: Language;
  themeStyle: ThemeStyle;
  busy?: boolean;
  onRetrySession: () => void;
  onRegenerate: () => void;
}

export const LLMRecoveryBar: React.FC<LLMRecoveryBarProps> = ({
  session,
  lang,
  themeStyle,
  busy,
  onRetrySession,
  onRegenerate,
}) => {
  if (!session?.prompt || busy) return null;
  const t = getTranslation(lang);
  const themeConfig = THEME_CONFIGS[themeStyle] || THEME_CONFIGS.light;
  return (
    <div className="mx-4 mb-3 p-3 rounded-xl border border-amber-500/40 bg-amber-500/10 space-y-2">
      <div className="flex items-start gap-2 text-[11px] text-amber-800 dark:text-amber-200">
        <AlertTriangle className="w-3.5 h-3.5 shrink-0 mt-0.5 text-amber-500" />
        <div className="min-w-0">
          <div className="font-semibold">{t.llmRetryGiveUp}</div>
          {session.error && (
            <div className={`mt-1 break-all ${themeConfig.textMuted}`}>{session.error}</div>
          )}
        </div>
      </div>
      <div className="flex flex-wrap gap-2">
        <button
          type="button"
          disabled={busy}
          onClick={onRetrySession}
          className="px-3 py-1.5 rounded-lg text-[11px] font-semibold bg-indigo-600 hover:bg-indigo-500 text-white disabled:opacity-50 flex items-center gap-1.5"
        >
          <RotateCcw className="w-3 h-3" />
          {t.llmRetrySession}
        </button>
        <button
          type="button"
          disabled={busy}
          onClick={onRegenerate}
          className={`px-3 py-1.5 rounded-lg text-[11px] font-semibold border disabled:opacity-50 flex items-center gap-1.5 ${themeConfig.btnSecondary} ${themeConfig.btnSecondaryText}`}
        >
          <RefreshCw className="w-3 h-3" />
          {t.llmRegenerate}
        </button>
      </div>
    </div>
  );
};
