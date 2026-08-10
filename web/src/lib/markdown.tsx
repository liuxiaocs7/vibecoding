import React from 'react';

/** Minimal safe-ish markdown renderer (no HTML passthrough). */
export function MarkdownView({ text, className = '' }: { text: string; className?: string }) {
  const lines = (text || '').split('\n');
  const nodes: React.ReactNode[] = [];
  let i = 0;
  let inCode = false;
  let codeBuf: string[] = [];
  let codeLang = '';

  const flushCode = () => {
    nodes.push(
      <pre
        key={`code-${nodes.length}`}
        className="my-2 overflow-x-auto rounded-lg bg-black/40 p-3 text-[11px] font-mono leading-relaxed"
      >
        {codeLang ? (
          <div className="mb-1 text-[10px] uppercase tracking-wider opacity-50">{codeLang}</div>
        ) : null}
        <code>{codeBuf.join('\n')}</code>
      </pre>
    );
    codeBuf = [];
    codeLang = '';
  };

  while (i < lines.length) {
    const line = lines[i];
    const trimmed = line.trim();

    if (trimmed.startsWith('```')) {
      if (inCode) {
        flushCode();
        inCode = false;
      } else {
        inCode = true;
        codeLang = trimmed.slice(3).trim();
      }
      i++;
      continue;
    }
    if (inCode) {
      codeBuf.push(line);
      i++;
      continue;
    }

    if (trimmed.startsWith('|') && trimmed.endsWith('|')) {
      const tableLines: string[] = [];
      while (i < lines.length) {
        const l = lines[i].trim();
        if (!l.startsWith('|')) break;
        tableLines.push(l);
        i++;
      }
      const rows = tableLines
        .filter((l) => !/^\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)+\|?$/.test(l))
        .map((l) =>
          l
            .replace(/^\|/, '')
            .replace(/\|$/, '')
            .split('|')
            .map((c) => c.trim())
        );
      if (rows.length > 0) {
        nodes.push(
          <div key={`table-${nodes.length}`} className="my-3 overflow-x-auto rounded-xl border border-white/10">
            <table className="w-full text-left text-xs">
              <thead>
                <tr className="bg-black/20">
                  {rows[0].map((cell, ci) => (
                    <th key={ci} className="px-3 py-2 font-semibold">
                      {inlineFormat(cell)}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {rows.slice(1).map((row, ri) => (
                  <tr key={ri} className="border-t border-white/10">
                    {row.map((cell, ci) => (
                      <td key={ci} className="px-3 py-2">
                        {inlineFormat(cell)}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        );
      }
      continue;
    }

    const heading = line.match(/^(#{1,6})\s+(.+)$/);
    if (heading) {
      const level = heading[1].length;
      const content = inlineFormat(heading[2]);
      const headingClass =
        level === 1
          ? 'mt-2 mb-3 text-lg font-bold'
          : level === 2
          ? 'mt-5 mb-2 text-base font-bold border-b border-white/10 pb-1'
          : level === 3
          ? 'mt-4 mb-1.5 text-sm font-bold'
          : level === 4
          ? 'mt-3 mb-1 text-sm font-semibold'
          : 'mt-2 mb-1 text-xs font-semibold opacity-90';
      nodes.push(
        React.createElement(
          `h${level}`,
          { key: i, className: headingClass },
          content
        )
      );
    } else if (/^\d+\.\s+/.test(trimmed)) {
      const content = trimmed.replace(/^\d+\.\s+/, '');
      const num = trimmed.match(/^(\d+)\./)?.[1] || '';
      nodes.push(
        <div key={i} className="ml-1 flex gap-2 text-sm leading-relaxed my-0.5">
          <span className="shrink-0 font-mono text-[11px] opacity-60 w-5 text-right">{num}.</span>
          <span>{inlineFormat(content)}</span>
        </div>
      );
    } else if (trimmed.startsWith('- ') || trimmed.startsWith('* ')) {
      nodes.push(
        <li key={i} className="ml-5 list-disc text-sm leading-relaxed my-0.5">
          {inlineFormat(trimmed.slice(2))}
        </li>
      );
    } else if (trimmed.startsWith('> ')) {
      nodes.push(
        <blockquote
          key={i}
          className="my-2 border-l-2 border-indigo-400/50 pl-3 text-sm leading-relaxed opacity-90"
        >
          {inlineFormat(trimmed.slice(2))}
        </blockquote>
      );
    } else if (/^[-*_]{3,}$/.test(trimmed)) {
      nodes.push(<hr key={i} className="my-4 border-white/10" />);
    } else if (trimmed === '') {
      nodes.push(<div key={i} className="h-2" />);
    } else {
      nodes.push(
        <p key={i} className="text-sm leading-relaxed my-1">
          {inlineFormat(line)}
        </p>
      );
    }
    i++;
  }
  if (inCode && codeBuf.length) flushCode();

  return <div className={`select-text prose-vibecoding ${className}`}>{nodes}</div>;
}

function inlineFormat(text: string): React.ReactNode {
  const parts = text.split(/(`[^`]+`|\*\*[^*]+\*\*|__[^_]+__|\*[^*]+\*|_[^_]+_|\[[^\]]+\]\([^)]+\))/g);
  return parts.map((part, idx) => {
    if (!part) return null;
    if (part.startsWith('`') && part.endsWith('`') && part.length >= 2) {
      return (
        <code key={idx} className="rounded bg-black/25 px-1 py-0.5 font-mono text-[11px]">
          {part.slice(1, -1)}
        </code>
      );
    }
    if ((part.startsWith('**') && part.endsWith('**')) || (part.startsWith('__') && part.endsWith('__'))) {
      return (
        <strong key={idx} className="font-semibold">
          {part.slice(2, -2)}
        </strong>
      );
    }
    if (
      ((part.startsWith('*') && part.endsWith('*')) || (part.startsWith('_') && part.endsWith('_'))) &&
      part.length >= 2 &&
      !part.startsWith('**')
    ) {
      return (
        <em key={idx} className="italic">
          {part.slice(1, -1)}
        </em>
      );
    }
    const link = part.match(/^\[([^\]]+)\]\(([^)]+)\)$/);
    if (link) {
      return (
        <a
          key={idx}
          href={link[2]}
          target="_blank"
          rel="noreferrer"
          className="text-indigo-400 underline underline-offset-2"
        >
          {link[1]}
        </a>
      );
    }
    return <span key={idx}>{part}</span>;
  });
}
