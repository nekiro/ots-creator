// Tibia-style replacement for window.confirm. ConfirmDialog (mounted in
// App) renders the pending question; ask() resolves with the answer.

export interface Question {
  title: string;
  message: string;
  ok: string;
  /** Red confirming button; on by default for removing words (Remove, Delete…). */
  danger?: boolean;
  alt?: string;
}

/** The button that closed a question; null is Cancel. */
export type Choice = "ok" | "alt" | null;

export const DESTRUCTIVE = /^(remove|delete|discard|clear|erase|reset)\b/i;

export const confirmState = $state<{ question: (Question & { resolve: (choice: Choice) => void }) | null }>({ question: null });

/** Shows a confirmation dialog. A second question cancels the first. */
export async function ask(question: Question): Promise<boolean> {
  return (await choose({ ...question, alt: undefined })) === "ok";
}

export function choose(question: Question): Promise<Choice> {
  confirmState.question?.resolve(null);
  return new Promise((resolve) => {
    confirmState.question = { ...question, resolve };
  });
}

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

export function askText(question: TextQuestion): Promise<string | null> {
  promptState.question?.resolve(null);
  return new Promise((resolve) => {
    promptState.question = { ...question, resolve };
  });
}

export function answerText(text: string | null): void {
  const question = promptState.question;
  promptState.question = null;
  question?.resolve(text);
}
