<script lang="ts">
  import type { LogEntry } from "../types";
  import { formatLogEntry } from "../utils/text";

  let { messageBuffer = [], showDebug = true }: { messageBuffer: LogEntry[]; showDebug?: boolean } = $props();

  const visible = $derived(
    showDebug ? messageBuffer : messageBuffer.filter((e) => e.level?.toLowerCase() !== "debug")
  );
</script>

{#if visible.length === 0}
  <p class="empty">No log lines yet. Open a link, or turn on “Show debug”.</p>
{:else}
  <ol class="log-content">
    {#each visible as entry}
      <li class="log-entry">
        <time class="log-time" datetime={entry.time} title={entry.time}>
          {new Date(entry.time).toLocaleTimeString()}
        </time>
        <span class="log-level log-level-{entry.level?.toLowerCase()}">{entry.level}</span>
        <div class="log-message log-level-{entry.level?.toLowerCase()}">
          {#each formatLogEntry(entry) as part}
            {#if part.type === "url"}
              <a href={part.content} target="_blank" rel="noopener noreferrer">{part.content}</a>
            {:else}
              {part.content}
            {/if}
          {/each}
        </div>
      </li>
    {/each}
  </ol>
{/if}

<style>
  .empty {
    margin: 0;
    padding: 18px;
    text-align: center;
    color: var(--text-secondary);
    font-size: 0.86em;
    border: 1px dashed var(--border-color);
    border-radius: 12px;
  }

  .log-content {
    list-style: none;
    margin: 0;
    padding: 10px 12px;
    font-family: var(--font-mono);
    font-size: 0.82em;
    line-height: 1.5;
    color: var(--text-primary);
    background: var(--card-bg);
    border: 1px solid var(--card-border);
    border-radius: 12px;
  }

  .log-entry {
    display: grid;
    grid-template-columns: auto 3.6em minmax(0, 1fr);
    gap: 0.9em;
    padding: 3px 0;
    align-items: baseline;
  }

  .log-time,
  .log-level {
    color: var(--text-secondary);
    white-space: nowrap;
  }

  .log-level {
    text-transform: lowercase;
  }

  .log-message {
    white-space: pre-wrap;
    word-break: break-word;
  }

  .log-level-error {
    color: var(--text-error);
  }

  .log-level-warn {
    color: var(--text-warning);
  }

  .log-level-debug {
    color: var(--log-debug);
  }

  .log-message a {
    color: inherit;
    text-decoration: underline;
    text-decoration-style: dotted;
  }

  .log-message a:hover {
    text-decoration-style: solid;
  }
</style>
