import type { BranchPrefixConfig, Issue, IssueKind } from '../types';

export const ISSUE_KINDS: IssueKind[] = ['feature', 'bugfix', 'hotfix'];

export function normalizeIssueKind(kind?: string | null): IssueKind {
  if (kind === 'bugfix' || kind === 'hotfix') return kind;
  return 'feature';
}

export function prefixForIssueKind(
  config: BranchPrefixConfig | undefined | null,
  kind?: string | null
): string {
  const k = normalizeIssueKind(kind);
  const feature = config?.featurePrefix || 'feature/';
  const bugfix = config?.bugfixPrefix || 'fix/';
  const hotfix = config?.hotfixPrefix || 'hotfix/';
  if (k === 'bugfix') return bugfix;
  if (k === 'hotfix') return hotfix;
  return feature;
}

/** Preview / actual branch name: `{prefix}issue-{shortId}`. */
export function branchNameForIssue(
  config: BranchPrefixConfig | undefined | null,
  issue: Pick<Issue, 'id' | 'kind'> | { id: string; kind?: string },
  shortLen = 8
): string {
  const prefix = prefixForIssueKind(config, issue.kind);
  let short = (issue.id || '').trim();
  if (short.length > shortLen) short = short.slice(-shortLen);
  if (!short) short = 'unknown';
  return `${prefix}issue-${short}`;
}
