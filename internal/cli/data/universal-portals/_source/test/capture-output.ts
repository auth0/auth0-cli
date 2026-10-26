import { Writable } from "node:stream";

// A stream that keeps what is written to it, shaped like a pipe (not a terminal).
export interface CapturedOutput {
  stream: NodeJS.WriteStream;
  text(): string;
}

export function captureOutput(): CapturedOutput {
  let text = "";
  const writable = new Writable({
    write(chunk, _encoding, done) {
      text += chunk;
      done();
    },
  });
  return {
    stream: Object.assign(writable, { isTTY: false, columns: 80 }) as unknown as NodeJS.WriteStream,
    text: () => text,
  };
}
