// Tibia-style replacement for window.confirm. ConfirmDialog (mounted in
// App) renders the pending question; ask() resolves with the answer.

export interface Question {
  title: string;
  message: string;
  /** Label of the confirming button. */
  ok: string;
  /** Red confirming button; on by default for removing words (Remove, Delete…). */
  danger?: boolean;
  /** Label of a second, non-cancelling button (see choose). */
  alt?: string;
}

/** The button that closed a question; null is Cancel. */
export type Choice = "ok" | "alt" | null;

/** Labels of buttons that destroy something: shown red. */
export const DESTRUCTIVE = /^(remove|delete|discard|clear|erase|reset)\b/i;

export const confirmState = $state<{ question: (Question & { resolve: (choice: Choice) => void }) | null }>({ question: null });

/** Shows a confirmation dialog. A second question cancels the first. */
export async function ask(question: Question): Promise<boolean> {
  return (await choose({ ...question, alt: undefined })) === "ok";
}

/** Shows a confirmation dialog with an extra button. A second question cancels the first. */
export function choose(question: Question): Promise<Choice> {
  confirmState.question?.resolve(null);
  return new Promise((resolve) => {
    confirmState.question = { ...question, resolve };
  });
}

/** Answers the pending question: true is the ok button, false is Cancel. */
export function answer(button: boolean | "alt"): void {
  const question = confirmState.question;
  confirmState.question = null;
  question?.resolve(button === true ? "ok" : button === "alt" ? "alt" : null);
}

export interface TextQuestion {
  title: string;
  label: string;
  value?: string;
  ok?: string;
}

export const promptState = $state<{ question: (TextQuestion & { resolve: (text: string | null) => void }) | null }>({ question: null });

/** Asks for a line of text (Tibia-style window.prompt); null when cancelled. */
export function askText(question: TextQuestion): Promise<string | null> {
  promptState.question?.resolve(null);
  return new Promise((resolve) => {
    promptState.question = { ...question, resolve };
  });
}

/** Answers the pending text question. */
export function answerText(text: string | null): void {
  const question = promptState.question;
  promptState.question = null;
  question?.resolve(text);
}
