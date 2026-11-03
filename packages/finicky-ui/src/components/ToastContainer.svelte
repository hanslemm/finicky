<script lang="ts">
  import { toast } from "../lib/toast";
  import type { Toast } from "../lib/toast";
  import { fade, fly } from "svelte/transition";
  import { flip } from "svelte/animate";
  import { prefersReducedMotion } from "../lib/motion";

  const reduced = prefersReducedMotion();
  const enter = reduced ? 0 : 180;
  const leave = reduced ? 0 : 160;
  const move = reduced ? 0 : 220;
</script>

<div class="toast-container">
  {#each $toast as t (t.id)}
    <div
      class="toast toast-{t.type}"
      role={t.type === "error" ? "alert" : "status"}
      in:fly={{ y: 24, duration: enter }}
      out:fade={{ duration: leave }}
      animate:flip={{ duration: move }}
    >
      <div class="toast-content">
        <svg class="toast-timer" viewBox="0 0 24 24">
          <circle class="timer-background" cx="12" cy="12" r="10" />
          {#key t.key}
            <circle
              class="timer-progress"
              cx="12"
              cy="12"
              r="10"
              style="animation-duration: {t.duration}ms"
            />
          {/key}
        </svg>
        <span class="toast-message">{t.message}</span>
        {#if t.action}
          <button
            class="toast-action"
            type="button"
            onclick={() => {
              t.action?.run();
              toast.remove(t.id);
            }}>{t.action.label}</button
          >
        {/if}
        <button class="toast-close" type="button" aria-label="Dismiss" onclick={() => toast.remove(t.id)}>×</button>
      </div>
      {#if t.extra}
        <pre class="toast-extra">{t.extra}</pre>
      {/if}
    </div>
  {/each}
</div>

<style>
  .toast-container {
    position: fixed;
    bottom: 60px;
    right: 24px;
    z-index: 9999;
    display: flex;
    flex-direction: column-reverse;
    gap: 12px;
    pointer-events: none;
  }

  .toast {
    display: flex;
    flex-direction: column;
    align-items: left;

    gap: 12px;
    padding: 12px 16px;
    border-radius: 8px;
    background: var(--log-bg);
    border: 1px solid var(--border-color);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
    min-width: 250px;
    max-width: 400px;
    pointer-events: auto;
    position: relative;
  }

  .toast-content {
    display: flex;
    align-items: center;
    gap: 12px;
    flex: 1;
  }

  .toast-timer {
    width: 20px;
    height: 20px;
    flex-shrink: 0;
    transform: rotate(-90deg);
  }

  .timer-background {
    fill: none;
    stroke: var(--border-color);
    stroke-width: 2;
    opacity: 0.3;
  }

  .timer-progress {
    fill: none;
    stroke: currentColor;
    stroke-width: 2;
    stroke-dasharray: 62.83;
    stroke-dashoffset: 0;
    animation: circleProgress linear forwards;
    opacity: 0.6;
  }

  @keyframes circleProgress {
    to {
      stroke-dashoffset: 62.83;
    }
  }

  .toast-message {
    flex: 1;
    font-size: 0.9em;
    line-height: 1.4;
  }

  .toast-action {
    background: none;
    border: 1px solid currentColor;
    border-radius: 6px;
    color: inherit;
    font: inherit;
    font-size: 0.85em;
    font-weight: 600;
    padding: 3px 10px;
    cursor: pointer;
  }

  .toast-close {
    background: none;
    border: none;
    color: var(--text-secondary);
    font-size: 1.1em;
    line-height: 1;
    width: 24px;
    height: 24px;
    border-radius: 6px;
    cursor: pointer;
  }

  .toast-close:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .toast-extra {
    font-family: var(--font-mono);
    background: var(--inset-bg);
    color: var(--text-primary);
    padding: 8px 10px;
    border-radius: 6px;
    font-size: 0.85em;
    overflow-x: auto;
    white-space: pre-wrap;
  }

  .toast-success {
    color: var(--text-success);
  }

  .toast-success .toast-message {
    color: var(--text-success);
  }

  .toast-error {
    color: var(--text-error);
  }

  .toast-error .toast-message {
    color: var(--text-error);
  }

  .toast-warning {
    color: var(--text-warning);
  }

  .toast-warning .toast-message {
    color: var(--text-warning);
  }

  .toast-info {
    color: var(--accent-color);
  }

  .toast-info .toast-message {
    color: var(--accent-color);
  }
</style>
