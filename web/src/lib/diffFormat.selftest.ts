/**
 * Lightweight self-test for pure diff helpers (no Vitest).
 * Run: npx --yes tsx src/lib/diffFormat.selftest.ts
 */
import { classifyDiffLine, parseUnifiedDiff, sumDiffStats } from './diffFormat.ts';

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

const totals = sumDiffStats([
  { stats: { additions: 2, deletions: 1, filesChanged: 1 } },
  { stats: { additions: 3, deletions: 4, filesChanged: 2 } },
]);
assert(totals.additions === 5 && totals.deletions === 5 && totals.filesChanged === 3, 'sum stats');

console.log('diffFormat.selftest: ok');
