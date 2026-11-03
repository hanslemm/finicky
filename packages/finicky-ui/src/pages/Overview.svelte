<script lang="ts">
  import PageContainer from "../components/PageContainer.svelte";
  import type { ConfigInfo, RecentLink, RulesFile, UpdateInfo } from "../types";
  import { navigate } from "../lib/router.svelte";
  import { draft, patternForUrl } from "../lib/draftRule.svelte";
  import { testUrlInput } from "../lib/testUrlStore";

  let {
    configured,
    isDefaultBrowser,
    config,
    rulesFile,
    numErrors,
    updateInfo,
    recentLinks,
  }: {
    configured: boolean;
    /** null while the app has not reported it yet. */
    isDefaultBrowser: boolean | null;
    config: ConfigInfo;
    rulesFile: RulesFile;
    numErrors: number;
    updateInfo: UpdateInfo | null;
    recentLinks: RecentLink[];
  } = $props();

  let askedForDefault = $state(false);

  const isJSConfig = $derived(config.isJSConfig ?? false);
  const ruleCount = $derived(rulesFile.rules?.length ?? 0);
  const configName = $derived(
    isJSConfig ? basename(config.configPath) : rulesFile.path ? "rules.json" : ""
  );
  const fallback = $derived(
    isJSConfig ? config.defaultBrowser || "—" : rulesFile.defaultBrowser || "Safari"
  );

  function basename(path: string): string {
    return path.split("/").pop() || path;
  }

  function makeDefault() {
    askedForDefault = true;
    window.finicky.sendMessage({ type: "makeDefaultBrowser" });
  }

  function host(url: string): string {
    try {
      return new URL(url).hostname.replace(/^www\./, "");
    } catch {
      return url;
    }
  }

  function rest(url: string): string {
    try {
      const u = new URL(url);
      const tail = u.pathname + u.search;
      return tail === "/" ? "" : tail;
    } catch {
      return "";
    }
  }

  function when(iso: string): string {
    const d = new Date(iso);
    const s = Math.round((Date.now() - d.getTime()) / 1000);
    if (s < 60) return "just now";
    if (s < 3600) return `${Math.floor(s / 60)} min ago`;
    return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  }

  function decidedBy(link: RecentLink): string {
    switch (link.source) {
      case "rule":
        return `Rule ${(link.ruleIndex ?? 0) + 1}`;
      case "js":
        return "JS config";
      case "error":
        return "Error";
      default:
        return "Default";
    }
  }

  function makeRule(link: RecentLink) {
    draft.rule = { pattern: patternForUrl(link.url), browser: link.browser, profile: link.profile };
    navigate("rules");
  }

  function testLink(link: RecentLink) {
    testUrlInput.set(link.url);
    navigate("rules");
  }
</script>

<PageContainer title="Overview">
  {#snippet description()}Finicky opens each link in the browser that your rules choose.{/snippet}

  <div class="status-grid">
    <!-- Default browser -->
    <div class="status" class:attention={isDefaultBrowser === false}>
      <span class="dot" class:ok={isDefaultBrowser} class:warn={isDefaultBrowser === false} aria-hidden="true"></span>
      <div class="status-body">
        <span class="status-title">
          {#if isDefaultBrowser === null}
            Checking default browser…
          {:else if isDefaultBrowser}
            Finicky is your default browser
          {:else}
            Finicky is not your default browser
          {/if}
        </span>
        <span class="status-text">
          {#if isDefaultBrowser === false}
            Links from other apps go straight to another browser, and your rules do not run.
          {:else}
            Links that you click in other apps come to Finicky first.
          {/if}
        </span>
        {#if isDefaultBrowser === false}
          <div class="status-actions">
            <button class="primary" onclick={makeDefault}>Make Finicky the default browser</button>
            {#if askedForDefault}
              <span class="status-text" role="status">Confirm the change in the macOS dialog.</span>
            {/if}
          </div>
        {/if}
      </div>
    </div>

    <!-- Configuration -->
    <div class="status" class:attention={!configured || numErrors > 0}>
      <span
        class="dot"
        class:ok={configured && numErrors === 0}
        class:warn={!configured}
        class:bad={numErrors > 0}
        aria-hidden="true"
      ></span>
      <div class="status-body">
        <span class="status-title">
          {#if !configured}
            No rules yet
          {:else if isJSConfig}
            JS config <code>{configName}</code> is active
          {:else}
            {ruleCount} {ruleCount === 1 ? "rule" : "rules"}, otherwise {fallback}
          {/if}
        </span>
        <span class="status-text">
          {#if numErrors > 0}
            {numErrors} {numErrors === 1 ? "error" : "errors"} since start.
            <a href="#logs">Show the logs</a>
          {:else if !configured}
            Every link opens in Safari until you add a rule or a default browser.
          {:else if isJSConfig}
            Its handlers run first. Rules from this window run after them.
          {:else}
            Edit the rules in this window. No file editing is necessary.
          {/if}
        </span>
        {#if configured}
          <div class="status-actions">
            <button onclick={() => navigate("rules")}>Edit rules</button>
            <button onclick={() => window.finicky.sendMessage({ type: "openConfig" })}>Open {isJSConfig ? "config" : "rules"} file</button>
            <button class="quiet" onclick={() => window.finicky.sendMessage({ type: "revealConfig" })}>Show in Finder</button>
          </div>
        {/if}
      </div>
    </div>

    {#if updateInfo?.hasUpdate}
      <div class="status">
        <span class="dot info" aria-hidden="true"></span>
        <div class="status-body">
          <span class="status-title">Finicky {updateInfo.version} is available</span>
          <div class="status-actions">
            <a class="button primary" href={updateInfo.downloadUrl} target="_blank" rel="noopener noreferrer">Download</a>
            <a class="button quiet" href={updateInfo.releaseUrl} target="_blank" rel="noopener noreferrer">Release notes</a>
          </div>
        </div>
      </div>
    {/if}
  </div>

  {#if !configured}
    <section class="setup" aria-labelledby="setup-title">
      <h3 id="setup-title">Set up in three steps</h3>
      <ol>
        <li class:done={isDefaultBrowser}>
          <span class="step-title">Make Finicky your default browser</span>
          <span class="step-text">Then links from Mail, Slack and other apps come to Finicky.</span>
          {#if isDefaultBrowser === false}
            <button class="primary" onclick={makeDefault}>Make default</button>
          {/if}
        </li>
        <li>
          <span class="step-title">Add a rule</span>
          <span class="step-text">For example: links that match <code>*zoom.us/*</code> open in zoom.us.</span>
          <button class="primary" onclick={() => navigate("rules")}>Add a rule</button>
        </li>
        <li>
          <span class="step-title">Test a link</span>
          <span class="step-text">The Rules page shows which rule wins for any URL.</span>
        </li>
      </ol>
      <p class="setup-more">
        Need more control? Write a JavaScript config.
        <a href="https://github.com/johnste/finicky/wiki/Configuration-(v4)" target="_blank" rel="noopener noreferrer">Read the guide</a>
      </p>
    </section>
  {/if}

  <section aria-labelledby="recent-title">
    <div class="section-head">
      <h3 id="recent-title">Recent links</h3>
      <span class="section-note">Kept in memory only, until Finicky quits.</span>
    </div>
    {#if recentLinks.length === 0}
      <p class="empty">Links that Finicky opens show here, with the rule that decided each one.</p>
    {:else}
      <ul class="links">
        {#each recentLinks as link (link.id)}
          <li class="link">
            <div class="link-main">
              <span class="link-url" title={link.url}><strong>{host(link.url)}</strong><span class="link-rest">{rest(link.url)}</span></span>
              <span class="link-meta">
                {link.opener ? `from ${link.opener} · ` : ""}{when(link.time)}
              </span>
            </div>
            <div class="link-result">
              <span class="link-browser">{link.profile ? `${link.browser} · ${link.profile}` : link.browser}</span>
              <span class="badge" class:badge-rule={link.source === "rule"} class:badge-error={link.source === "error"}>{decidedBy(link)}</span>
            </div>
            <div class="link-actions">
              <button class="quiet" onclick={() => testLink(link)} aria-label="Test {host(link.url)}">Test</button>
              <button onclick={() => makeRule(link)} aria-label="Make a rule for {host(link.url)}">Make a rule</button>
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </section>
</PageContainer>

<style>
  .status-grid {
    display: grid;
    gap: 10px;
  }

  .status {
    display: flex;
    gap: 12px;
    padding: 14px 16px;
    background: var(--card-bg);
    border: 1px solid var(--card-border);
    border-radius: 12px;
  }

  .status.attention {
    border-color: color-mix(in srgb, var(--text-warning) 45%, var(--card-border));
  }

  .dot {
    flex-shrink: 0;
    width: 9px;
    height: 9px;
    margin-top: 6px;
    border-radius: 50%;
    background: var(--text-secondary);
  }

  .dot.ok {
    background: var(--log-success);
  }

  .dot.warn {
    background: var(--log-warning);
  }

  .dot.bad {
    background: var(--log-error);
  }

  .dot.info {
    background: var(--accent-fill);
  }

  .status-body {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .status-title {
    color: var(--text-primary);
    font-weight: 600;
    font-size: 0.95em;
  }

  .status-text {
    color: var(--text-secondary);
    font-size: 0.85em;
  }

  .status-actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    margin-top: 10px;
  }

  code {
    font-family: var(--font-mono);
    font-size: 0.9em;
    background: var(--inset-bg);
    padding: 1px 5px;
    border-radius: 4px;
  }

  button,
  .button {
    display: inline-flex;
    align-items: center;
    min-height: 28px;
    padding: 4px 12px;
    border-radius: 7px;
    border: 1px solid var(--border-color);
    background: var(--button-bg);
    color: var(--text-primary);
    font: inherit;
    font-size: 0.85em;
    text-decoration: none;
    cursor: pointer;
    transition: background 0.15s ease;
  }

  button:hover,
  .button:hover {
    background: var(--button-hover);
  }

  .primary {
    background: var(--accent-fill);
    border-color: var(--accent-fill);
    color: var(--accent-fill-text);
    font-weight: 500;
  }

  .primary:hover {
    background: var(--accent-fill);
    filter: brightness(1.1);
  }

  .quiet {
    background: transparent;
    border-color: transparent;
    color: var(--text-secondary);
  }

  .quiet:hover {
    color: var(--text-primary);
  }

  h3 {
    margin: 0;
    font-size: 0.95em;
    font-weight: 600;
    color: var(--text-primary);
  }

  .setup {
    padding: 16px;
    background: var(--card-bg);
    border: 1px solid var(--card-border);
    border-radius: 12px;
  }

  .setup ol {
    list-style: none;
    counter-reset: step;
    margin: 12px 0 0;
    padding: 0;
    display: grid;
    gap: 12px;
  }

  .setup li {
    counter-increment: step;
    position: relative;
    padding-left: 34px;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }

  .setup li::before {
    content: counter(step);
    position: absolute;
    left: 0;
    top: 0;
    width: 22px;
    height: 22px;
    border-radius: 50%;
    display: grid;
    place-items: center;
    font-size: 0.78em;
    font-weight: 600;
    background: var(--inset-bg);
    color: var(--text-primary);
    border: 1px solid var(--border-color);
  }

  .setup li.done::before {
    content: "✓";
    background: var(--log-success);
    border-color: var(--log-success);
    color: #fff;
  }

  .step-title {
    color: var(--text-primary);
    font-weight: 500;
    font-size: 0.9em;
  }

  .step-text,
  .setup-more {
    color: var(--text-secondary);
    font-size: 0.85em;
  }

  .setup-more {
    margin: 14px 0 0;
  }

  .section-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 12px;
    margin-bottom: 8px;
  }

  .section-note,
  .empty {
    color: var(--text-secondary);
    font-size: 0.82em;
  }

  .empty {
    margin: 0;
    padding: 18px;
    text-align: center;
    border: 1px dashed var(--border-color);
    border-radius: 12px;
  }

  .links {
    list-style: none;
    margin: 0;
    padding: 0;
    border: 1px solid var(--card-border);
    border-radius: 12px;
    background: var(--card-bg);
    overflow: hidden;
  }

  .link {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto auto;
    align-items: center;
    gap: 14px;
    padding: 9px 12px 9px 14px;
    border-bottom: 1px solid var(--border-color);
  }

  .link:last-child {
    border-bottom: none;
  }

  .link-main {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .link-url {
    font-size: 0.88em;
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .link-url strong {
    font-weight: 600;
  }

  .link-rest {
    color: var(--text-secondary);
  }

  .link-meta {
    color: var(--text-secondary);
    font-size: 0.78em;
  }

  .link-result {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 2px;
    font-size: 0.82em;
  }

  .link-browser {
    color: var(--text-primary);
    white-space: nowrap;
  }

  .badge {
    font-size: 0.9em;
    padding: 0 6px;
    border-radius: 4px;
    background: var(--inset-bg);
    color: var(--text-secondary);
    white-space: nowrap;
  }

  .badge-rule {
    color: var(--accent-color);
  }

  .badge-error {
    color: var(--text-error);
  }

  .link-actions {
    display: flex;
    gap: 4px;
  }

  .link-actions button {
    min-height: 26px;
    padding: 2px 10px;
  }
</style>
