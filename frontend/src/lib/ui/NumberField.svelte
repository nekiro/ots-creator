<script lang="ts">
  // OTClient SpinBox: TextEdit with up/down buttons (spinbox_up/down).
  // Mouse wheel works while focused; holding a button repeats.
  let {
    label,
    value,
    min = 0,
    max = 65535,
    width = 56,
    disabled = false,
    title = "",
    onchange,
  }: {
    label?: string;
    value: number;
    min?: number;
    max?: number;
    width?: number;
    disabled?: boolean;
    title?: string;
    onchange: (v: number) => void;
  } = $props();

  let current = $state(0);
  $effect.pre(() => {
    current = value;
  });

  function commit(v: number) {
    if (!Number.isFinite(v)) v = min;
    v = Math.min(Math.max(Math.trunc(v), min), max);
    current = v;
    if (v !== value) onchange(v);
  }

  let timer: ReturnType<typeof setTimeout> | undefined;
  function hold(delta: number, e: PointerEvent) {
    if (disabled || e.button !== 0) return;
    e.preventDefault();
    const step = () => commit(current + delta);
    step();
    let wait = 350;
    const again = () => {
      step();
      wait = Math.max(30, wait * 0.8);
      timer = setTimeout(again, wait);
    };
    timer = setTimeout(again, wait);
  }
  function release() {
    clearTimeout(timer);
  }

  function onwheel(e: WheelEvent) {
    if (disabled || document.activeElement !== e.currentTarget) return;
    e.preventDefault();
    commit(current + (e.deltaY < 0 ? 1 : -1) * (e.shiftKey ? 10 : 1));
  }

  function onkeydown(e: KeyboardEvent) {
    const input = e.currentTarget as HTMLInputElement;
    if (e.key === "Enter") commit(Number(input.value));
    else if (e.key === "ArrowUp" || e.key === "ArrowDown") {
      e.preventDefault();
      commit(current + (e.key === "ArrowUp" ? 1 : -1) * (e.shiftKey ? 10 : 1));
    }
  }
</script>

<svelte:window onpointerup={release} onblur={release} />

<label class="nf" {title}>
  {#if label}<span class="t-label">{label}</span>{/if}
  <span class="spin" class:disabled style="width:{width + 11}px">
    <input
      class="t-input"
      inputmode="numeric"
      {disabled}
      value={current}
      onchange={(e) => commit(Number(e.currentTarget.value))}
      {onkeydown}
      {onwheel}
    />
    <button type="button" class="up" tabindex="-1" aria-label="Increase" {disabled} onpointerdown={(e) => hold(1, e)}></button>
    <button type="button" class="down" tabindex="-1" aria-label="Decrease" {disabled} onpointerdown={(e) => hold(-1, e)}></button>
  </span>
</label>

<style>
  .nf {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .spin {
    position: relative;
    display: inline-block;
    height: 20px;
    flex: none;
  }
  .spin input {
    width: 100%;
    height: 20px;
    padding-right: 13px;
  }
  .up,
  .down {
    position: absolute;
    right: 0;
    width: 10px;
    height: 10px;
    padding: 0;
    border: 0;
    background: url("../../assets/ui/spin-up-idle.png");
    image-rendering: pixelated;
    cursor: pointer;
  }
  .up {
    top: 0;
  }
  .down {
    bottom: 0;
    background-image: url("../../assets/ui/spin-down-idle.png");
  }
  .up:hover:not(:disabled) {
    background-image: url("../../assets/ui/spin-up-hover.png");
  }
  .up:active:not(:disabled) {
    background-image: url("../../assets/ui/spin-up-pressed.png");
  }
  .down:hover:not(:disabled) {
    background-image: url("../../assets/ui/spin-down-hover.png");
  }
  .down:active:not(:disabled) {
    background-image: url("../../assets/ui/spin-down-pressed.png");
  }
  .spin.disabled .up,
  .spin.disabled .down {
    opacity: 0.5;
    cursor: default;
  }
</style>
