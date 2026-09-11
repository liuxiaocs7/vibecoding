/** Save a text file. WebView/Safari ignore <a download>, so we never rely on it. */

type SavePickerHandle = {
  createWritable: () => Promise<{
    write: (data: Blob | string) => Promise<void>;
    close: () => Promise<void>;
  }>;
};

type WailsWindow = Window & {
  showSaveFilePicker?: (opts: {
    suggestedName?: string;
    types?: { description: string; accept: Record<string, string[]> }[];
  }) => Promise<SavePickerHandle>;
  go?: Record<string, Record<string, Record<string, unknown>>>;
};

export type SaveTextResult =
  | { status: 'saved'; path?: string }
  | { status: 'cancelled' }
  | { status: 'copied' };

function findWailsFn(name: string): ((...args: string[]) => Promise<string>) | undefined {
  const pkgs = (window as WailsWindow).go;
  if (!pkgs || typeof pkgs !== 'object') return undefined;
  for (const pkg of Object.values(pkgs)) {
    if (!pkg || typeof pkg !== 'object') continue;
    for (const svc of Object.values(pkg)) {
      if (!svc || typeof svc !== 'object') continue;
      const fn = (svc as Record<string, unknown>)[name];
      if (typeof fn === 'function') {
        return fn as (...args: string[]) => Promise<string>;
      }
    }
  }
  return undefined;
}

async function saveViaPicker(filename: string, contents: string): Promise<SaveTextResult | null> {
  const w = window as WailsWindow;
  if (typeof w.showSaveFilePicker !== 'function') return null;
  try {
    const handle = await w.showSaveFilePicker({
      suggestedName: filename,
      types: [
        {
          description: 'Markdown',
          accept: { 'text/plain': ['.md', '.txt'] },
        },
      ],
    });
    const writable = await handle.createWritable();
    await writable.write(contents);
    await writable.close();
    return { status: 'saved' };
  } catch (err: unknown) {
    const name = (err as { name?: string })?.name;
    if (name === 'AbortError') return { status: 'cancelled' };
    return null;
  }
}

async function saveViaLocalAPI(filename: string, contents: string): Promise<SaveTextResult | null> {
  try {
    const res = await fetch('/api/export-file', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ filename, contents }),
    });
    const data = (await res.json().catch(() => ({}))) as { path?: string; error?: string };
    if (!res.ok) return null;
    if (data.path) return { status: 'saved', path: data.path };
    return { status: 'saved' };
  } catch {
    return null;
  }
}

async function copyToClipboard(contents: string): Promise<boolean> {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(contents);
      return true;
    }
  } catch {
    /* fall through */
  }
  try {
    const ta = document.createElement('textarea');
    ta.value = contents;
    ta.setAttribute('readonly', '');
    ta.style.position = 'fixed';
    ta.style.left = '-9999px';
    document.body.appendChild(ta);
    ta.select();
    const ok = document.execCommand('copy');
    ta.remove();
    return ok;
  } catch {
    return false;
  }
}

/**
 * Save markdown for the "导出下载" button.
 *
 * Desktop (Wails): write straight to ~/Downloads first. Native save-sheets often
 * open behind the Issue modal and look like a dead click when cancelled — we
 * must not stop the fallback chain on that cancel path for the download button.
 */
export async function saveTextFile(filename: string, contents: string): Promise<SaveTextResult> {
  // 1) Desktop: direct Downloads write (most reliable in WKWebView + modal UI).
  const toDownloads = findWailsFn('SaveTextFileToDownloads');
  if (toDownloads) {
    try {
      const path = await toDownloads(filename, contents);
      if (path) {
        void findWailsFn('RevealInFinder')?.(path);
        return { status: 'saved', path };
      }
    } catch {
      // Fall through to dialog / HTTP / clipboard.
    }
  }

  // 2) Native save-as dialog (optional; may open behind modal).
  const wailsSave = findWailsFn('SaveTextFile');
  if (wailsSave) {
    try {
      const path = await wailsSave(filename, contents);
      if (path) {
        void findWailsFn('RevealInFinder')?.(path);
        return { status: 'saved', path };
      }
      // Cancelled — still try server-side Downloads so the button never feels dead.
    } catch {
      // Dialog failed — keep going.
    }
  }

  // 3) Browser File System Access API.
  const picked = await saveViaPicker(filename, contents);
  if (picked) return picked;

  // 4) Local HTTP API → ~/Downloads (works in desktop asset server + browser server mode).
  const local = await saveViaLocalAPI(filename, contents);
  if (local) {
    if (local.status === 'saved' && local.path) {
      void findWailsFn('RevealInFinder')?.(local.path);
    }
    return local;
  }

  // 5) Last resort: clipboard.
  if (await copyToClipboard(contents)) {
    return { status: 'copied' };
  }

  throw new Error('save failed');
}
