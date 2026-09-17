import type { Issue, SpecFileChange } from '../types';

export function hasSubRequirements(issue: Issue): boolean {
  return (issue.subRequirements?.length ?? 0) > 0;
}

function hasDevSpecMarkdown(issue: Issue): boolean {
  if (hasSubRequirements(issue)) {
    const list = issue.subRequirements || [];
    return list.length > 0 && list.every((s) => !!s.devSpec?.rawMarkdown?.trim());
  }
  return !!issue.devSpec?.rawMarkdown?.trim();
}

function hasNonEmptyFileChanges(issue: Issue): boolean {
  if (hasSubRequirements(issue)) {
    const list = issue.subRequirements || [];
    return list.length > 0 && list.every((s) => (s.devSpec?.fileChanges?.length ?? 0) > 0);
  }
  return (issue.devSpec?.fileChanges?.length ?? 0) > 0;
}

export function hasReqDoc(issue: Issue): boolean {
  return !!issue.reqDoc?.rawMarkdown?.trim();
}

export function legacySpecOnly(issue: Issue): boolean {
  return !hasReqDoc(issue) && hasDevSpecMarkdown(issue);
}

export function requirementAccepted(issue: Issue): boolean {
  if (!hasReqDoc(issue)) {
    return hasDevSpecMarkdown(issue);
  }
  const acceptedAt = issue.reqDoc?.acceptedAt?.trim();
  if (!acceptedAt) return false;
  const updatedAt = issue.reqDoc?.updatedAt?.trim();
  if (updatedAt && updatedAt > acceptedAt) return false;
  return true;
}

export function designStale(issue: Issue): boolean {
  if (!hasReqDoc(issue)) return false;
  const reqTouch = issue.reqDoc?.updatedAt?.trim();
  if (!reqTouch) return false;
  if (hasSubRequirements(issue)) {
    for (const sub of issue.subRequirements || []) {
      if (!sub.devSpec) continue;
      const u = sub.devSpec.updatedAt?.trim();
      if (u && reqTouch > u) return true;
      if (!u && sub.devSpec.rawMarkdown?.trim()) return true;
    }
    return false;
  }
  if (!issue.devSpec?.rawMarkdown?.trim()) return false;
  const u = issue.devSpec.updatedAt?.trim();
  if (!u) return true;
  return reqTouch > u;
}

export function specReadyForDev(issue: Issue): boolean {
  if (!hasDevSpecMarkdown(issue)) return false;
  if (legacySpecOnly(issue)) return true;
  if (!requirementAccepted(issue) || designStale(issue)) return false;
  return hasNonEmptyFileChanges(issue);
}

export function hasUnverifiedModifies(issue: Issue): boolean {
  return aggregatedFileChanges(issue).some((ch) => {
    const action = (ch.action || '').toLowerCase();
    if (action === 'create' || action === '') return false;
    return !ch.verified;
  });
}

export function readySubCount(issue: Issue): { ready: number; total: number } {
  const list = issue.subRequirements || [];
  return {
    ready: list.filter((s) => !!s.devSpec?.rawMarkdown?.trim()).length,
    total: list.length,
  };
}

export function aggregatedFileChanges(issue: Issue): SpecFileChange[] {
  if (!hasSubRequirements(issue)) {
    return issue.devSpec?.fileChanges || [];
  }
  return (issue.subRequirements || []).flatMap((s) => s.devSpec?.fileChanges || []);
}

export function visibleSpec(issue: Issue, scope: string) {
  if (scope && scope !== 'all') {
    const sub = (issue.subRequirements || []).find((s) => s.id === scope);
    return sub?.devSpec;
  }
  return issue.devSpec;
}

export function specFilename(title: string, fallback: string): string {
  const base =
    (title || fallback || 'dev-spec')
      .replace(/[\\/:*?"<>|]+/g, '-')
      .replace(/\s+/g, '-')
      .replace(/-+/g, '-')
      .slice(0, 80)
      .replace(/^-|-$/g, '') || fallback || 'dev-spec';
  return base.endsWith('.md') ? base : `${base}.md`;
}

export function specMarkdownForExport(
  issue: Issue,
  scope: string,
  editingMarkdown?: string
): { filename: string; markdown: string } {
  const editing = editingMarkdown?.trim();
  if (editing) {
    const spec = visibleSpec(issue, scope);
    const title = spec?.title || issue.title || 'dev-spec';
    return { filename: specFilename(title, issue.id), markdown: editing };
  }
  if (scope && scope !== 'all') {
    const sub = (issue.subRequirements || []).find((s) => s.id === scope);
    const md = (sub?.devSpec?.rawMarkdown || sub?.devSpec?.summary || '').trim();
    const title = sub?.devSpec?.title || sub?.title || issue.title;
    return { filename: specFilename(title, issue.id), markdown: md };
  }
  if (hasSubRequirements(issue)) {
    const parts: string[] = [];
    const overview = issue.devSpec?.rawMarkdown?.trim();
    if (overview) parts.push(overview);
    for (const sub of issue.subRequirements || []) {
      const body = sub.devSpec?.rawMarkdown?.trim() || `# ${sub.title}\n\n${sub.description || ''}`.trim();
      if (body) parts.push(body);
    }
    const title = issue.devSpec?.title || issue.title;
    return { filename: specFilename(title, issue.id), markdown: parts.join('\n\n---\n\n') };
  }
  const spec = issue.devSpec;
  return {
    filename: specFilename(spec?.title || issue.title, issue.id),
    markdown: (spec?.rawMarkdown || spec?.summary || '').trim(),
  };
}
