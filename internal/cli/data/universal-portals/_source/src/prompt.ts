// The question types the setup and audit flows depend on. The flows take a
// Prompter, so tests can answer questions without a terminal. The terminal
// implementation is in clack-prompter.ts.
export interface Choice<T> {
  label: string;
  hint?: string;
  value: T;
}

export interface ChecklistItem<T> {
  label: string;
  hint?: string;
  value: T;
  checked: boolean;
}

export interface Prompter {
  // The default is returned on Enter and shown as the placeholder. Without a default,
  // the example is shown as the placeholder only.
  ask(question: string, defaultValue?: string, example?: string): Promise<string>;
  confirm(question: string, defaultValue: boolean): Promise<boolean>;
  choose<T>(question: string, options: Choice<T>[]): Promise<T>;
  // Returns the values of the items the user keeps selected. Items start selected when
  // they say so.
  checklist<T>(question: string, items: ChecklistItem<T>[]): Promise<T[]>;
}

// Thrown when the user presses Ctrl+C at a question.
export class CancelledError extends Error {
  constructor() {
    super("Cancelled.");
    this.name = "CancelledError";
  }
}
