import React, { useEffect, useMemo, useState } from 'react';
import { AlertTriangle, CheckCircle2, FileCode2, GitBranch, Loader2, SkipForward, XCircle } from 'lucide-react';
import { api } from '../lib/api';
import { parseUnifiedDiff, sumDiffStats } from '../lib/diffFormat';
import { Language, TRANSLATIONS } from '../lib/i18n';
import { DiffFile, IssueDiff, QualityGate } from '../types';

interface DiffReviewProps {
  issueId: string;
  lang?: Language;
  className?: string;
}

function gateLabel(ran: boolean, passed: boolean, lang: Language) {
  const t = TRANSLATIONS[lang];
  if (!ran) return t.gateSkipped;
  return passed ? t.gatePassed : t.gateFailed;
}

function GateCell({
  title,
  ran,
  passed,
  output,
  lang,
}: {
  title: string;
  ran: boolean;
  passed: boolean;
  output?: string;
  lang: Language;
}) {
  const [open, setOpen] = useState(false);
  const ok = ran && passed;
  const Icon = !ran ? SkipForward : ok ? CheckCircle2 : XCircle;
  const color = !ran ? 'text-slate-500' : ok ? 'text-emerald-600' : 'text-rose-600';
  return (
    <div className="p-3 rounded-xl border border-black/10 dark:border-white/10 space-y-1">
      <div className="text-[10px] uppercase font-semibold text-slate-500">{title}</div>
      <div className={`font-bold font-mono text-sm flex items-center gap-1 ${color}`}>
        <Icon className="w-4 h-4" /> {gateLabel(ran, passed, lang)}
      </div>
      {ran && !passed && output ? (
        <button type="button" className="text-[10px] underline text-rose-600" onClick={() => setOpen((v) => !v)}>
          {open ? (lang === 'zh' ? '收起输出' : 'Hide output') : lang === 'zh' ? '展开输出' : 'Show output'}
        </button>
      ) : null}
      {open && output ? (
        <pre className="mt-1 max-h-40 overflow-auto text-[10px] whitespace-pre-wrap bg-black/5 dark:bg-white/5 p-2 rounded">
          {output}
        </pre>
      ) : null}
    </div>
  );
}

export const DiffReview: React.FC<DiffReviewProps> = ({ issueId, lang = 'zh' as Language, className }) => {
  const t = TRANSLATIONS[lang];
  const [data, setData] = useState<IssueDiff | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<{ repoId: string; file: DiffFile } | null>(null);
  const [editorMsg, setEditorMsg] = useState('');

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError('');
    api
      .getIssueDiff(issueId)
      .then((d) => {
        if (cancelled) return;
        setData(d);
        const firstRepo = d.repos?.[0];
        const firstFile = firstRepo?.files?.[0];
        if (firstRepo && firstFile) {
          setSelected({ repoId: firstRepo.repoId, file: firstFile });
        }
      })
      .catch((e: { message?: string }) => {
        if (!cancelled) setError(e?.message || String(e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [issueId]);

  const totals = useMemo(() => sumDiffStats(data?.repos || []), [data]);
  const patchLines = useMemo(() => parseUnifiedDiff(selected?.file.patch || ''), [selected]);
  const quality: QualityGate | undefined = data?.quality;

  if (loading) {
    return (
      <div className={`flex items-center gap-2 text-sm text-slate-500 ${className || ''}`}>
        <Loader2 className="w-4 h-4 animate-spin" />
        {t.diffLoading}
      </div>
    );
  }
  if (error) {
    return (
      <div className={`p-4 rounded-xl border border-amber-500/30 bg-amber-500/10 text-sm ${className || ''}`}>
        <div className="flex items-center gap-2 font-semibold text-amber-800 dark:text-amber-200">
          <AlertTriangle className="w-4 h-4" />
          {t.diffLoadFailed}
        </div>
        <p className="mt-1 text-xs opacity-80">{error}</p>
      </div>
    );
  }
  if (!data) return null;

  const empty = totals.filesChanged === 0;
  const openEditor = async (app: 'cursor' | 'vscode') => {
    setEditorMsg('');
    try {
      const res = await api.openEditor(issueId, app, selected?.repoId);
      setEditorMsg(`${app}: ${res.path}`);
    } catch (e: unknown) {
      setEditorMsg(e instanceof Error ? e.message : String(e));
    }
  };

  return (
    <div className={`space-y-4 ${className || ''}`}>
      <div className="flex flex-wrap items-center gap-3 text-xs">
        <span className="inline-flex items-center gap-1.5 font-mono">
          <GitBranch className="w-3.5 h-3.5" />
          {data.baseBranch || 'base'}…{data.branchName}
        </span>
        {data.executor ? (
          <span className="px-2 py-0.5 rounded bg-indigo-500/15 text-indigo-700 dark:text-indigo-300">
            {t.executorLabel}: {data.executor}
          </span>
        ) : null}
        <span className="font-mono text-emerald-600">+{totals.additions}</span>
        <span className="font-mono text-rose-600">-{totals.deletions}</span>
        <span className="text-slate-500">
          {totals.filesChanged} {t.diffFiles}
        </span>
        {data.repos?.[0] ? (
          <span className="text-slate-500">
            ahead {data.repos[0].ahead} / behind {data.repos[0].behind}
          </span>
        ) : null}
        <button
          type="button"
          onClick={() => openEditor('cursor')}
          className="px-2 py-0.5 rounded border border-black/10 dark:border-white/10 hover:bg-black/5 dark:hover:bg-white/5"
        >
          {t.openInCursor}
        </button>
        <button
          type="button"
          onClick={() => openEditor('vscode')}
          className="px-2 py-0.5 rounded border border-black/10 dark:border-white/10 hover:bg-black/5 dark:hover:bg-white/5"
        >
          {t.openInVSCode}
        </button>
      </div>
      {editorMsg ? <p className="text-[11px] text-slate-500 font-mono">{editorMsg}</p> : null}

      {empty ? <p className="text-xs text-slate-500">{t.diffEmpty}</p> : null}

      <div className="grid grid-cols-1 lg:grid-cols-[240px_1fr] gap-3 min-h-[280px]">
        <div className="border rounded-xl overflow-auto max-h-[420px] text-xs">
          {(data.repos || []).map((repo) => (
            <div key={repo.repoId} className="border-b last:border-b-0">
              <div className="px-3 py-2 font-semibold bg-black/5 dark:bg-white/5 sticky top-0">
                {repo.repoName}
                {repo.error ? <span className="ml-2 text-rose-600 font-normal">{repo.error}</span> : null}
              </div>
              {(repo.files || []).map((f) => {
                const active = selected?.repoId === repo.repoId && selected.file.path === f.path;
                return (
                  <button
                    key={f.path}
                    type="button"
                    onClick={() => setSelected({ repoId: repo.repoId, file: f })}
                    className={`w-full text-left px-3 py-1.5 flex items-center gap-2 hover:bg-indigo-500/10 ${
                      active ? 'bg-indigo-500/15' : ''
                    }`}
                  >
                    <FileCode2 className="w-3.5 h-3.5 shrink-0 opacity-60" />
                    <span className="truncate font-mono">{f.path}</span>
                    <span className="ml-auto font-mono text-[10px] text-slate-500">{f.status}</span>
                  </button>
                );
              })}
            </div>
          ))}
        </div>
        <pre className="border rounded-xl p-3 overflow-auto max-h-[420px] text-[11px] font-mono leading-5 bg-black/[0.03] dark:bg-white/[0.03]">
          {!selected?.file.patch ? (
            <span className="text-slate-500">
              {selected?.file.truncated
                ? lang === 'zh'
                  ? '补丁过大，已截断（仅保留统计）'
                  : 'Patch truncated (stats only)'
                : lang === 'zh'
                  ? '选择左侧文件查看 unified diff'
                  : 'Select a file to view unified diff'}
            </span>
          ) : null}
          {patchLines.map((l, i) => (
            <div
              key={i}
              className={
                l.kind === 'add'
                  ? 'bg-emerald-500/15 text-emerald-800 dark:text-emerald-200'
                  : l.kind === 'del'
                    ? 'bg-rose-500/15 text-rose-800 dark:text-rose-200'
                    : l.kind === 'hunk' || l.kind === 'meta'
                      ? 'text-slate-500'
                      : ''
              }
            >
              {l.text || ' '}
            </div>
          ))}
        </pre>
      </div>

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 text-xs">
        <GateCell
          title={t.gateTests}
          ran={!!quality?.testsRan}
          passed={!!quality?.testsPassed}
          output={quality?.testsOutput}
          lang={lang}
        />
        <GateCell
          title={t.gateLint}
          ran={!!quality?.lintRan}
          passed={!!quality?.lintPassed}
          output={quality?.lintOutput}
          lang={lang}
        />
        <div className="p-3 rounded-xl border border-black/10 dark:border-white/10 space-y-1">
          <div className="text-[10px] uppercase font-semibold text-slate-500">{t.gateRepairRounds}</div>
          <div className="font-bold font-mono text-sm">{quality?.repairRounds ?? 0}</div>
        </div>
        <div className="p-3 rounded-xl border border-black/10 dark:border-white/10 space-y-1">
          <div className="text-[10px] uppercase font-semibold text-slate-500">{t.gateRealChanges}</div>
          <div className="font-bold font-mono text-sm">
            <span className="text-emerald-600">+{totals.additions}</span>
            {' / '}
            <span className="text-rose-600">-{totals.deletions}</span>
          </div>
        </div>
      </div>
    </div>
  );
};
