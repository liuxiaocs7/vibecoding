import React from 'react';
import { Issue } from '../../types';
import { Language, getTranslation, ThemeStyle } from '../../lib/i18n';
import { THEME_CONFIGS } from '../../lib/theme';
import { SubRequirementBar } from '../SubRequirementBar';
import { Terminal, Play } from 'lucide-react';

interface IssueConsoleTabProps {
  issue: Issue;
  splitIssue: boolean;
  selectedScope: string;
  setSelectedScope: (scope: string) => void;
  lang: Language;
  themeStyle: ThemeStyle;
  onStartAutoDev: (issueId: string, subRequirementId?: string) => void;
}

export const IssueConsoleTab: React.FC<IssueConsoleTabProps> = ({
  issue,
  splitIssue,
  selectedScope,
  setSelectedScope,
  lang,
  themeStyle,
  onStartAutoDev,
}) => {
  const themeConfig = THEME_CONFIGS[themeStyle] || THEME_CONFIGS.glass;
  const isLight = themeConfig.isLight;
  const t = getTranslation(lang);

  return (
    <div className="flex-1 p-6 flex flex-col gap-4 overflow-hidden">
      <div className={`p-4 rounded-xl border flex items-center justify-between ${themeConfig.cardBg} ${themeConfig.cardBorder}`}>
        <div>
          <div className={`text-xs font-bold flex items-center gap-2 ${themeConfig.textPrimary}`}>
            <Terminal className="w-4 h-4 text-indigo-500" />
            {t.vibeBotConsoleTitle}
          </div>
          <div className={`text-[11px] mt-0.5 ${themeConfig.textMuted}`}>
            {splitIssue
              ? t.sequentialDev
              : lang === 'zh'
              ? '根据待开发文档自动切换分支，编写补丁，运行测试并创建 PR'
              : 'Branch, patch, test, and open a local review from the Dev Spec'}
          </div>
        </div>

        <div className="flex items-center gap-3">
          <div className="text-right">
            <div className="text-xs font-bold text-indigo-600 dark:text-indigo-300 font-mono">
              {issue.autoDevProgress}%
            </div>
            <div className={`text-[10px] uppercase ${themeConfig.textMuted}`}>当前完成度</div>
          </div>

          {issue.status === 'backlog' && (
            <button
              onClick={() => onStartAutoDev(issue.id)}
              className="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white font-semibold text-xs rounded-xl shadow transition-all flex items-center gap-1.5"
            >
              <Play className="w-3.5 h-3.5 fill-current" />
              开始自治开发
            </button>
          )}
        </div>
      </div>

      {splitIssue && (
        <div className={`px-3 py-2 rounded-xl border ${themeConfig.cardBg} ${themeConfig.cardBorder}`}>
          <SubRequirementBar
            issue={issue}
            selectedScope={issue.currentSubId || selectedScope}
            onSelect={setSelectedScope}
            lang={lang}
            themeStyle={themeStyle}
            compact
          />
        </div>
      )}

      <div className={`w-full h-2 rounded-full overflow-hidden border ${themeConfig.inputBg} ${themeConfig.inputBorder}`}>
        <div
          className="bg-gradient-to-r from-indigo-500 to-purple-500 h-full transition-all duration-500"
          style={{ width: `${issue.autoDevProgress}%` }}
        />
      </div>

      <div className={`flex-1 border rounded-xl p-4 font-mono text-xs overflow-y-auto space-y-2 leading-relaxed backdrop-blur-md ${isLight ? 'bg-slate-900 text-slate-100 border-slate-800' : 'bg-black/60 text-slate-200 border-white/10'}`}>
        <div className="opacity-40 text-[11px]">{t.autoExecLogHeader}</div>
        {issue.autoDevLogs.length === 0 ? (
          <div className="opacity-40 italic py-8 text-center">
            准备就绪。点击【开始自治开发】触发后台工程构建与自动化改码流程...
          </div>
        ) : (
          issue.autoDevLogs.map((log) => (
            <div key={log.id} className="flex items-start gap-2">
              <span className="opacity-40 shrink-0">[{log.timestamp}]</span>
              <span
                className={`font-semibold shrink-0 uppercase px-1.5 py-0.5 rounded text-[10px] ${
                  log.phase === 'analyzing'
                    ? 'text-amber-300 bg-amber-500/20'
                    : log.phase === 'branching'
                    ? 'text-indigo-300 bg-indigo-500/20'
                    : log.phase === 'coding' || log.phase === 'agent'
                    ? 'text-purple-300 bg-purple-500/20'
                    : log.phase === 'testing'
                    ? 'text-cyan-300 bg-cyan-500/20'
                    : log.phase === 'completed'
                    ? 'text-emerald-300 bg-emerald-500/20'
                    : 'opacity-60'
                }`}
              >
                {lang === 'zh'
                  ? ({
                      analyzing: '分析中',
                      branching: '建分支',
                      coding: '编码中',
                      agent: 'Agent',
                      testing: '测试中',
                      linting: '检查中',
                      committing: '提交中',
                      completed: '已完成',
                      failed: '失败',
                    } as Record<string, string>)[log.phase] || log.phase
                  : log.phase}
              </span>
              <span>{log.message}</span>
            </div>
          ))
        )}
      </div>
    </div>
  );
};
