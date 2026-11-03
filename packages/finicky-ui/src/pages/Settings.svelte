<script lang="ts">
  import { onDestroy, untrack } from "svelte";
  import PageContainer from "../components/PageContainer.svelte";
  import OptionRow from "../components/OptionRow.svelte";
  import type { ConfigInfo, ConfigOptions, RulesFile, UpdateInfo } from "../types";

  let {
    config,
    rulesFile,
    updateInfo,
  }: {
    config: ConfigInfo;
    rulesFile: RulesFile;
    updateInfo: UpdateInfo | null;
  } = $props();

  const DEFAULTS: ConfigOptions = {
    keepRunning: true,
    hideIcon: false,
    logRequests: false,
    checkForUpdates: true,
    resolveShortUrls: true,
  };

  const isJSConfig = $derived(config.isJSConfig ?? false);

  // With a JS config, the options come from it; else from rules.json.
  function current(): ConfigOptions {
    const source = isJSConfig ? config.options : rulesFile.options;
    return { ...DEFAULTS, ...(source ?? {}) };
  }

  let options = $state<ConfigOptions>(untrack(current));
  let saveTimer: ReturnType<typeof setTimeout> | undefined;
  let saved = $state(false);

  $effect(() => {
    rulesFile;
    config;
    if (untrack(() => saveTimer === undefined)) options = current();
  });

  function save() {
    clearTimeout(saveTimer);
    saveTimer = undefined;
    const { path: _path, ...rest } = rulesFile;
    window.finicky.sendMessage({
      type: "saveRules",
      payload: { ...rest, options: $state.snapshot(options) },
    });
    saved = true;
    setTimeout(() => (saved = false), 2000);
  }

  function scheduleSave() {
    if (isJSConfig) return;
    clearTimeout(saveTimer);
    saveTimer = setTimeout(save, 300);
  }

  onDestroy(() => {
    if (saveTimer !== undefined) save();
  });

  const configName = $derived(config.configPath ? config.configPath : rulesFile.path ?? "");
</script>

<PageContainer title="Settings">
  {#snippet description()}
    {#if isJSConfig}
      Your JS config sets these options. Change them in the file.
    {:else}
      Changes save at once.
      <span class="saved" role="status">{saved ? "Saved" : ""}</span>
    {/if}
  {/snippet}

  <section class="group" aria-labelledby="g-behavior">
    <h3 id="g-behavior">Behavior</h3>
    <div class="rows">
      <OptionRow
        label="Keep running"
        hint="Stay open after a link. Links then open faster, because Finicky does not start each time."
        bind:checked={options.keepRunning}
        locked={isJSConfig}
        onchange={scheduleSave}
      />
      <OptionRow
        label="Expand short links"
        hint="Follow bit.ly, t.co and similar links before the rules run. Costs up to 0.75 s per short link, and the shortener sees the request."
        bind:checked={options.resolveShortUrls}
        locked={isJSConfig}
        onchange={scheduleSave}
      />
      <OptionRow
        label="Hide menu bar icon"
        hint="Open this window from Spotlight or Launchpad instead."
        bind:checked={options.hideIcon}
        locked={isJSConfig}
        onchange={scheduleSave}
      />
    </div>
  </section>

  <section class="group" aria-labelledby="g-privacy">
    <h3 id="g-privacy">Privacy and updates</h3>
    <div class="rows">
      <OptionRow
        label="Write logs to a file"
        hint="Saves each link to ~/Library/Logs/Finicky. Useful to find a problem; the file holds every URL that you open."
        bind:checked={options.logRequests}
        locked={isJSConfig}
        onchange={scheduleSave}
      />
      <OptionRow
        label="Check for updates"
        hint={updateInfo?.hasUpdate
          ? `Version ${updateInfo.version} is available.`
          : "Ask once a day if a new version exists."}
        bind:checked={options.checkForUpdates}
        locked={isJSConfig}
        onchange={scheduleSave}
      />
    </div>
  </section>

  <section class="group" aria-labelledby="g-file">
    <h3 id="g-file">Configuration file</h3>
    <div class="file">
      {#if configName}
        <code class="path">{configName}</code>
        <div class="file-actions">
          <button onclick={() => window.finicky.sendMessage({ type: "openConfig" })}>Open</button>
          <button onclick={() => window.finicky.sendMessage({ type: "revealConfig" })}>Show in Finder</button>
        </div>
      {:else}
        <p class="muted">
          No file yet. Finicky makes <code>rules.json</code> when you add a rule. For more control, write
          <code>~/.finicky.js</code>.
          <a href="https://github.com/johnste/finicky/wiki/Configuration-(v4)" target="_blank" rel="noopener noreferrer">Read the guide</a>
        </p>
      {/if}
    </div>
  </section>
</PageContainer>

<style>
  .group {
    display: grid;
    gap: 8px;
  }

  h3 {
    margin: 0;
    padding-left: 2px;
    font-size: 0.82em;
    font-weight: 600;
    color: var(--text-secondary);
  }

  .rows {
    background: var(--card-bg);
    border: 1px solid var(--card-border);
    border-radius: 12px;
    overflow: hidden;
  }

  .rows > :global(.option-row + .option-row) {
    border-top: 1px solid var(--border-color);
  }

  .saved {
    margin-left: 6px;
    color: var(--text-success);
  }

  .file {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    padding: 12px 14px;
    background: var(--card-bg);
    border: 1px solid var(--card-border);
    border-radius: 12px;
  }

  .path {
    font-family: var(--font-mono);
    font-size: 0.84em;
    color: var(--text-primary);
    word-break: break-all;
  }

  code {
    font-family: var(--font-mono);
    font-size: 0.92em;
  }

  .file-actions {
    display: flex;
    gap: 6px;
  }

  button {
    min-height: 28px;
    padding: 4px 12px;
    border-radius: 7px;
    border: 1px solid var(--border-color);
    background: var(--button-bg);
    color: var(--text-primary);
    font: inherit;
    font-size: 0.85em;
    cursor: pointer;
  }

  button:hover {
    background: var(--button-hover);
  }

  .muted {
    margin: 0;
    color: var(--text-secondary);
    font-size: 0.86em;
  }
</style>
