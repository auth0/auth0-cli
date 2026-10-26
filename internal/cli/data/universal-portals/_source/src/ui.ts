import { box, cancel, log, note, spinner } from "@clack/prompts";

// The only module that writes human output. It draws with Clack's prompts package:
// the cover, section headers, item and result lines, and a closing note. Everything
// goes to stderr, and stdout is reserved for the JSON result. Silent mode (agent and
// --json) draws nothing.
//
// Output is sequential: each line stays where it was printed. A section header
// starts a block, its items follow, and its result line closes it. A blank line
// comes before every header, item and result, so each entity reads on its own.

const RESET = "\x1b[0m";
const codes = {
  bold: "\x1b[1m",
  dim: "\x1b[2m",
  green: "\x1b[32m",
  yellow: "\x1b[33m",
  red: "\x1b[91m",
  blue: "\x1b[34m",
  cyan: "\x1b[36m",
  magenta: "\x1b[35m",
} as const;

export type Style = keyof typeof codes;

export function colorEnabled(env: NodeJS.ProcessEnv, isTTY: boolean, noColorFlag = false): boolean {
  if (env.NO_COLOR !== undefined || noColorFlag) return false;
  return isTTY;
}

export class Ui {
  // silent suppresses all human output (agent and --json modes). animate enables
  // the spinner, which needs a terminal.
  private spinnerGap = false;

  constructor(
    private readonly output: NodeJS.WriteStream,
    private readonly color: boolean,
    private readonly silent: boolean,
    private readonly animate: boolean = false,
  ) {}

  paint(style: Style, text: string): string {
    if (!this.color) return text;
    return `${codes[style]}${text}${RESET}`;
  }

  // The title box: the command, and the tenant every change will go to. It prints
  // before any question, so a wrong tenant is visible from the start.
  banner(title: string, tenant: string): void {
    if (this.silent) return;
    box(this.paint("cyan", tenant), this.paint("bold", title), {
      output: this.output,
      width: "auto",
      rounded: true,
      contentAlign: "center",
      titleAlign: "center",
    });
  }

  // A section header starts a block of items, closed by its result.
  section(title: string): void {
    if (this.silent) return;
    this.breakLine();
    log.step(title, { output: this.output, spacing: 0 });
  }

  // One item of a section: a plain line. Only the section result carries a symbol.
  success(text: string, detail?: string): void {
    if (this.silent) return;
    const suffix = detail ? ` ${this.paint("dim", detail)}` : "";
    this.breakLine();
    log.message(`${text}${suffix}`, { output: this.output, spacing: 0 });
  }

  // A change the CLI made, with a green check so it reads as done.
  fixed(text: string, detail?: string): void {
    if (this.silent) return;
    const suffix = detail ? ` ${this.paint("dim", detail)}` : "";
    this.breakLine();
    log.message(`${this.paint("green", "✓")} ${text}${suffix}`, { output: this.output, spacing: 0 });
  }

  // The outcome of a section, drawn like the other results.
  result(text: string): void {
    if (this.silent) return;
    this.breakLine();
    log.success(text, { output: this.output, spacing: 0 });
  }

  warn(text: string): void {
    if (this.silent) return;
    log.warn(text, { output: this.output, spacing: 0 });
  }

  error(text: string): void {
    if (this.silent) return;
    log.error(text, { output: this.output, spacing: 0 });
  }

  info(text: string): void {
    if (this.silent) return;
    log.info(text, { output: this.output, spacing: 0 });
  }

  detail(text: string): void {
    if (this.silent) return;
    log.message(this.paint("dim", text), { output: this.output, spacing: 0 });
  }

  // Shows an ID that links to its dashboard page. Terminals with OSC 8 support
  // make the ID clickable. Without color (no terminal, NO_COLOR) it stays plain
  // text, as the core CLI does.
  reference(label: string, id: string, url: string): void {
    if (this.silent) return;
    const linked = this.color ? `\x1b]8;;${url}\x1b\\${this.paint("cyan", id)}\x1b]8;;\x1b\\` : id;
    log.message(`${this.paint("dim", `${label}:`)} ${linked}`, { output: this.output, spacing: 0 });
  }

  // The closing block of a successful setup: the result, then the portal URL in a note.
  ready(url: string): void {
    if (this.silent) return;
    this.breakLine();
    const linked = this.color ? `\x1b]8;;${url}\x1b\\${this.paint("cyan", url)}\x1b]8;;\x1b\\` : url;
    note(`${this.paint("dim", "Open portal:")} ${linked}`, "Portal URL", { output: this.output });
  }

  // The closing line when the user cancels. Clack draws it in the frame.
  cancelled(message: string): void {
    if (this.silent) return;
    cancel(message, { output: this.output });
  }

  // A table with a header row. Widths follow the longest cell, and the first
  // column is the status symbol, colored by its text.
  table(headers: string[], rows: string[][]): void {
    if (this.silent) return;
    const widths = headers.map((header, column) =>
      Math.max(header.length, ...rows.map((row) => row[column].length)),
    );
    const format = (cells: string[], style?: (cell: string, column: number) => string) =>
      cells
        .map((cell, column) => {
          const padded = cell.padEnd(widths[column]);
          return style ? style(padded, column) : padded;
        })
        .join("  ");
    log.message(this.paint("dim", format(headers)), { output: this.output, spacing: 0 });
    for (const row of rows) {
      const status = row[0] === "✖" ? "red" : "yellow";
      log.message(format(row, (cell, column) => (column === 0 ? this.paint(status, cell) : cell)), { output: this.output, spacing: 0 });
    }
  }

  newline(): void {
    if (this.silent) return;
    log.message("", { output: this.output, spacing: 0 });
  }

  // Runs work behind a spinner. The spinner line is cleared when the work ends,
  // so the caller prints the result in its place. Without a terminal there is no
  // spinner, only the work itself.
  async task<T>(label: string, work: () => Promise<T>): Promise<T> {
    if (this.silent || !this.animate) return work();

    const running = spinner({ output: this.output });
    running.start(label);
    try {
      return await work();
    } finally {
      running.clear();
      // The spinner drew its own guide line, which is the gap before the next line.
      this.spinnerGap = true;
    }
  }

  // A blank line before a header, item or result. Right after a spinner the gap is
  // already there, so no second one is drawn.
  private breakLine(): void {
    if (this.spinnerGap) {
      this.spinnerGap = false;
      return;
    }
    log.message("", { output: this.output, spacing: 0 });
  }
}
