<script lang="ts">
  import PageContainer from "../components/PageContainer.svelte";
  import type { LogEntry } from "../types";
  import LogContent from "../components/LogContent.svelte";
  import { toast } from "../lib/toast";

  let {
    messageBuffer = [],
    onClearLogs,
  }: {
    messageBuffer: LogEntry[];
    onClearLogs: () => void;
  } = $props();

  function readPref(): boolean {
    try {
      return localStorage.getItem("showDebugLogs") === "true";
    } catch {
      return false;
    }
  }

  function writePref(value: boolean) {
    try {
      localStorage.setItem("showDebugLogs", String(value));
    } catch {
      // Storage can be unavailable; the toggle still works for this window.
    }
  }

  let showDebug = $state(readPref());

  async function copyLogs() {
    const logEntries = messageBuffer
      .filter((entry) => showDebug || entry.level.toLowerCase() !== "debug")
      .map((entry) => {
        const time = new Date(entry.time).toISOString();
        const baseMessage = `[${time}] [${entry.level.padEnd(5)}] ${entry.msg}`;
        const extraFields = Object.entries(entry)
          .filter(([key]: [string, any]) => !["level", "msg", "time", "error"].includes(key))
          .map(([key, value]: [string, any]) => `${key}: ${value}`)
          .join(" | ");
        const parts = [baseMessage];
        if (extraFields) parts.push(extraFields);
        if (entry.error) parts.push(`Error: ${entry.error}`);
        return parts.join(" | ");
      })
      .join("\n");

    try {
      await navigator.clipboard.writeText(logEntries);
      toast.success("Logs copied to clipboard");
    } catch (err) {
      console.error("Failed to copy logs:", err);
      toast.error("Failed to copy logs", err instanceof Error ? err.message : (err?.toString() ?? "unknown error"));
    }
  }
</script>

<PageContainer title="Logs">
  {#snippet description()}
    What Finicky did since it started: the last 1000 lines, in memory.
  {/snippet}
    <div class="log-controls">
      <label class="debug-toggle">
        <input
          type="checkbox"
          bind:checked={showDebug}
          onchange={() => writePref(showDebug)}
        />
        Show debug
      </label>
      <button onclick={onClearLogs}>Clear</button>
      <button onclick={copyLogs}>Copy</button>
    </div>

  <LogContent {messageBuffer} {showDebug} />
</PageContainer>

<style>
  .log-controls {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .debug-toggle {
    display: flex;
    align-items: center;
    gap: 5px;
    cursor: pointer;
    font-size: 0.85em;
    color: var(--text-secondary);
    user-select: none;
  }

  .debug-toggle input[type="checkbox"] {
    cursor: pointer;
    accent-color: var(--accent-color);
  }

  button {
    min-height: 28px;
    padding: 3px 12px;
    border: 1px solid var(--border-color);
    border-radius: 7px;
    background: var(--button-bg);
    color: var(--text-primary);
    cursor: pointer;
    font-size: 0.85em;
    font-family: inherit;
  }

  button:hover {
    background: var(--button-hover);
  }
</style>
