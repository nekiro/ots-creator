// Tibia-style replacement for window.confirm. ConfirmDialog (mounted in
// App) renders the pending question; ask() resolves with the answer.

export interface Question {
  title: string;
  message: string;
  /** Label of the confirming button. */
  ok: string;
}

export const confirmState = $state<{ question: (Question & { resolve: (yes: boolean) => void }) | null }>({ question: null });

/** Shows a confirmation dialog. A second question cancels the first. */
export function ask(q: Question): Promise<boolean> {
  confirmState.question?.resolve(false);
  return new Promise((resolve) => {
    confirmState.question = { ...q, resolve };
  });
}

/** Answers the pending question. */
export function answer(yes: boolean): void {
  const q = confirmState.question;
  confirmState.question = null;
  q?.resolve(yes);
}
