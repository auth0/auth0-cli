import { confirm, isCancel, multiselect, select, text } from "@clack/prompts";
import { CancelledError, type ChecklistItem, type Choice, type Prompter } from "./prompt.js";

// The interactive questions, drawn with Clack. Input and output are injected so
// tests can answer them without a terminal.
export function clackPrompter(input: NodeJS.ReadStream, output: NodeJS.WriteStream): Prompter {
  return {
    async ask(question, defaultValue, example) {
      const answer = await text({ message: question, defaultValue, placeholder: defaultValue ?? example, input, output });
      return unwrap(answer).trim() || defaultValue || "";
    },

    async confirm(question, defaultValue) {
      return unwrap(await confirm({ message: question, initialValue: defaultValue, input, output }));
    },

    async checklist<T>(question: string, items: ChecklistItem<T>[]): Promise<T[]> {
      const selected = unwrap(
        await multiselect({
          message: question,
          options: items.map((item, position) => ({ value: position, label: item.label, hint: item.hint })),
          initialValues: items.flatMap((item, position) => (item.checked ? [position] : [])),
          required: false,
          input,
          output,
        }),
      );
      return selected.map((position) => items[position].value);
    },

    async choose<T>(question: string, options: Choice<T>[]): Promise<T> {
      // Clack compares option values by identity, so the option index is the value.
      const index = unwrap(
        await select({
          message: question,
          options: options.map((option, position) => ({ value: position, label: option.label, hint: option.hint })),
          input,
          output,
        }),
      );
      return options[index].value;
    },
  };
}

// Clack answers with a cancel symbol instead of a value when the user presses Ctrl+C.
function unwrap<T>(answer: T): Exclude<T, symbol> {
  if (isCancel(answer)) throw new CancelledError();
  return answer as Exclude<T, symbol>;
}
