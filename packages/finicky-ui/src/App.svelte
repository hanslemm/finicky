<script lang="ts">
  import LogViewer from "./pages/LogViewer.svelte";
  import Overview from "./pages/Overview.svelte";
  import Settings from "./pages/Settings.svelte";
  import TabBar from "./components/TabBar.svelte";
  import About from "./pages/About.svelte";
  import Rules from "./pages/Rules.svelte";
  import ToastContainer from "./components/ToastContainer.svelte";
  import type { LogEntry, UpdateInfo, ConfigInfo, RulesFile, RecentLink } from "./types";
  import { testUrlResult } from "./lib/testUrlStore";
  import { toast } from "./lib/toast";
  import { router } from "./lib/router.svelte";

  // The Go side keeps the last 1000 log lines too; keep the same bound here.
  const MAX_LOG_ENTRIES = 1000;

  function basename(path: string): string {
    return path.split("/").pop() || path;
  }

  function showPathToast(label: string, description: string, path: string) {
    toast.show(label, "info", `${description}\n${path}`, 5000);
  }

  let version = $state("v0.0.0");

  // Configuration state
  let hasConfig = $state(false);
  let config = $state<ConfigInfo>({ configPath: "" });
  let messageBuffer = $state.raw<LogEntry[]>([]);
  let updateInfo = $state<UpdateInfo | null>(null);
  let rulesFile = $state<RulesFile>({ defaultBrowser: "", rules: [] });
  let installedBrowsers = $state<string[]>([]);
  let profilesByBrowser = $state<Record<string, string[]>>({});
  let recentLinks = $state.raw<RecentLink[]>([]);
  let isDefaultBrowser = $state<boolean | null>(null);

  // A rules file counts as a configuration, the same as a JS config file.
  const configured = $derived(hasConfig || !!rulesFile.path);

  const numErrors = $derived(
    messageBuffer.filter((msg) => msg.level?.toLowerCase() === "error").length
  );

  function appendLog(entry: LogEntry) {
    const next =
      messageBuffer.length >= MAX_LOG_ENTRIES
        ? messageBuffer.slice(messageBuffer.length - MAX_LOG_ENTRIES + 1)
        : messageBuffer.slice();
    next.push(entry);
    messageBuffer = next;
  }

  // Handle a message from the native app.
  function handleMessage(msg: any) {
    const parsedMsg = typeof msg === "string" ? JSON.parse(msg) : msg;
    switch (parsedMsg.type) {
      case "version":
        version = parsedMsg.message;
        break;
      case "buildInfo":
        break;
      case "config":
        hasConfig = true;
        if (parsedMsg.message) {
          config = parsedMsg.message;
        }
        break;
      case "updateInfo":
        updateInfo = parsedMsg.message;
        break;
      case "testUrlResult":
        testUrlResult.set(parsedMsg.message);
        break;
      case "rules":
        rulesFile = parsedMsg.message;
        break;
      case "installedBrowsers":
        installedBrowsers = parsedMsg.message;
        break;
      case "browserProfiles":
        profilesByBrowser = {
          ...profilesByBrowser,
          [parsedMsg.message.browser]: parsedMsg.message.profiles,
        };
        break;
      case "recentLinks":
        recentLinks = parsedMsg.message ?? [];
        break;
      case "defaultBrowserStatus":
        isDefaultBrowser = !!parsedMsg.message?.isDefault;
        break;
      case "saveRulesError":
        toast.show("Rules not saved", "error", parsedMsg.message?.error ?? "Unknown error");
        break;
      default: {
        const entry = parsedMsg.message ? JSON.parse(parsedMsg.message) : parsedMsg;
        appendLog(entry);
      }
    }
  }

  function clearAllLogs() {
    messageBuffer = [];
  }

  // Messages buffered by the WKUserScript stub before Svelte loaded. Without
  // the native host (npm run dev in a browser) there is no stub.
  const preloadQueue: any[] = window.finicky?._queue ?? [];

  window.finicky = {
    sendMessage: (msg: any) => {
      window.webkit?.messageHandlers?.finicky?.postMessage(JSON.stringify(msg));
    },
    receiveMessage: handleMessage,
  };

  for (const msg of preloadQueue) {
    handleMessage(msg);
  }

  // Each page starts at its top.
  let scroller: HTMLElement | undefined = $state();
  $effect(() => {
    router.tab;
    scroller?.scrollTo({ top: 0 });
  });

  // Ask for the rules file at start so the footer and start page are correct.
  window.finicky.sendMessage({ type: "getRules" });
  window.finicky.sendMessage({ type: "getInstalledBrowsers" });
</script>

<main>
  <div class="layout">
    <TabBar {numErrors} needsAttention={isDefaultBrowser === false} />
    <div class="container" bind:this={scroller}>
      <div class="content">
        {#if router.tab === "overview"}
          <Overview
            {configured}
            {isDefaultBrowser}
            {config}
            {rulesFile}
            {numErrors}
            {updateInfo}
            {recentLinks}
          />
        {:else if router.tab === "rules"}
          <Rules {rulesFile} {installedBrowsers} {profilesByBrowser} {config} />
        {:else if router.tab === "settings"}
          <Settings {config} {rulesFile} {updateInfo} />
        {:else if router.tab === "logs"}
          <LogViewer {messageBuffer} onClearLogs={clearAllLogs} />
        {:else}
          <About {version} />
        {/if}
      </div>
    </div>
  </div>
  <footer class="footer">
    {#if configured}
      <span class="config-label">Config loaded:</span>
      {#if hasConfig && config.configPath}
        <button
          class="config-badge"
          onclick={() => showPathToast(basename(config.configPath), "Configuration loaded from", config.configPath)}
        >
          ✓ {basename(config.configPath)}
        </button>
      {/if}
      {#if rulesFile.path && rulesFile.path !== config.configPath}
        <button
          class="config-badge"
          onclick={() => showPathToast(basename(rulesFile.path!), "Rules loaded from", rulesFile.path!)}
        >
          ✓ {basename(rulesFile.path)}
        </button>
      {/if}
    {:else}
      <span class="config-status">Not set up yet</span>
    {/if}
    <span class="spacer"></span>
    {#if updateInfo && updateInfo.updateCheckEnabled && !updateInfo.hasUpdate}
      <span class="up-to-date">✓ Up to date</span>
    {/if}
    <span class="version">{version}</span>
  </footer>
</main>

<ToastContainer />

<style>
  main {
    display: flex;
    flex-direction: column;
    height: 100vh;
    position: relative;
  }

  .layout {
    display: flex;
    flex: 1 1 auto;
    min-height: 0;
  }

  .container {
    padding: 1.25rem 1.5rem;
    max-width: 100%;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    flex: 1 1 100%;
    overflow-y: auto;
    scrollbar-gutter: stable;
  }

  .footer {
    display: flex;
    flex: 0 0 auto;
    align-items: center;
    gap: 0.6rem;
    padding: 0.4rem 0.75rem;
    background: var(--bg-nav);
    border-top: 1px solid var(--border-color);
    overflow: hidden;
    font-size: 0.75em;
    color: var(--text-secondary);
  }

  .version {
    font-variant-numeric: tabular-nums;
    letter-spacing: 0.02em;
  }

  .content {
    flex: 1;
  }

  .spacer {
    flex: 1;
  }

  .up-to-date {
    color: var(--text-success);
  }

  .config-badge {
    font-family: var(--font-mono);
    font-size: 0.96em;
    padding: 2px 8px;
    border-radius: 4px;
    background: transparent;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    white-space: nowrap;
    flex-shrink: 0;
    transition: background 0.15s ease, color 0.15s ease;
  }

  .config-badge:hover {
    background: var(--button-hover);
    color: var(--text-primary);
  }
</style>
