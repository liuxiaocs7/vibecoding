export type DiffLineKind = 'ctx' | 'add' | 'del' | 'hunk' | 'meta';

export interface DiffLine {
  kind: DiffLineKind;
  text: string;
}

/** Classify a unified-diff line for simple syntax coloring. */
export function classifyDiffLine(line: string): DiffLine {
  if (
    line.startsWith('+++') ||
    line.startsWith('---') ||
    line.startsWith('diff ') ||
    line.startsWith('index ')
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

export function parseUnifiedDiff(patch: string): DiffLine[] {
  if (!patch) return [];
  return patch.replace(/\r\n/g, '\n').split('\n').map(classifyDiffLine);
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
