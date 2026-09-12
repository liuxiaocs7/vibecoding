export type DiffLineKind = 'ctx' | 'add' | 'del' | 'hunk' | 'meta';

export interface DiffLine {
  kind: DiffLineKind;
  text: string;
  oldLine?: number;
  newLine?: number;
}

const HUNK_RE = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/;

/** Classify a unified-diff line for simple syntax coloring. */
export function classifyDiffLine(line: string): DiffLine {
  if (
    line.startsWith('+++') ||
    line.startsWith('---') ||
    line.startsWith('diff ') ||
    line.startsWith('index ') ||
    line.startsWith('\\')
  ) {
    return { kind: 'meta', text: line };
  }
  if (line.startsWith('@@')) {
    return { kind: 'hunk', text: line };
  }
  if (line.startsWith('+')) {
    return { kind: 'add', text: line };
  }
  if (line.startsWith('-')) {
    return { kind: 'del', text: line };
  }
  return { kind: 'ctx', text: line };
}

/** Parse a unified diff and attach old/new file line numbers from hunk headers. */
export function parseUnifiedDiff(patch: string): DiffLine[] {
  if (!patch) return [];
  const lines = patch.replace(/\r\n/g, '\n').split('\n').map(classifyDiffLine);
  let oldLine = 0;
  let newLine = 0;
  for (const line of lines) {
    if (line.kind === 'hunk') {
      const m = line.text.match(HUNK_RE);
      if (m) {
        oldLine = Number(m[1]);
        newLine = Number(m[2]);
      }
      continue;
    }
    if (line.kind === 'meta') continue;
    if (line.kind === 'del') {
      line.oldLine = oldLine;
      oldLine += 1;
      continue;
    }
    if (line.kind === 'add') {
      line.newLine = newLine;
      newLine += 1;
      continue;
    }
    line.oldLine = oldLine;
    line.newLine = newLine;
    oldLine += 1;
    newLine += 1;
  }
  return lines;
}

export function isCommentableDiffLine(line: DiffLine): boolean {
  return line.kind === 'add' || line.kind === 'del' || line.kind === 'ctx';
}

export function sumDiffStats(
  repos: { stats: { additions: number; deletions: number; filesChanged: number } }[]
) {
  return repos.reduce(
    (acc, r) => ({
      additions: acc.additions + (r.stats?.additions || 0),
      deletions: acc.deletions + (r.stats?.deletions || 0),
      filesChanged: acc.filesChanged + (r.stats?.filesChanged || 0),
    }),
    { additions: 0, deletions: 0, filesChanged: 0 }
  );
}
