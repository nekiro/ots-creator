import { errorMessage } from "./api";
import { app, toast } from "./state/app.svelte";
import { isTyping } from "./ui/typing";
import { clipboardCommands } from "./commands/objectClipboard";
import { dropCommands } from "./commands/drop";
import { objectCommands } from "./commands/objects";
import { projectCommands } from "./commands/project";
import { sheetCommands } from "./commands/sheets";
import { spriteCommands } from "./commands/sprites";

export { PREVIEW_DROP, thingDropId } from "./commands/classifyDrop";
export { offerOptimize, openedNote, otbNote, sheetsNote } from "./commands/compile";
export { handlePaste } from "./commands/objectClipboard";
export { moveSelection } from "./commands/objects";

export const commands = {
  ...projectCommands,
  ...objectCommands,
  ...sheetCommands,
  ...spriteCommands,
  ...clipboardCommands,
  ...dropCommands,
};

export function handleShortcut(event: KeyboardEvent): void {
  const typing = isTyping(event);
  const mod = event.ctrlKey || event.metaKey;
  const key = event.key.toLowerCase();
  const run = (action: () => unknown) => {
    event.preventDefault();
    Promise.resolve(action()).catch((err) => toast(errorMessage(err), "error"));
  };
  if (app.dialog) return;
  if (mod && key === "o") return run(() => commands.open());
  if (mod && key === "n") return run(commands.create);
  if (mod && event.shiftKey && key === "s") return run(commands.compileAs);
  if (mod && key === "s") return run(commands.compile);
  if (mod && key === "enter") return run(commands.apply);
  if (typing) return;
  if (mod && key === "z" && !event.shiftKey) return run(commands.undo);
  if (mod && (key === "y" || (key === "z" && event.shiftKey))) return run(commands.redo);
  if (mod && key === "d") return run(commands.duplicate);
  if (mod && key === "i") return run(() => commands.importObd());
  if (mod && key === "e") return run(() => commands.exportObd());
  if (mod && event.shiftKey && key === "c") return run(() => commands.copySheet(0));
  if (mod && key === "c") return run(commands.copyObject);
  if (mod && event.shiftKey && key === "v") return run(() => commands.paste("properties"));
  // Plain Ctrl+V arrives as a paste event (handlePaste) with the clipboard.
}
