<script lang="ts">
  // Tibia client progress bar (progressbar.png frame, green fill) with a
  // centered label.
  let { percent, label = "", title = "" }: { percent: number; label?: string; title?: string } = $props();
</script>

<div class="progress" {title} role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(percent)}>
  <div class="fill" style="width:calc((100% - 4px) * {Math.max(0, Math.min(100, percent)) / 100})"></div>
  <span>{label}</span>
</div>

<style>
  .progress {
    position: relative;
    height: 16px;
    display: grid;
    place-items: center;
    color: var(--text-bright);
    text-shadow: 1px 1px 0 #000;
    white-space: nowrap;
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
    background: #3c9a3c;
    transition: width 0.15s linear;
  }
  span {
    position: relative;
    z-index: 1;
  }
</style>
