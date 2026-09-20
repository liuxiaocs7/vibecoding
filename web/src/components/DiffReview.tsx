import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { AlertTriangle, CheckCircle2, FileCode2, GitBranch, Loader2, MessageSquarePlus, SkipForward, Trash2, XCircle } from 'lucide-react';
import { api } from '../lib/api';
import { isCommentableDiffLine, parseUnifiedDiff, sumDiffStats } from '../lib/diffFormat';
import { truncateQuote } from '../lib/reviewComments';
import { Language, TRANSLATIONS } from '../lib/i18n';
import { DiffComment, DiffFile, Issue, IssueDiff, QualityGate } from '../types';

interface DiffReviewProps {
  issueId: string;
  issue?: Issue;
  lang?: Language;
  className?: string;
  canComment?: boolean;
  canRebase?: boolean;
  compareBase?: string;
  onCommentsChange?: (comments: DiffComment[]) => void;
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

function lineNum(line: { kind: string; oldLine?: number; newLine?: number }) {
  if (line.kind === 'del') return line.oldLine;
  return line.newLine ?? line.oldLine;
}

export const DiffReview: React.FC<DiffReviewProps> = ({
  issueId,
  issue,
  lang = 'zh' as Language,
  className,
  canComment = false,
  canRebase = false,
  compareBase,
  onCommentsChange,
}) => {
  const t = TRANSLATIONS[lang];
  const [data, setData] = useState<IssueDiff | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<{ repoId: string; file: DiffFile } | null>(null);
  const [editorMsg, setEditorMsg] = useState('');
  const [anchor, setAnchor] = useState<number | null>(null);
  const [head, setHead] = useState<number | null>(null);
  const [dragging, setDragging] = useState(false);
  const [draft, setDraft] = useState('');
  const [rebasing, setRebasing] = useState(false);

  const comments = issue?.reviewComments || [];

  const loadDiff = useCallback(() => {
    let cancelled = false;
    setLoading(true);
    setError('');
    api
      .getIssueDiff(issueId, compareBase || undefined)
      .then((d) => {
        if (cancelled) return;
        setData(d);
        setSelected((prev) => {
          const stillThere =
            prev &&
            (d.repos || []).some(
              (r) => r.repoId === prev.repoId && (r.files || []).some((f) => f.path === prev.file.path)
            );
          if (stillThere) return prev;
          const firstRepo = d.repos?.[0];
          const firstFile = firstRepo?.files?.[0];
          if (firstRepo && firstFile) {
            return { repoId: firstRepo.repoId, file: firstFile };
          }
          return null;
        });
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
  }, [issueId, compareBase]);

  useEffect(() => loadDiff(), [loadDiff]);

  useEffect(() => {
    const up = () => setDragging(false);
    window.addEventListener('mouseup', up);
    return () => window.removeEventListener('mouseup', up);
  }, []);

  useEffect(() => {
    setAnchor(null);
    setHead(null);
    setDraft('');
  }, [selected?.repoId, selected?.file.path]);

  const totals = useMemo(() => sumDiffStats(data?.repos || []), [data]);
  const patchLines = useMemo(() => parseUnifiedDiff(selected?.file.patch || ''), [selected]);
  const quality: QualityGate | undefined = data?.quality;

  const sel = useMemo(() => {
    if (anchor == null || head == null) return null;
    return { a: Math.min(anchor, head), b: Math.max(anchor, head) };
  }, [anchor, head]);

  const fileComments = useMemo(
    () =>
      comments.filter(
        (c) => selected && c.repoId === selected.repoId && c.path === selected.file.path
      ),
    [comments, selected]
  );

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

  const maxBehind = Math.max(0, ...(data.repos || []).map((r) => r.behind || 0));
  const doRebase = async () => {
    setRebasing(true);
    setEditorMsg('');
    try {
      await api.rebaseIssue(issueId);
      setEditorMsg(t.rebaseOk);
      loadDiff();
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      setEditorMsg(`${msg}. ${t.rebaseNeedEditor}`);
    } finally {
      setRebasing(false);
    }
  };

  const pickLine = (i: number, extend: boolean) => {
    if (!canComment || !isCommentableDiffLine(patchLines[i])) return;
    if (!extend || anchor == null) {
      setAnchor(i);
      setHead(i);
      return;
    }
    setHead(i);
  };

  const addComment = () => {
    if (!sel || !selected || !onCommentsChange || !draft.trim()) return;
    const slice = patchLines.slice(sel.a, sel.b + 1).filter(isCommentableDiffLine);
    if (!slice.length) return;
    const first = slice[0];
    const side: 'new' | 'old' = first.kind === 'del' ? 'old' : 'new';
    const nums = slice
      .map((l) => (side === 'old' ? l.oldLine : l.newLine))
      .filter((n): n is number => typeof n === 'number');
    if (!nums.length) return;
    const comment: DiffComment = {
      id: `cmt-${Date.now().toString(36)}`,
      repoId: selected.repoId,
      path: selected.file.path,
      side,
      startLine: Math.min(...nums),
      endLine: Math.max(...nums),
      quote: truncateQuote(slice.map((l) => l.text).join('\n')),
      body: draft.trim(),
      createdAt: new Date().toISOString(),
    };
    onCommentsChange([...comments, comment]);
    setDraft('');
    setAnchor(null);
    setHead(null);
  };

  const removeComment = (id: string) => {
    if (!onCommentsChange) return;
    onCommentsChange(comments.filter((c) => c.id !== id));
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
        {canRebase && maxBehind > 0 ? (
          <button
            type="button"
            disabled={rebasing}
            onClick={() => void doRebase()}
            className="px-2 py-0.5 rounded border border-amber-500/40 text-amber-800 dark:text-amber-200 hover:bg-amber-500/10 disabled:opacity-50"
          >
            {rebasing ? t.rebasing : t.rebaseOnto.replace('{base}', data.baseBranch || 'base')}
          </button>
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
      {canComment ? <p className="text-[11px] text-slate-500">{t.diffCommentHint}</p> : null}

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
                const n = comments.filter((c) => c.repoId === repo.repoId && c.path === f.path).length;
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
                    {n > 0 ? (
                      <span className="text-[10px] px-1 rounded bg-amber-500/20 text-amber-800 dark:text-amber-200">{n}</span>
                    ) : null}
                    <span className="ml-auto font-mono text-[10px] text-slate-500">{f.status}</span>
                  </button>
                );
              })}
            </div>
          ))}
        </div>
        <div className="border rounded-xl overflow-hidden max-h-[420px] flex flex-col bg-black/[0.03] dark:bg-white/[0.03]">
          <pre className="p-3 overflow-auto flex-1 text-[11px] font-mono leading-5 select-none">
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
            {patchLines.map((l, i) => {
              const active = sel && i >= sel.a && i <= sel.b && isCommentableDiffLine(l);
              const num = lineNum(l);
              const marked = fileComments.some((c) => {
                const sideNum = c.side === 'old' ? l.oldLine : l.newLine;
                return typeof sideNum === 'number' && sideNum >= c.startLine && sideNum <= c.endLine;
              });
              return (
                <div
                  key={i}
                  onMouseDown={(e) => {
                    if (!canComment) return;
                    e.preventDefault();
                    setDragging(true);
                    pickLine(i, false);
                  }}
                  onMouseEnter={() => {
                    if (dragging) pickLine(i, true);
                  }}
                  className={`flex gap-2 px-1 rounded ${
                    l.kind === 'add'
                      ? 'bg-emerald-500/15 text-emerald-800 dark:text-emerald-200'
                      : l.kind === 'del'
                        ? 'bg-rose-500/15 text-rose-800 dark:text-rose-200'
                        : l.kind === 'hunk' || l.kind === 'meta'
                          ? 'text-slate-500'
                          : ''
                  } ${active ? 'ring-1 ring-amber-400 bg-amber-400/20' : ''} ${
                    canComment && isCommentableDiffLine(l) ? 'cursor-text' : ''
                  } ${marked && !active ? 'border-l-2 border-amber-500' : ''}`}
                >
                  <span className="w-8 shrink-0 text-right text-slate-400 tabular-nums">
                    {typeof num === 'number' ? num : ''}
                  </span>
                  <span className="whitespace-pre-wrap break-all">{l.text || ' '}</span>
                </div>
              );
            })}
          </pre>
          {canComment && sel ? (
            <div className="border-t p-2 space-y-2 bg-amber-500/10">
              <textarea
                rows={2}
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                placeholder={t.diffCommentPlaceholder}
                className="w-full text-xs p-2 rounded-lg border border-amber-500/40 bg-white/80 dark:bg-black/30"
              />
              <div className="flex justify-end gap-2">
                <button
                  type="button"
                  onClick={() => {
                    setAnchor(null);
                    setHead(null);
                    setDraft('');
                  }}
                  className="px-2 py-1 text-[11px] rounded border border-black/10 dark:border-white/10"
                >
                  {t.cancel}
                </button>
                <button
                  type="button"
                  disabled={!draft.trim()}
                  onClick={addComment}
                  className="px-3 py-1 text-[11px] rounded bg-amber-600 text-white disabled:opacity-40 flex items-center gap-1"
                >
                  <MessageSquarePlus className="w-3 h-3" />
                  {t.diffCommentAdd}
                </button>
              </div>
            </div>
          ) : null}
        </div>
      </div>

      {comments.length > 0 ? (
        <div className="space-y-2">
          <div className="text-xs font-semibold text-slate-500">{t.diffCommentsTitle}</div>
          {comments.map((c) => {
            const repoName = data.repos.find((r) => r.repoId === c.repoId)?.repoName || c.repoId;
            return (
              <div
                key={c.id}
                className="text-xs p-2 rounded-lg border border-amber-500/30 bg-amber-500/5 flex gap-2"
              >
                <div className="flex-1 min-w-0">
                  <div className="font-mono text-[10px] text-slate-500 truncate">
                    [{repoName}] {c.path}:{c.side}:{c.startLine}-{c.endLine}
                  </div>
                  {c.quote ? (
                    <pre className="mt-1 text-[10px] whitespace-pre-wrap text-slate-500 max-h-16 overflow-auto">{c.quote}</pre>
                  ) : null}
                  <p className="mt-1">{c.body}</p>
                </div>
                {canComment ? (
                  <button
                    type="button"
                    onClick={() => removeComment(c.id)}
                    className="shrink-0 p-1 text-slate-400 hover:text-rose-500"
                    title={t.diffCommentRemove}
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                ) : null}
              </div>
            );
          })}
        </div>
      ) : null}

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
