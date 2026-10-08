<script lang="ts">
  import { Browser } from "@wailsio/runtime";
  import { app } from "../../lib/state.svelte";
  import { installUpdate, REPO_URL, updates } from "../../lib/updates.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";

  const release = $derived(updates.info?.release ?? null);
  const busy = $derived(updates.stage !== "idle");
  const percent = $derived(updates.progress && updates.progress.total > 0 ? Math.min(100, (updates.progress.written / updates.progress.total) * 100) : 0);
  const mb = (n: number) => (n / 1048576).toFixed(1);
  const STAGES = { idle: "", downloading: "Downloading…", verifying: "Verifying checksum…", installing: "Installing…", restarting: "Restarting…" };

  function close() {
    if (!busy) app.dialog = null;
  }
</script>

<Dialog title="Update available" width={460} onclose={close}>
  {#if release}
    <div class="col">
      <p>OTS Creator <strong>{release.version}</strong> is available. You have {updates.version}.</p>
      {#if release.notes}
        <div class="notes t-panel">{release.notes}</div>
      {/if}
      {#if busy}
        <div class="progress" title={STAGES[updates.stage]}>
          <div class="fill" style="width:{updates.stage === 'downloading' ? percent : 100}%"></div>
          <span>
            {#if updates.stage === "downloading" && updates.progress}{mb(updates.progress.written)} / {mb(updates.progress.total)} MB{:else}{STAGES[updates.stage]}{/if}
          </span>
        </div>
        <p class="t-label">OTS Creator restarts into the new version when the update is installed.</p>
      {/if}
    </div>
  {/if}
  {#snippet footer()}
    <button class="t-btn" disabled={!release} onclick={() => release && Browser.OpenURL(`${REPO_URL}/releases/tag/v${release.version}`)}>Release page</button>
    <span class="grow"></span>
    <button class="t-btn primary" disabled={!release || busy} onclick={installUpdate}>Update now</button>
    <button class="t-btn" disabled={busy} onclick={close}>Later</button>
  {/snippet}
</Dialog>

<style>
  p {
    margin: 0;
  }
  strong {
    color: var(--text-bright);
  }
  .notes {
    max-height: 220px;
    overflow: auto;
    padding: 6px 8px;
    white-space: pre-wrap;
    user-select: text;
  }
  /* OTClient ProgressBar: progressbar.png, image-border 2, color drawn behind */
  .progress {
    position: relative;
    height: 16px;
    display: grid;
    place-items: center;
    color: var(--text-bright);
    text-shadow: 1px 1px 0 #000;
  }
  .progress::after {
    content: "";
    position: absolute;
    inset: 0;
    border-style: solid;
    border-width: 2px;
    border-image: url("../../assets/ui/progressbar.png") 2 fill / 2px repeat;
    image-rendering: pixelated;
    pointer-events: none;
  }
  .fill {
    position: absolute;
    left: 2px;
    top: 2px;
    bottom: 2px;
    max-width: calc(100% - 4px);
    background: #3c9a3c;
  }
  .progress span {
    position: relative;
    z-index: 1;
  }
</style>
