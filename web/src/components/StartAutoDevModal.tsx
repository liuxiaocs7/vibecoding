import React, { useEffect, useState } from 'react';
import { ExecutorConfig, ExecutorProbe } from '../types';
import { Language, ThemeStyle, getTranslation } from '../lib/i18n';
import { THEME_CONFIGS } from '../lib/theme';
import { api } from '../lib/api';
import { ThemedSelect } from './ThemedSelect';
import { Cpu, Loader2, X } from 'lucide-react';

interface StartAutoDevModalProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: (cfg: ExecutorConfig) => void;
  lang: Language;
  themeStyle: ThemeStyle;
}

export const StartAutoDevModal: React.FC<StartAutoDevModalProps> = ({
  isOpen,
  onClose,
  onConfirm,
  lang,
  themeStyle,
}) => {
  const t = getTranslation(lang);
  const themeConfig = THEME_CONFIGS[themeStyle] || THEME_CONFIGS.glass;
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [probes, setProbes] = useState<ExecutorProbe[]>([]);
  const [globalCfg, setGlobalCfg] = useState<ExecutorConfig | null>(null);
  const [choice, setChoice] = useState('llm');
  const [customCommand, setCustomCommand] = useState('');
  const [customArgs, setCustomArgs] = useState('{prompt}');
  const [promptStdin, setPromptStdin] = useState(false);

  useEffect(() => {
    if (!isOpen) return;
    let cancelled = false;
    setLoading(true);
    setError('');
    (async () => {
      try {
        const [cfg, listed] = await Promise.all([api.getExecutor(), api.listExecutors()]);
        if (cancelled) return;
        setGlobalCfg(cfg);
        setProbes(listed.executors || []);
        if (cfg.type === 'agent') {
          setChoice(cfg.preset || 'claude');
          setCustomCommand(cfg.command || '');
          setCustomArgs((cfg.args || ['{prompt}']).join(' '));
          setPromptStdin(!!cfg.promptStdin);
        } else {
          setChoice('llm');
        }
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [isOpen]);

  if (!isOpen) return null;

  const probeByID = (id: string) => probes.find((p) => p.id === id || p.preset === id);
  const selectedProbe = probeByID(choice);
  const unavailable = !!selectedProbe && !selectedProbe.available && choice !== 'custom';

  const confirm = () => {
    if (unavailable) return;
    const base: ExecutorConfig = {
      type: choice === 'llm' ? 'llm' : 'agent',
      timeoutSec: globalCfg?.timeoutSec ?? 1800,
      maxHeal: globalCfg?.maxHeal ?? 2,
    };
    if (base.type === 'agent') {
      base.preset = choice;
      if (choice === 'custom') {
        base.command = customCommand.trim();
        base.args = customArgs.trim() ? customArgs.trim().split(/\s+/) : ['{prompt}'];
        base.promptStdin = promptStdin;
      }
    }
    onConfirm(base);
  };

  return (
    <div className="fixed inset-0 z-[80] flex items-center justify-center bg-black/50 p-4">
      <div className={`w-full max-w-md rounded-2xl border shadow-xl ${themeConfig.modalBg} ${themeConfig.cardBorder}`}>
        <div className={`flex items-center justify-between px-4 py-3 border-b ${themeConfig.subtleBorder}`}>
          <div className="flex items-center gap-2 text-sm font-semibold">
            <Cpu className="w-4 h-4 text-violet-500" />
            {t.startAutoDevPickTitle}
          </div>
          <button type="button" onClick={onClose} className={`p-1 rounded ${themeConfig.textSecondary}`}>
            <X className="w-4 h-4" />
          </button>
        </div>
        <div className="p-4 space-y-3 text-sm">
          <p className={`text-xs ${themeConfig.textMuted}`}>{t.startAutoDevPickHint}</p>
          {loading ? (
            <div className="flex items-center gap-2 text-xs text-slate-500">
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
              {lang === 'zh' ? '加载执行器…' : 'Loading executors…'}
            </div>
          ) : (
            <>
              <ThemedSelect
                value={choice}
                onChange={(e) => setChoice(e.target.value)}
                isLight={themeConfig.isLight}
                chevronClassName={themeConfig.textSecondary}
                className={`w-full p-2 border rounded-xl text-xs ${themeConfig.inputBg} ${themeConfig.inputText} ${themeConfig.inputBorder}`}
              >
                {(probes.length ? probes : [{ id: 'llm', name: 'LLM (VibeBot)', type: 'llm', available: true }]).map((p) => (
                  <option key={p.id} value={p.preset || p.id} disabled={!p.available && p.id !== 'custom'}>
                    {p.name}
                    {!p.available && p.id !== 'custom' ? (lang === 'zh' ? '（不可用）' : ' (unavailable)') : ''}
                    {(p.preset || p.id) === (globalCfg?.type === 'agent' ? globalCfg.preset : 'llm')
                      ? lang === 'zh'
                        ? ' · 默认'
                        : ' · default'
                      : ''}
                  </option>
                ))}
              </ThemedSelect>
              {choice === 'custom' ? (
                <div className="space-y-2">
                  <input
                    value={customCommand}
                    onChange={(e) => setCustomCommand(e.target.value)}
                    placeholder="command"
                    className={`w-full p-2 border rounded-xl text-xs font-mono ${themeConfig.inputBg} ${themeConfig.inputText} ${themeConfig.inputBorder}`}
                  />
                  <input
                    value={customArgs}
                    onChange={(e) => setCustomArgs(e.target.value)}
                    placeholder="{prompt}"
                    className={`w-full p-2 border rounded-xl text-xs font-mono ${themeConfig.inputBg} ${themeConfig.inputText} ${themeConfig.inputBorder}`}
                  />
                  <label className={`flex items-center gap-2 text-xs ${themeConfig.textSecondary}`}>
                    <input type="checkbox" checked={promptStdin} onChange={(e) => setPromptStdin(e.target.checked)} />
                    stdin
                  </label>
                </div>
              ) : null}
              {unavailable && selectedProbe?.hint ? (
                <p className="text-[11px] text-amber-700 dark:text-amber-300">{selectedProbe.hint}</p>
              ) : null}
              {error ? <p className="text-[11px] text-rose-600">{error}</p> : null}
            </>
          )}
        </div>
        <div className={`px-4 py-3 border-t flex justify-end gap-2 ${themeConfig.subtleBorder}`}>
          <button
            type="button"
            onClick={onClose}
            className={`px-3 py-1.5 text-xs rounded-xl border ${themeConfig.btnSecondary} ${themeConfig.btnSecondaryText}`}
          >
            {t.cancel}
          </button>
          <button
            type="button"
            disabled={loading || unavailable}
            onClick={confirm}
            className="px-4 py-1.5 text-xs rounded-xl bg-indigo-600 text-white font-semibold disabled:opacity-40"
          >
            {t.startAutoDev}
          </button>
        </div>
      </div>
    </div>
  );
};
