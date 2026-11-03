<script lang="ts">
  import LockIcon from "./icons/Lock.svelte";

  let {
    label,
    hint,
    checked = $bindable(false),
    locked = false,
    onchange,
  }: {
    label: string;
    hint: string;
    checked: boolean;
    /** The JS config owns this option; show it read-only. */
    locked?: boolean;
    onchange?: () => void;
  } = $props();

  const id = `opt-${Math.random().toString(36).slice(2, 9)}`;
</script>

<label class="option-row" class:locked for={id}>
  <span class="option-text">
    <span class="option-label">{label}</span>
    <span class="option-hint" id="{id}-hint">
      {hint}
      {#if locked}<span class="locked-note"><LockIcon /> Set in your JS config</span>{/if}
    </span>
  </span>
  <span class="toggle">
    <input
      {id}
      type="checkbox"
      role="switch"
      bind:checked
      disabled={locked}
      aria-describedby="{id}-hint"
      {onchange}
    />
    <span class="toggle-track" aria-hidden="true"><span class="toggle-knob"></span></span>
  </span>
</label>

<style>
  .option-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 12px 14px;
    cursor: pointer;
    transition: background 0.15s ease;
  }

  .option-row:hover:not(.locked) {
    background: var(--bg-hover);
  }

  .option-row.locked {
    cursor: default;
  }

  .option-text {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .option-label {
    color: var(--text-primary);
    font-size: 0.92em;
    font-weight: 500;
  }

  .option-hint {
    color: var(--text-secondary);
    font-size: 0.82em;
    line-height: 1.4;
  }

  .locked-note {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    margin-left: 6px;
    white-space: nowrap;
  }

  .locked-note :global(svg) {
    width: 11px;
    height: 11px;
  }

  .toggle {
    position: relative;
    flex-shrink: 0;
    width: 38px;
    height: 22px;
  }

  .toggle input {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    margin: 0;
    opacity: 0;
    cursor: inherit;
  }

  .toggle-track {
    position: absolute;
    inset: 0;
    border-radius: 11px;
    background: var(--toggle-off);
    transition: background 0.18s ease;
  }

  .toggle-knob {
    position: absolute;
    top: 2px;
    left: 2px;
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: var(--toggle-knob);
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.3);
    transition: transform 0.18s cubic-bezier(0.2, 0.8, 0.2, 1);
  }

  .toggle input:checked + .toggle-track {
    background: var(--toggle-active);
  }

  .toggle input:checked + .toggle-track .toggle-knob {
    transform: translateX(16px);
    background: var(--toggle-knob-active);
  }

  .toggle input:focus-visible + .toggle-track {
    outline: 2px solid var(--focus-ring);
    outline-offset: 2px;
  }

  .toggle input:disabled + .toggle-track {
    opacity: 0.55;
  }
</style>
