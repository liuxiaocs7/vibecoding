/** Consume a POST SSE (`text/event-stream`) response. */

export type StreamEvent = {
  type: string;
  text?: string;
  message?: string;
  error?: string;
  spec?: unknown;
  chatReply?: string;
  process?: string;
  [key: string]: unknown;
};

export async function readSSE(
  res: Response,
  onEvent: (ev: StreamEvent) => void
): Promise<void> {
  if (!res.body) {
    throw new Error('Streaming unsupported by browser');
  }
  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });

    let sep: number;
    while ((sep = buffer.indexOf('\n\n')) >= 0) {
      const raw = buffer.slice(0, sep);
      buffer = buffer.slice(sep + 2);
      for (const line of raw.split('\n')) {
        const trimmed = line.trim();
        if (!trimmed.startsWith('data:')) continue;
        const data = trimmed.slice(5).trim();
        if (!data || data === '[DONE]') continue;
        try {
          onEvent(JSON.parse(data) as StreamEvent);
        } catch {
          // ignore malformed chunks
        }
      }
    }
  }
}
