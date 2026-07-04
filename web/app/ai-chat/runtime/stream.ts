export interface SSEParser {
  push(chunk: string): void;
  finish(): void;
}

export function createSSEParser(
  onFrame: (event: string, data: any) => void,
): SSEParser {
  let buffer = "";

  const flush = (final: boolean) => {
    const frames = buffer.split(/\r?\n\r?\n/);
    if (final) buffer = "";
    else buffer = frames.pop() || "";
    for (const frame of frames) parseFrame(frame, onFrame);
  };

  return {
    push(chunk) {
      buffer += chunk;
      flush(false);
    },
    finish() {
      flush(true);
    },
  };
}

function parseFrame(frame: string, onFrame: (event: string, data: any) => void) {
  let event = "message";
  const dataLines: string[] = [];
  for (const line of frame.split(/\r?\n/)) {
    if (line.startsWith("event:")) event = line.slice(6).trim();
    else if (line.startsWith("data:")) dataLines.push(line.slice(5).trim());
  }
  if (dataLines.length === 0) return;
  try {
    onFrame(event, JSON.parse(dataLines.join("\n")));
  } catch {
    // A malformed frame is isolated; later frames remain consumable.
  }
}

export async function consumeSSEStream(
  body: ReadableStream<Uint8Array>,
  onFrame: (event: string, data: any) => void,
) {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  const parser = createSSEParser(onFrame);
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      parser.push(decoder.decode(value, { stream: true }));
    }
    parser.push(decoder.decode());
    parser.finish();
  } finally {
    reader.releaseLock();
  }
}
