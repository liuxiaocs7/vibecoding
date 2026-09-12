/**
 * Lightweight self-test for pure diff helpers (no Vitest).
 * Run: npx --yes tsx src/lib/diffFormat.selftest.ts
 */
import { classifyDiffLine, parseUnifiedDiff, sumDiffStats } from './diffFormat.ts';
import { formatReviewComments, QUOTE_MAX_CHARS, truncateQuote } from './reviewComments.ts';

function assert(cond: unknown, msg: string) {
  if (!cond) throw new Error(msg);
}

assert(classifyDiffLine('+foo').kind === 'add', 'add line');
assert(classifyDiffLine('-bar').kind === 'del', 'del line');
assert(classifyDiffLine('@@ -1 +1 @@').kind === 'hunk', 'hunk line');
assert(classifyDiffLine('diff --git a/x b/x').kind === 'meta', 'meta line');
assert(classifyDiffLine(' context').kind === 'ctx', 'ctx line');

const lines = parseUnifiedDiff('--- a/f\n+++ b/f\n@@ -1 +1 @@\n-old\n+new');
assert(lines.length === 5, `expected 5 lines got ${lines.length}`);
assert(lines[3].kind === 'del' && lines[4].kind === 'add', 'del then add');
assert(lines[3].oldLine === 1 && lines[3].newLine === undefined, `del line numbers ${JSON.stringify(lines[3])}`);
assert(lines[4].newLine === 1 && lines[4].oldLine === undefined, `add line numbers ${JSON.stringify(lines[4])}`);

const hunk = parseUnifiedDiff(
  ['diff --git a/foo.go b/foo.go', '--- a/foo.go', '+++ b/foo.go', '@@ -10,3 +12,4 @@ func F() {', ' ctxA', '-old', '+new1', '+new2', ' ctxB'].join(
    '\n'
  )
);
const ctxA = hunk.find((l) => l.text === ' ctxA');
const del = hunk.find((l) => l.kind === 'del');
const add1 = hunk.find((l) => l.text === '+new1');
const add2 = hunk.find((l) => l.text === '+new2');
const ctxB = hunk.find((l) => l.text === ' ctxB');
assert(ctxA?.oldLine === 10 && ctxA?.newLine === 12, `ctxA ${JSON.stringify(ctxA)}`);
assert(del?.oldLine === 11, `del ${JSON.stringify(del)}`);
assert(add1?.newLine === 13 && add2?.newLine === 14, `adds ${JSON.stringify(add1)} ${JSON.stringify(add2)}`);
assert(ctxB?.oldLine === 12 && ctxB?.newLine === 15, `ctxB ${JSON.stringify(ctxB)}`);

const totals = sumDiffStats([
  { stats: { additions: 2, deletions: 1, filesChanged: 1 } },
  { stats: { additions: 3, deletions: 4, filesChanged: 2 } },
]);
assert(totals.additions === 5 && totals.deletions === 5 && totals.filesChanged === 3, 'sum stats');

assert(truncateQuote('short') === 'short', 'short quote');
const long = 'x'.repeat(QUOTE_MAX_CHARS + 20);
assert(truncateQuote(long).endsWith('…'), 'ellipsis');
assert([...truncateQuote(long)].length === QUOTE_MAX_CHARS + 1, 'truncated length');

const prompt = formatReviewComments(
  [
    {
      id: 'c1',
      repoId: 'repo-1',
      path: 'internal/foo.go',
      side: 'new',
      startLine: 42,
      endLine: 58,
      quote: '+timeout := 20 * time.Minute',
      body: 'map timeout errors to LLMRecoveryBar',
      createdAt: '2026-01-01T00:00:00Z',
    },
  ],
  (id) => (id === 'repo-1' ? 'ziya' : id)
);
assert(prompt.includes('[ziya] internal/foo.go:new:42-58'), prompt);
assert(prompt.includes('map timeout errors to LLMRecoveryBar'), prompt);
assert(formatReviewComments([]).length === 0, 'empty comments');

console.log('diffFormat.selftest: ok');
