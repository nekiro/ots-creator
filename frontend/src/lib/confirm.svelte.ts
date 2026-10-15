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

export const confirmState = $state<{ question: (Question & { resolve: (c: Choice) => void }) | null }>({ question: null });

/** Shows a confirmation dialog. A second question cancels the first. */
export async function ask(q: Question): Promise<boolean> {
  return (await choose({ ...q, alt: undefined })) === "ok";
}

/** Shows a confirmation dialog with an extra button. A second question cancels the first. */
export function choose(q: Question): Promise<Choice> {
  confirmState.question?.resolve(null);
  return new Promise((resolve) => {
    confirmState.question = { ...q, resolve };
  });
}

/** Answers the pending question: true is the ok button, false is Cancel. */
export function answer(c: boolean | "alt"): void {
  const q = confirmState.question;
  confirmState.question = null;
  q?.resolve(c === true ? "ok" : c === "alt" ? "alt" : null);
}

export interface TextQuestion {
  title: string;
  label: string;
  value?: string;
  ok?: string;
}

export const promptState = $state<{ question: (TextQuestion & { resolve: (v: string | null) => void }) | null }>({ question: null });

/** Asks for a line of text (Tibia-style window.prompt); null when cancelled. */
export function askText(q: TextQuestion): Promise<string | null> {
  promptState.question?.resolve(null);
  return new Promise((resolve) => {
    promptState.question = { ...q, resolve };
  });
}

/** Answers the pending text question. */
export function answerText(v: string | null): void {
  const q = promptState.question;
  promptState.question = null;
  q?.resolve(v);
}
