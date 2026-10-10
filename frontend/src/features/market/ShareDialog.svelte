<script lang="ts">
  // Shares an object or sprites to the market. No account: the author is a
  // nickname, and a captcha (a page of the market API shown in a frame)
  // guards against spam. The delete token is kept in the settings.
  import { onMount } from "svelte";
  import {
    OBJECT_EXT,
    CATEGORIES,
    CATEGORY_LABELS,
    CATEGORY_NAMES,
    Category,
    DialogService,
    MarketService,
    ThingService,
    ViewerService,
    decodeBytes,
    errorMessage,
    minId,
    normalizeThing,
    res,
    type MarketConfig,
    type Thing,
  } from "../../lib/api";
  import { formatIds, groupOptions, LICENSES, MAX_TAGS, parseIds, tagGroups, tagProblem, type TagKind } from "../../lib/market";
  import { app, applyDraft, run, toast, versions, type ShareTarget } from "../../lib/state.svelte";
  import Dialog from "../../lib/ui/Dialog.svelte";
  import NumberField from "../../lib/ui/NumberField.svelte";
  import Select from "../../lib/ui/Select.svelte";
  import TagPicker from "../../lib/ui/TagPicker.svelte";
  import ThingCanvas from "../../lib/ui/ThingCanvas.svelte";

  // Opened from the market, the window picks what to share; from a menu it
  // shares the selection.

  const initial = app.shareTarget!;
  const pick = !!initial.pick;
  let target = $state<ShareTarget>(initial);
  let spriteText = $state(formatIds(initial.kind === "sprites" ? initial.ids : app.selectedSprites.toSorted((a, b) => a - b)));
  let nameTouched = $state(false);
  const spriteCount = $derived(app.project?.info.counts.sprites ?? 0);

  function pickObject(category: Category, id: number) {
    target = { kind: "object", category, id: Math.min(Math.max(id, minId(category)), Math.max(app.maxId(category), minId(category))), pick };
    if (!nameTouched) name = defaultName();
  }

  function pickSprites(text: string) {
    spriteText = text;
    target = { kind: "sprites", ids: parseIds(text, spriteCount), pick };
    if (!nameTouched) name = "";
  }

  // A file from disk: an OBD object or an image cut into sprites.
  const FILE_FILTERS = [
    { name: "Objects and images (*.otobj, *.obd, *.png, *.bmp, *.gif)", pattern: "*.otobj;*.obd;*.png;*.bmp;*.gif" },
    { name: "Object files (*.otobj, *.obd)", pattern: "*.otobj;*.obd" },
    { name: "Images (*.png, *.bmp, *.gif)", pattern: "*.png;*.bmp;*.gif" },
  ];
  const OBD_EXT = OBJECT_EXT;
  let fileSize = $state(app.project?.info.features.spriteSize ?? 32);
  let fileThing = $state<{ thing: Thing; pixels: Uint8ClampedArray[]; size: number } | null>(null);
  let fileImage = $state<{ url: string; width: number; height: number } | null>(null);
  let fileError = $state("");
  const fileName = (p: string) => p.replace(/^.*[\\/]/, "");
  const isObdFile = $derived(target.kind === "file" && OBD_EXT.test(target.path));
  const fileGrid = $derived(!!fileImage && fileImage.width % fileSize === 0 && fileImage.height % fileSize === 0);
  const fileGet = (id: number) => (id > 0 ? fileThing?.pixels[id - 1] : undefined);

  async function chooseFile() {
    const paths = await DialogService.OpenFiles("marketShare", "Publish a file", FILE_FILTERS, false);
    if (paths?.length) await pickFile(paths[0]);
  }

  async function pickFile(path: string) {
    target = { kind: "file", path, pick };
    fileThing = null;
    fileImage = null;
    fileError = "";
    if (!path) return;
    if (!nameTouched) name = fileName(path).replace(/\.[^.]+$/, "").replace(/[_-]+/g, " ");
    try {
      if (OBD_EXT.test(path)) {
        const f = await ThingService.ReadObject(path);
        if (!f?.thing) throw new Error("empty object file");
        fileThing = { thing: normalizeThing(f.thing), pixels: (f.sprites ?? []).map((b) => decodeBytes(b as unknown as string)), size: f.spriteSize };
      } else {
        const img = await ViewerService.ReadImage(path);
        if (!img) throw new Error("empty image");
        fileImage = { url: `data:image/png;base64,${img.png}`, width: img.width, height: img.height };
      }
    } catch (e) {
      fileError = errorMessage(e);
    }
  }

  function defaultName(): string {
    if (target.kind !== "object") return "";
    const own = target.category === app.category && target.id === app.focused ? app.draft?.name : "";
    return own || `${CATEGORY_NAMES[target.category]} ${target.id}`;
  }
  const LICENSE_KEY = "market.license";

  let config = $state<MarketConfig | null>(null);
  let configError = $state("");
  let captcha = $state("");
  let frameKey = $state(0);

  let name = $state(defaultName());
  let author = $state("");
  // Tags from the curated groups or typed by the author.
  let tags = $state<string[]>([]);
  let description = $state("");
  let license = $state(readLicense());
  let allowed = $state(false);
  let sharedId = $state("");

  const tagKind = $derived.by((): TagKind => {
    if (target.kind === "object") return CATEGORY_NAMES[target.category] as TagKind;
    if (target.kind === "file" && fileThing) return CATEGORY_NAMES[fileThing.thing.category] as TagKind;
    return "sprites";
  });
  const tagOptions = $derived(groupOptions(tagGroups(tagKind)));
  const problem = $derived(
    !name.trim()
      ? "Give it a name."
      : name.trim().length > 64
        ? "The name is longer than 64 characters."
        : !author.trim()
          ? "Enter your nickname."
          : author.trim().length > 32
            ? "The nickname is longer than 32 characters."
            : description.length > 500
              ? "The description is longer than 500 characters."
              : tagProblem(tags),
  );
  const what = $derived.by(() => {
    if (target.kind === "object") return `${CATEGORY_NAMES[target.category]} #${target.id}`;
    if (target.kind === "sprites") return `${target.ids.length} sprite(s)`;
    if (!target.path) return "No file chosen";
    if (fileThing) return `${CATEGORY_NAMES[fileThing.thing.category]} · ${fileName(target.path)}`;
    if (fileImage) return `${fileImage.width}×${fileImage.height} image · ${fileName(target.path)}`;
    return fileName(target.path);
  });
  const empty = $derived(
    (target.kind === "sprites" && target.ids.length === 0) ||
      (target.kind === "file" && (!target.path || !!fileError || (isObdFile ? !fileThing : !fileGrid))),
  );

  function readLicense(): string {
    try {
      return localStorage.getItem(LICENSE_KEY) || "OTS-free";
    } catch {
      return "OTS-free";
    }
  }

  onMount(() => {
    MarketService.Config()
      .then((c) => {
        config = c;
        if (c && !author) author = c.author;
      })
      .catch((e) => (configError = errorMessage(e)));
    // The captcha page posts its token; only messages from its origin count.
    const onmessage = (e: MessageEvent) => {
      if (!config || e.origin !== config.captchaOrigin || typeof e.data?.captcha !== "string") return;
      captcha = e.data.captcha;
    };
    window.addEventListener("message", onmessage);
    return () => window.removeEventListener("message", onmessage);
  });



  async function share() {
    if (target.kind === "object" && app.dirty) await applyDraft();
    try {
      localStorage.setItem(LICENSE_KEY, license);
    } catch {
      /* per viewer convenience only */
    }
    const req = { name: name.trim(), author: author.trim(), tags, description: description.trim(), license, captcha };
    const t = target;
    const id = await run("Sharing to the market", () =>
      t.kind === "object"
        ? MarketService.ShareObject(t.category, t.id, req)
        : t.kind === "sprites"
          ? MarketService.ShareSprites(t.ids, req)
          : MarketService.ShareFile(t.path, fileSize, req),
    );
    if (id) {
      sharedId = id;
      toast(`${name.trim()} is in the market.`, "success");
    } else {
      // A captcha token works once; a failed share needs a new one.
      captcha = "";
      frameKey++;
    }
  }
</script>

<Dialog title="Publish to Market" width={470} onclose={() => (app.dialog = null)}>
  {#if sharedId}
    <div class="col done">
      <strong class="ok">Published!</strong>
      <p><b>{name}</b> is in the market now. You can delete it later from the market window on this computer.</p>
      <button class="t-btn" onclick={() => (app.dialog = "market")}>Open the market</button>
    </div>
  {:else if configError || (config && !config.share)}
    <p class="err">{configError || "This build has no market to publish to."}</p>
  {:else}
    <div class="col gap">
      {#if pick}
        <div class="picker">
          <div class="t-tabs">
            <button
              class="t-tab"
              class:on={target.kind === "object"}
              disabled={!app.open}
              title={app.open ? "" : "Open a client to publish its objects"}
              onclick={() => pickObject(app.category, app.focused ?? minId(app.category))}>Object</button
            >
            <button
              class="t-tab"
              class:on={target.kind === "sprites"}
              disabled={!app.open}
              title={app.open ? "" : "Open a client to publish its sprites"}
              onclick={() => pickSprites(spriteText)}>Sprites</button
            >
            <button class="t-tab" class:on={target.kind === "file"} onclick={() => pickFile(target.kind === "file" ? target.path : "")}>File</button>
          </div>
          {#if target.kind === "file"}
            <div class="row pickrow">
              <button class="t-btn" onclick={chooseFile}>Choose…</button>
              <span class="t-label fname" title={target.path}>{target.path ? fileName(target.path) : "An .otobj or .obd file, or an image"}</span>
              {#if target.path && !isObdFile}
                <span class="grow"></span>
                <Select
                  value={fileSize}
                  options={[
                    { value: 32, label: "32 px" },
                    { value: 64, label: "64 px" },
                  ]}
                  width={64}
                  title="Sprite size the image is cut into"
                  onchange={(v) => (fileSize = v)}
                />
              {/if}
            </div>
            {#if fileError}<span class="err">{fileError}</span>{/if}
            {#if fileImage && !fileGrid}<span class="err">The image is not a grid of {fileSize} px sprites.</span>{/if}
          {:else if target.kind === "object"}
            <div class="row pickrow">
              <Select
                value={target.category}
                options={CATEGORIES.map((c) => ({ value: c, label: CATEGORY_LABELS[c] }))}
                width={96}
                onchange={(c) => pickObject(c, minId(c))}
              />
              <NumberField
                label="Id"
                value={target.id}
                min={minId(target.category)}
                max={Math.max(app.maxId(target.category), minId(target.category))}
                width={64}
                onchange={(v) => target.kind === "object" && pickObject(target.category, v)}
              />
              <span class="t-label">of {app.maxId(target.category)}</span>
            </div>
          {:else}
            <label class="field">
              <span class="t-label">Sprite ids, e.g. 100-120, 135 (selected sprites are filled in)</span>
              <input class="t-input" value={spriteText} oninput={(e) => pickSprites(e.currentTarget.value)} placeholder="100-120, 135" />
            </label>
          {/if}
        </div>
      {/if}
      <div class="what row">
        {#if target.kind === "file"}
          {#if fileThing}
            <span class="thumb checker"><ThingCanvas thing={fileThing.thing} get={fileGet} size={fileThing.size} fit={32} /></span>
          {:else if fileImage}
            <span class="thumb checker"><img class="pixel" src={fileImage.url} alt="" /></span>
          {/if}
        {:else if target.kind === "object"}
          <span class="thumb checker"><img class="pixel" src={res.thumb(target.category, target.id, versions.thing(target.category, target.id))} alt="" /></span>
        {:else if target.kind === "sprites"}
          {#each target.ids.slice(0, 8) as id}
            <span class="thumb checker"><img class="pixel" src={res.spritePng(id, versions.sprite(id))} alt="" /></span>
          {/each}
          {#if target.ids.length > 8}<span class="t-label">+{target.ids.length - 8}</span>{/if}
        {/if}
        <span class="t-label">{what}</span>
      </div>

      <div class="form">
        <span class="t-label">Name</span>
        <input class="t-input" maxlength="64" bind:value={name} oninput={() => (nameTouched = true)} placeholder="Fire sword" />
        <span class="t-label">Author</span>
        <input class="t-input" maxlength="32" bind:value={author} placeholder="Your nickname" />
        <span class="t-label">Tags</span>
        <TagPicker value={tags} options={tagOptions} max={MAX_TAGS} allowCustom placeholder="Type to search, e.g. sword, thais" onchange={(t) => (tags = t)} />
        <span class="t-label">About</span>
        <input class="t-input" maxlength="500" bind:value={description} placeholder="Optional description" />
        <span class="t-label">License</span>
        <Select value={license} options={LICENSES} onchange={(v) => (license = v)} />
      </div>
      <label class="row agree">
        <input type="checkbox" class="t-check" bind:checked={allowed} />
        <span>I made this or may publish it. It will be public.</span>
      </label>
      {#if config?.captchaUrl}
        {#key frameKey}
          <iframe class="captcha" title="Captcha" scrolling="no" src={config.captchaUrl}></iframe>
        {/key}
      {/if}
      {#if target.kind === "object" && app.dirty}
        <p class="note">Unapplied editor changes are applied before sharing.</p>
      {/if}
    </div>
  {/if}
  {#snippet footer()}
    {#if problem && name && !sharedId}<span class="err hint">{problem}</span>{/if}
    <span class="grow"></span>
    {#if sharedId || !config?.share}
      <button class="t-btn primary" onclick={() => (app.dialog = null)}>Close</button>
    {:else}
      <button
        class="t-btn primary"
        disabled={!!problem || empty || !allowed || !captcha || !!app.busy}
        title={captcha ? "" : "Solve the captcha first"}
        onclick={share}>Publish</button
      >
      <button class="t-btn" onclick={() => (app.dialog = null)}>Cancel</button>
    {/if}
  {/snippet}
</Dialog>

<style>
  .gap {
    gap: 6px;
  }
  .what {
    gap: 4px;
    min-height: 36px;
  }
  .thumb {
    width: 34px;
    height: 34px;
    display: grid;
    place-items: center;
    border: 1px solid #1a1a1a;
  }
  .thumb img {
    max-width: 32px;
    max-height: 32px;
  }
  .what .t-label {
    margin-left: 4px;
  }
  .picker {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 6px 8px 8px;
    border: 1px solid #2a2a2a;
    background: rgba(0, 0, 0, 0.2);
  }
  .pickrow {
    gap: 8px;
  }
  .fname {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .form {
    display: grid;
    grid-template-columns: 52px 1fr;
    align-items: center;
    gap: 5px 6px;
  }
  .form > .t-label {
    text-align: right;
  }
  .agree {
    gap: 6px;
  }
  .captcha {
    width: 100%;
    height: 70px;
    overflow: hidden;
    border: none;
    background: transparent;
    /* The app is color-scheme: dark and the captcha page is not; with
       different schemes the browser paints the frame white. */
    color-scheme: normal;
  }
  .note {
    margin: 0;
    color: var(--gold);
  }
  .err {
    margin: 0;
    color: #ff9c9c;
  }
  .hint {
    align-self: center;
    max-width: 260px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .done {
    gap: 8px;
  }
  .done p {
    margin: 0;
  }
  .done .t-btn {
    align-self: flex-start;
  }
  .ok {
    color: var(--green);
    font-size: 14px;
  }
</style>
