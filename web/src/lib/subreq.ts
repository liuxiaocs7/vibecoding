import type { Issue, SpecFileChange, DevSpec } from '../types';

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

/** Recover Markdown when a doc body accidentally stored the model JSON envelope. */
export function coerceReqMarkdown(md: string | undefined | null): string {
  return coerceDocMarkdown(md);
}

/** Same recovery for Dev Spec / ReqDoc bodies that stored {"chatReply","rawMarkdown",...}. */
export function coerceDocMarkdown(md: string | undefined | null): string {
  const raw = (md || '').trim();
  if (!raw) return '';
  if (!(raw.startsWith('{') && (raw.includes('"rawMarkdown"') || raw.includes('"chatReply"')))) {
    return raw;
  }
  try {
    const parsed = JSON.parse(raw) as { rawMarkdown?: string };
    const inner = (parsed.rawMarkdown || '').trim();
    if (inner && !(inner.startsWith('{') && inner.includes('"rawMarkdown"'))) {
      return inner;
    }
  } catch {
    // fall through to field scanner
  }
  const m = raw.match(/"rawMarkdown"\s*:\s*"((?:\\.|[^"\\])*)"/);
  if (m?.[1]) {
    return m[1]
      .replace(/\\n/g, '\n')
      .replace(/\\r/g, '\r')
      .replace(/\\t/g, '\t')
      .replace(/\\"/g, '"')
      .replace(/\\\\/g, '\\');
  }
  return raw;
}

/** Heal a DevSpec that stored the model JSON envelope; also recover fileChanges etc. */
export function coerceDevSpec(spec: DevSpec | undefined | null): DevSpec | undefined {
  if (!spec) return undefined;
  const raw = (spec.rawMarkdown || '').trim();
  if (!(raw.startsWith('{') && (raw.includes('"rawMarkdown"') || raw.includes('"chatReply"')))) {
    return spec;
  }
  try {
    const parsed = JSON.parse(raw) as {
      title?: string;
      summary?: string;
      architectureDesign?: string;
      rawMarkdown?: string;
      fileChanges?: SpecFileChange[];
      implementationSteps?: string[];
      testCases?: string[];
    };
    const md = coerceDocMarkdown(raw);
    if (!md || md === raw) return { ...spec, rawMarkdown: md || spec.rawMarkdown };
    return {
      ...spec,
      title: (parsed.title || spec.title || '').trim() || spec.title,
      summary: (parsed.summary || spec.summary || '').trim() || spec.summary,
      architectureDesign:
        (parsed.architectureDesign || spec.architectureDesign || '').trim() || spec.architectureDesign,
      fileChanges:
        Array.isArray(parsed.fileChanges) && parsed.fileChanges.length > 0
          ? parsed.fileChanges
          : spec.fileChanges || [],
      implementationSteps:
        Array.isArray(parsed.implementationSteps) && parsed.implementationSteps.length > 0
          ? parsed.implementationSteps
          : spec.implementationSteps || [],
      testCases:
        Array.isArray(parsed.testCases) && parsed.testCases.length > 0
          ? parsed.testCases
          : spec.testCases || [],
      rawMarkdown: md,
    };
  } catch {
    const md = coerceDocMarkdown(raw);
    return md === raw ? spec : { ...spec, rawMarkdown: md };
  }
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
  return true;
}

/** Human-readable reason Spec is not ready for backlog (empty string if ready). */
export function backlogBlockReason(issue: Issue, lang: 'zh' | 'en' = 'zh'): string {
  if (specReadyForDev(issue)) return '';
  if (!hasDevSpecMarkdown(issue)) {
    return lang === 'zh'
      ? '移入待执行前需先有 Markdown 开发设计文档'
      : 'Markdown Dev Spec required before backlog';
  }
  if (!requirementAccepted(issue) && hasReqDoc(issue)) {
    return lang === 'zh'
      ? '请先确认需求文档，再排期开发设计'
      : 'Accept the requirement document before scheduling';
  }
  if (designStale(issue)) {
    return lang === 'zh'
      ? '需求已变更，请重新生成开发设计后再排期'
      : 'Requirement changed — regenerate the Dev Spec before backlog';
  }
  return lang === 'zh' ? '暂不可排期，请检查需求与开发设计' : 'Not ready for backlog yet';
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

/** Build a Markdown requirement document from the issue brief + attachment text. */
export function briefToReqMarkdown(issue: Issue): string {
  const title = (issue.title || 'Requirement').trim();
  const desc = (issue.description || '').trim();
  const parts: string[] = [`# ${title}`, ''];

  parts.push('## 概述 / Summary', '');
  parts.push(desc || '_(待补充)_', '');

  const attachments = issue.attachments || [];
  const textAtts = attachments.filter((a) => (a.text || '').trim());
  if (textAtts.length > 0) {
    parts.push('## 附件原文 / Attached briefs', '');
    for (const a of textAtts) {
      parts.push(`### ${a.name || 'attachment'}`, '', a.text!.trim(), '');
    }
  }

  const imageAtts = attachments.filter((a) => a.kind === 'image');
  if (imageAtts.length > 0) {
    parts.push('## 附图 / Images', '');
    for (const a of imageAtts) {
      parts.push(`- ${a.name || 'image'}`, '');
    }
  }

  parts.push(
    '## 范围 / Scope',
    '',
    '_(待补充：本期要做什么)_',
    '',
    '## 非目标 / Non-goals',
    '',
    '_(待补充：明确不做的内容)_',
    '',
    '## 验收标准 / Acceptance',
    '',
    '_(待补充：可验证的完成条件)_',
    '',
    '## 约束 / Constraints',
    '',
    '_(待补充：技术/业务约束)_',
    ''
  );
  return parts.join('\n');
}

export function reqMarkdownForExport(
  issue: Issue,
  editingMarkdown?: string
): { filename: string; markdown: string } {
  const editing = coerceReqMarkdown(editingMarkdown);
  if (editing) {
    const title = issue.reqDoc?.title || issue.title || 'requirement';
    return { filename: specFilename(title, `${issue.id}-req`), markdown: editing };
  }
  const md = coerceReqMarkdown(issue.reqDoc?.rawMarkdown);
  const title = issue.reqDoc?.title || issue.title || 'requirement';
  return { filename: specFilename(`${title}-req`, `${issue.id}-req`), markdown: md };
}

export function specMarkdownForExport(
  issue: Issue,
  scope: string,
  editingMarkdown?: string
): { filename: string; markdown: string } {
  const editing = coerceDocMarkdown(editingMarkdown);
  if (editing) {
    const spec = visibleSpec(issue, scope);
    const title = spec?.title || issue.title || 'dev-spec';
    return { filename: specFilename(title, issue.id), markdown: editing };
  }
  if (scope && scope !== 'all') {
    const sub = (issue.subRequirements || []).find((s) => s.id === scope);
    const md = coerceDocMarkdown(sub?.devSpec?.rawMarkdown || sub?.devSpec?.summary || '');
    const title = sub?.devSpec?.title || sub?.title || issue.title;
    return { filename: specFilename(title, issue.id), markdown: md };
  }
  if (hasSubRequirements(issue)) {
    const parts: string[] = [];
    const overview = coerceDocMarkdown(issue.devSpec?.rawMarkdown);
    if (overview) parts.push(overview);
    for (const sub of issue.subRequirements || []) {
      const body =
        coerceDocMarkdown(sub.devSpec?.rawMarkdown) ||
        `# ${sub.title}\n\n${sub.description || ''}`.trim();
      if (body) parts.push(body);
    }
    const title = issue.devSpec?.title || issue.title;
    return { filename: specFilename(title, issue.id), markdown: parts.join('\n\n---\n\n') };
  }
  const spec = issue.devSpec;
  return {
    filename: specFilename(spec?.title || issue.title, issue.id),
    markdown: coerceDocMarkdown(spec?.rawMarkdown || spec?.summary || ''),
  };
}
