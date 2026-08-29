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
  runtime?: { BrowserOpenURL?: (url: string) => void };
};

export type SaveTextResult =
  | { status: 'saved'; path?: string }
  | { status: 'cancelled' }
  | { status: 'copied' };

function wailsSaveFn(): ((filename: string, contents: string) => Promise<string>) | undefined {
  const w = window as WailsWindow;
  const pkgs = w.go;
  if (!pkgs || typeof pkgs !== 'object') return undefined;
  for (const pkg of Object.values(pkgs)) {
    if (!pkg || typeof pkg !== 'object') continue;
    for (const svc of Object.values(pkg)) {
      if (!svc || typeof svc !== 'object') continue;
      const fn = (svc as { SaveTextFile?: unknown }).SaveTextFile;
      if (typeof fn === 'function') {
        return fn as (filename: string, contents: string) => Promise<string>;
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

export async function saveTextFile(filename: string, contents: string): Promise<SaveTextResult> {
  const wailsSave = wailsSaveFn();
  if (wailsSave) {
    try {
      const path = await wailsSave(filename, contents);
      if (!path) return { status: 'cancelled' };
      return { status: 'saved', path };
    } catch {
      // Binding present but dialog failed — keep going.
    }
  }

  const picked = await saveViaPicker(filename, contents);
  if (picked) return picked;

  const local = await saveViaLocalAPI(filename, contents);
  if (local) return local;

  if (await copyToClipboard(contents)) {
    return { status: 'copied' };
  }

  throw new Error('save failed');
}
