<script lang="ts">
  import { untrack, tick, onMount, onDestroy } from "svelte";
  import PageContainer from "../components/PageContainer.svelte";
  import BrowserProfileSelector from "../components/BrowserProfileSelector.svelte";
  import WarningIcon from "../components/icons/Warning.svelte";
  import XIcon from "../components/icons/X.svelte";
  import type { ConfigInfo, Rule, RulesFile } from "../types";
  import { toast } from "../lib/toast";
  import { draft } from "../lib/draftRule.svelte";
  import { testUrlInput, testUrlResult } from "../lib/testUrlStore";

  let {
    rulesFile = { defaultBrowser: "", rules: [] },
    installedBrowsers = [],
    profilesByBrowser = {},
    config,
  }: {
    rulesFile: RulesFile;
    installedBrowsers: string[];
    profilesByBrowser: Record<string, string[]>;
    config: ConfigInfo;
  } = $props();

  const SAVE_DEBOUNCE = 500;
  const SAFARI = "Safari";

  const isJSConfig = $derived(config.isJSConfig ?? false);

  // ---- Local editable state -------------------------------------------------

  let rules = $state<Rule[]>([]);
  let rowIsCustom = $state<boolean[]>([]);
  let rowProfileIsCustom = $state<boolean[]>([]);
  let defaultBrowser = $state("");
  let defaultProfile = $state("");

  type SaveState = "idle" | "pending" | "saved";
  let saveState = $state<SaveState>("idle");
  let saveTimer: ReturnType<typeof setTimeout> | undefined;
  let savedTimer: ReturnType<typeof setTimeout> | undefined;

  let announcement = $state("");
  let focusTarget = $state<{ rule: number; pattern: number } | null>(null);

  function normalize(r: Rule): Rule {
    return {
      ...r,
      match: Array.isArray(r.match) ? r.match : r.match ? [r.match as unknown as string] : [""],
    };
  }

  // Take incoming props into local state, except while a local edit waits to be saved.
  $effect(() => {
    const incoming = rulesFile;
    if (untrack(() => saveState === "pending")) return;
    const newRules = (incoming.rules ?? []).map(normalize);
    rules = newRules;
    defaultBrowser = incoming.defaultBrowser || SAFARI;
    defaultProfile = incoming.defaultProfile ?? "";
    rowIsCustom = newRules.map((r) => r.browser !== "" && !untrack(() => installedBrowsers).includes(r.browser));
    rowProfileIsCustom = newRules.map(() => false);
    untrack(() => requestProfiles([...newRules.map((r) => r.browser), defaultBrowser]));
  });

  // Correct the "custom" flags when the browser and profile lists arrive.
  $effect(() => {
    rules.forEach((r, i) => {
      if (r.browser) rowIsCustom[i] = !installedBrowsers.includes(r.browser);
      const profile = r.profile ?? "";
      const profiles = profilesByBrowser[r.browser];
      if (profile && profiles !== undefined) rowProfileIsCustom[i] = !profiles.includes(profile);
    });
  });

  const defaultIsCustom = $derived(defaultBrowser !== "" && !installedBrowsers.includes(defaultBrowser));
  const defaultProfileIsCustom = $derived(
    defaultProfile !== "" && !(profilesByBrowser[defaultBrowser] ?? []).includes(defaultProfile)
  );

  function requestProfiles(browsers: string[]) {
    for (const b of new Set(browsers)) {
      if (b && profilesByBrowser[b] === undefined) {
        window.finicky.sendMessage({ type: "getBrowserProfiles", browser: b });
      }
    }
  }

  // ---- Saving ---------------------------------------------------------------

  function payload(): RulesFile {
    // Keep every other field of the file (options!) as it is.
    const { path: _path, ...rest } = rulesFile;
    return {
      ...rest,
      defaultBrowser: defaultBrowser === SAFARI && !rulesFile.defaultBrowser ? "" : defaultBrowser,
      defaultProfile,
      rules: $state.snapshot(rules),
    };
  }

  function save() {
    clearTimeout(saveTimer);
    saveTimer = undefined;
    window.finicky.sendMessage({ type: "saveRules", payload: payload() });
    saveState = "saved";
    clearTimeout(savedTimer);
    savedTimer = setTimeout(() => (saveState = "idle"), 2000);
  }

  function scheduleSave() {
    clearTimeout(saveTimer);
    saveState = "pending";
    saveTimer = setTimeout(save, SAVE_DEBOUNCE);
  }

  onDestroy(() => {
    // Leaving the page must not lose an edit that waits for its timer.
    if (saveTimer !== undefined) save();
    clearTimeout(savedTimer);
    clearTimeout(testTimer);
  });

  // ---- Editing --------------------------------------------------------------

  function setPattern(i: number, j: number, value: string) {
    const match = [...rules[i].match];
    match[j] = value;
    rules[i] = { ...rules[i], match };
    scheduleSave();
  }

  function addPattern(i: number) {
    const j = rules[i].match.length;
    rules[i] = { ...rules[i], match: [...rules[i].match, ""] };
    focusTarget = { rule: i, pattern: j };
  }

  function removePattern(i: number, j: number) {
    const match = rules[i].match.filter((_, idx) => idx !== j);
    rules[i] = { ...rules[i], match: match.length > 0 ? match : [""] };
    save();
  }

  function addRule(rule: Rule = { match: [""], browser: "", profile: "" }) {
    const i = rules.length;
    rules = [...rules, rule];
    rowIsCustom = [...rowIsCustom, rule.browser !== "" && !installedBrowsers.includes(rule.browser)];
    rowProfileIsCustom = [...rowProfileIsCustom, false];
    focusTarget = { rule: i, pattern: 0 };
    if (rule.browser) requestProfiles([rule.browser]);
  }

  function removeRule(i: number) {
    const removed = rules[i];
    const custom = rowIsCustom[i];
    const profileCustom = rowProfileIsCustom[i];
    rules = rules.filter((_, idx) => idx !== i);
    rowIsCustom = rowIsCustom.filter((_, idx) => idx !== i);
    rowProfileIsCustom = rowProfileIsCustom.filter((_, idx) => idx !== i);
    save();
    toast.action(`Rule ${i + 1} deleted`, {
      label: "Undo",
      run: () => {
        rules = [...rules.slice(0, i), removed, ...rules.slice(i)];
        rowIsCustom = [...rowIsCustom.slice(0, i), custom, ...rowIsCustom.slice(i)];
        rowProfileIsCustom = [...rowProfileIsCustom.slice(0, i), profileCustom, ...rowProfileIsCustom.slice(i)];
        save();
      },
    });
  }

  function move(from: number, to: number) {
    if (to < 0 || to >= rules.length || from === to) return;
    const reorder = <T,>(list: T[]) => {
      const next = [...list];
      const [item] = next.splice(from, 1);
      next.splice(to, 0, item);
      return next;
    };
    rules = reorder(rules);
    rowIsCustom = reorder(rowIsCustom);
    rowProfileIsCustom = reorder(rowProfileIsCustom);
  }

  // Keyboard reorder on the grip: arrow keys move the rule, Home/End to the ends.
  async function onGripKey(e: KeyboardEvent, i: number) {
    const to =
      e.key === "ArrowUp" ? i - 1 :
      e.key === "ArrowDown" ? i + 1 :
      e.key === "Home" ? 0 :
      e.key === "End" ? rules.length - 1 : null;
    if (to === null || to < 0 || to >= rules.length || to === i) return;
    e.preventDefault();
    move(i, to);
    save();
    announcement = `Rule moved to position ${to + 1} of ${rules.length}.`;
    await tick();
    document.getElementById(`grip-${to}`)?.focus();
  }

  // Mouse drag
  let dragIndex = $state<number | null>(null);

  function onDragOver(e: DragEvent, i: number) {
    e.preventDefault();
    if (dragIndex === null || dragIndex === i) return;
    move(dragIndex, i);
    dragIndex = i;
  }

  function onDragEnd() {
    if (dragIndex !== null) save();
    dragIndex = null;
  }

  function autofocusNew(node: HTMLInputElement, coords: { rule: number; pattern: number }) {
    if (focusTarget && focusTarget.rule === coords.rule && focusTarget.pattern === coords.pattern) {
      tick().then(() => {
        node.focus();
        focusTarget = null;
      });
    }
  }

  // A pattern with no "*" must equal the whole URL, which rarely happens.
  function needsWildcard(pattern: string): boolean {
    const p = pattern.trim();
    return p.length > 0 && !p.includes("*");
  }

  function wildcardFix(pattern: string): string {
    const p = pattern.trim().replace(/^https?:\/\//, "").replace(/\/+$/, "");
    return `*${p}*`;
  }

  // ---- Test a URL -----------------------------------------------------------

  let testUrl = $state(untrack(() => $testUrlInput));
  let testing = $state(false);
  let testTimer: ReturnType<typeof setTimeout> | undefined;

  function normalizeUrl(url: string): string | null {
    const value = url.trim();
    if (!value) return null;
    const withScheme = /^[a-z][a-z0-9+.-]*:/i.test(value) ? value : `https://${value}`;
    try {
      new URL(withScheme);
      return withScheme;
    } catch {
      return null;
    }
  }

  $effect(() => {
    const url = normalizeUrl(testUrl);
    testUrlInput.set(testUrl);
    clearTimeout(testTimer);
    if (!url) {
      testUrlResult.set(null);
      testing = false;
      return;
    }
    testing = true;
    testTimer = setTimeout(() => window.finicky.sendMessage({ type: "testUrl", url }), 250);
  });

  $effect(() => {
    $testUrlResult;
    testing = false;
  });

  // Saved rules change the result: test again after each save.
  $effect(() => {
    rulesFile;
    const url = untrack(() => normalizeUrl(testUrl));
    if (url) window.finicky.sendMessage({ type: "testUrl", url });
  });

  const matchedRule = $derived(
    $testUrlResult && $testUrlResult.source === "rule" ? ($testUrlResult.ruleIndex ?? null) : null
  );

  function resultReason(): string {
    const r = $testUrlResult;
    if (!r) return "";
    switch (r.source) {
      case "rule":
        return `Rule ${(r.ruleIndex ?? 0) + 1} matches.`;
      case "js":
        return `Handler ${(r.handlerIndex ?? 0) + 1} of your JS config matches.`;
      case "default":
        return "No rule matches, so the default browser opens it.";
      default:
        return "";
    }
  }

  // ---- Start ----------------------------------------------------------------

  onMount(() => {
    window.finicky.sendMessage({ type: "getRules" });
    window.finicky.sendMessage({ type: "getInstalledBrowsers" });
    if (draft.rule) {
      const d = draft.rule;
      draft.rule = null;
      // Wait for the rules from disk, then add the draft once.
      setTimeout(() => {
        addRule({ match: [d.pattern], browser: d.browser, profile: d.profile ?? "" });
        save();
        announcement = `Rule ${rules.length} added for ${d.pattern}.`;
      }, 50);
    }
  });
</script>

<PageContainer title="Rules">
  {#snippet description()}
    Rules run from top to bottom. The first rule that matches opens the link. Use <code>*</code> as a wildcard.
  {/snippet}

  <div class="sr-only" role="status" aria-live="polite">{announcement}</div>

  <!-- Test -->
  <div class="test" class:has-result={!!$testUrlResult}>
    <label class="test-label" for="test-url">Test a link</label>
    <div class="test-field">
      <input
        id="test-url"
        class="text-input test-input"
        type="text"
        inputmode="url"
        placeholder="Paste a URL, for example github.com/settings"
        autocapitalize="off"
        autocomplete="off"
        spellcheck="false"
        bind:value={testUrl}
      />
      {#if testUrl}
        <button class="icon-btn" aria-label="Clear test URL" onclick={() => (testUrl = "")}><XIcon /></button>
      {/if}
    </div>
    <div class="test-result" aria-live="polite">
      {#if $testUrlResult?.error}
        <span class="result-error">{$testUrlResult.error}</span>
      {:else if $testUrlResult}
        <span class="result-main">
          Opens in <strong>{$testUrlResult.browser || "nothing"}</strong>{$testUrlResult.profile ? ` · ${$testUrlResult.profile}` : ""}{$testUrlResult.openInBackground ? ", in the background" : ""}.
        </span>
        <span class="result-reason">{resultReason()}</span>
        {#if $testUrlResult.url && normalizeUrl(testUrl) !== $testUrlResult.url}
          <span class="result-url">Final URL: <code>{$testUrlResult.url}</code></span>
        {/if}
      {:else if testUrl.trim() && !testing}
        <span class="result-reason">Type a full or partial URL, for example <code>zoom.us/j/123</code>.</span>
      {/if}
    </div>
  </div>

  {#if isJSConfig}
    <p class="note">
      Your JS config is active. Its handlers run first; these rules run only when no handler matches.
    </p>
  {/if}

  <!-- Rules -->
  {#if rules.length === 0}
    <div class="empty">
      <p class="empty-title">No rules yet</p>
      <p class="empty-text">A rule sends the links that match a pattern to one browser or profile.</p>
      <button class="primary" onclick={() => addRule()}>Add the first rule</button>
    </div>
  {:else}
    <ol class="rules" aria-label="Rules, in the order they run">
      {#each rules as rule, i}
        {@const missingBrowser = !rule.browser}
        <li
          class="rule"
          class:dragging={dragIndex === i}
          class:matched={matchedRule === i}
          ondragover={(e) => onDragOver(e, i)}
          ondrop={(e) => e.preventDefault()}
        >
          <button
            id="grip-{i}"
            class="grip"
            draggable="true"
            ondragstart={() => (dragIndex = i)}
            ondragend={onDragEnd}
            onkeydown={(e) => onGripKey(e, i)}
            aria-label="Rule {i + 1} of {rules.length}. Use the arrow keys to move it."
            title="Drag, or use the arrow keys, to change the order"
          >
            <span class="rule-no">{i + 1}</span>
            <span class="grip-dots" aria-hidden="true">⠿</span>
          </button>

          <div class="rule-body">
            <div class="rule-line">
              <span class="rule-word">When the URL matches</span>
              <div class="patterns">
                {#each rule.match as pattern, j}
                  {@const hintId = `hint-${i}-${j}`}
                  <div class="pattern">
                    <div class="pattern-row">
                      <input
                        class="text-input pattern-input"
                        type="text"
                        placeholder="*example.com/*"
                        autocapitalize="off"
                        autocomplete="off"
                        spellcheck="false"
                        aria-label="Rule {i + 1}, URL pattern {j + 1}"
                        aria-describedby={needsWildcard(pattern) ? hintId : undefined}
                        value={pattern}
                        oninput={(e) => setPattern(i, j, (e.target as HTMLInputElement).value)}
                        onblur={() => saveTimer !== undefined && save()}
                        use:autofocusNew={{ rule: i, pattern: j }}
                      />
                      {#if rule.match.length > 1}
                        <button class="icon-btn" onclick={() => removePattern(i, j)} aria-label="Remove pattern {j + 1} of rule {i + 1}"><XIcon /></button>
                      {/if}
                    </div>
                    {#if needsWildcard(pattern)}
                      <p class="hint" id={hintId}>
                        <WarningIcon />
                        Without <code>*</code> this matches only this exact URL.
                        <button class="link-btn" onclick={() => { setPattern(i, j, wildcardFix(pattern)); save(); }}>
                          Use <code>{wildcardFix(pattern)}</code>
                        </button>
                      </p>
                    {/if}
                  </div>
                {/each}
                <button class="link-btn add-pattern" onclick={() => addPattern(i)}>+ Another pattern</button>
              </div>
            </div>

            <div class="rule-line">
              <span class="rule-word">Open in</span>
              <div class="target">
                <BrowserProfileSelector
                  browser={rule.browser}
                  profile={rule.profile ?? ""}
                  isCustom={rowIsCustom[i]}
                  isProfileCustom={rowProfileIsCustom[i]}
                  {installedBrowsers}
                  {profilesByBrowser}
                  required
                  label="Browser for rule {i + 1}"
                  describedBy={missingBrowser ? `missing-${i}` : undefined}
                  browserPlaceholder="Choose a browser"
                  onBrowserChange={(browser, profile, isCustom) => {
                    rules[i] = { ...rules[i], browser, profile };
                    rowIsCustom[i] = isCustom;
                    rowProfileIsCustom[i] = false;
                  }}
                  onProfileChange={(profile, isProfileCustom) => {
                    rules[i] = { ...rules[i], profile };
                    rowProfileIsCustom[i] = isProfileCustom;
                  }}
                  onRequestProfiles={(b) => requestProfiles([b])}
                  onSave={save}
                  onInput={scheduleSave}
                />
                {#if missingBrowser}
                  <p class="hint" id="missing-{i}"><WarningIcon /> Choose a browser. Until then this rule does nothing.</p>
                {/if}
              </div>
            </div>
          </div>

          <button class="icon-btn delete" onclick={() => removeRule(i)} aria-label="Delete rule {i + 1}" title="Delete rule">
            <XIcon />
          </button>
        </li>
      {/each}
    </ol>
  {/if}

  <div class="list-actions">
    {#if rules.length > 0}
      <button class="secondary" onclick={() => addRule()}>+ Add rule</button>
    {/if}
    <span class="save-state" role="status">
      {#if saveState === "pending"}Saving…{:else if saveState === "saved"}Saved{/if}
    </span>
  </div>

  <!-- Default browser -->
  <div class="fallback" class:matched={$testUrlResult?.source === "default"}>
    <span class="rule-word">Otherwise open in</span>
    {#if isJSConfig}
      <span class="fallback-fixed">
        <strong>{config.defaultBrowser || "—"}</strong>
        <span class="fallback-note">Set by your JS config</span>
      </span>
    {:else}
      <div class="target">
        <BrowserProfileSelector
          browser={defaultBrowser}
          profile={defaultProfile}
          isCustom={defaultIsCustom}
          isProfileCustom={defaultProfileIsCustom}
          {installedBrowsers}
          {profilesByBrowser}
          label="Default browser"
          browserPlaceholder="Safari"
          onBrowserChange={(browser, profile) => {
            defaultBrowser = browser;
            defaultProfile = profile;
          }}
          onProfileChange={(profile) => {
            defaultProfile = profile;
          }}
          onRequestProfiles={(b) => requestProfiles([b])}
          onSave={save}
          onInput={scheduleSave}
        />
      </div>
    {/if}
  </div>
</PageContainer>

<style>
  code {
    font-family: var(--font-mono);
    font-size: 0.92em;
    background: var(--inset-bg);
    padding: 1px 4px;
    border-radius: 4px;
  }

  .text-input {
    width: 100%;
    box-sizing: border-box;
    padding: 7px 10px;
    background: var(--input-bg);
    border: 1px solid var(--border-color);
    border-radius: 8px;
    color: var(--text-primary);
    font-size: 0.88em;
    font-family: var(--font-mono);
  }

  .text-input:focus-visible {
    outline: 2px solid var(--focus-ring);
    outline-offset: 1px;
    border-color: var(--accent-color);
  }

  .text-input::placeholder {
    color: var(--text-secondary);
  }

  /* Test */
  .test {
    display: grid;
    gap: 6px;
    padding: 14px 16px;
    background: var(--card-bg);
    border: 1px solid var(--card-border);
    border-radius: 12px;
  }

  .test-label {
    font-size: 0.85em;
    font-weight: 600;
    color: var(--text-primary);
  }

  .test-field {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .test-input {
    font-size: 0.92em;
    padding: 9px 12px;
  }

  .test-result {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-height: 1.4em;
    font-size: 0.86em;
  }

  .result-main {
    color: var(--text-primary);
  }

  .result-main strong {
    color: var(--accent-color);
  }

  .result-reason,
  .result-url {
    color: var(--text-secondary);
  }

  .result-url code {
    word-break: break-all;
  }

  .result-error {
    color: var(--text-error);
    white-space: pre-wrap;
  }

  .note {
    margin: 0;
    padding: 10px 14px;
    border-radius: 10px;
    background: var(--inset-bg);
    color: var(--text-secondary);
    font-size: 0.85em;
  }

  /* Rules */
  .rules {
    list-style: none;
    margin: 0;
    padding: 0;
    border: 1px solid var(--card-border);
    border-radius: 12px;
    background: var(--card-bg);
    overflow: hidden;
  }

  .rule {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    gap: 12px;
    padding: 12px 10px 12px 8px;
    border-bottom: 1px solid var(--border-color);
    transition: background 0.15s ease, box-shadow 0.15s ease;
  }

  .rule:last-child {
    border-bottom: none;
  }

  .rule.matched,
  .fallback.matched {
    background: color-mix(in srgb, var(--accent-fill) 10%, transparent);
    box-shadow: inset 2px 0 0 var(--accent-fill);
  }

  .rule.dragging {
    opacity: 0.6;
    background: var(--bg-hover);
  }

  .grip {
    align-self: start;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    width: 30px;
    padding: 4px 0;
    background: none;
    border: none;
    border-radius: 6px;
    color: var(--text-secondary);
    cursor: grab;
    font: inherit;
  }

  .grip:hover {
    background: var(--bg-hover);
  }

  .rule-no {
    font-size: 0.78em;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
    color: var(--text-primary);
  }

  .grip-dots {
    font-size: 0.95em;
    line-height: 1;
  }

  .rule-body {
    display: grid;
    gap: 8px;
    min-width: 0;
  }

  .rule-line {
    display: grid;
    grid-template-columns: 9.5em minmax(0, 1fr);
    align-items: start;
    gap: 10px;
  }

  .rule-word {
    padding-top: 7px;
    color: var(--text-secondary);
    font-size: 0.84em;
    white-space: nowrap;
  }

  .patterns,
  .target {
    display: grid;
    gap: 6px;
    min-width: 0;
  }

  .pattern {
    display: grid;
    gap: 4px;
  }

  .pattern-row {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .hint {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px;
    margin: 0;
    color: var(--text-warning);
    font-size: 0.8em;
  }

  .hint :global(svg) {
    width: 12px;
    height: 12px;
    flex-shrink: 0;
  }

  .icon-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    width: 26px;
    height: 26px;
    padding: 0;
    background: none;
    border: none;
    border-radius: 6px;
    color: var(--text-secondary);
    cursor: pointer;
  }

  .icon-btn:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .icon-btn.delete {
    align-self: start;
  }

  .icon-btn.delete:hover {
    color: var(--text-error);
  }

  .link-btn {
    justify-self: start;
    padding: 2px 4px;
    margin-left: -4px;
    background: none;
    border: none;
    border-radius: 4px;
    color: var(--accent-color);
    font: inherit;
    font-size: inherit;
    cursor: pointer;
  }

  .link-btn:hover {
    text-decoration: underline;
  }

  .add-pattern {
    font-size: 0.8em;
  }

  .list-actions {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 32px;
  }

  .save-state {
    color: var(--text-secondary);
    font-size: 0.82em;
  }

  button.primary,
  button.secondary {
    min-height: 30px;
    padding: 5px 14px;
    border-radius: 8px;
    font: inherit;
    font-size: 0.86em;
    cursor: pointer;
  }

  button.primary {
    background: var(--accent-fill);
    border: 1px solid var(--accent-fill);
    color: var(--accent-fill-text);
    font-weight: 500;
  }

  button.secondary {
    background: var(--button-bg);
    border: 1px solid var(--border-color);
    color: var(--text-primary);
  }

  button.secondary:hover {
    background: var(--button-hover);
  }

  .empty {
    display: grid;
    justify-items: center;
    gap: 6px;
    padding: 24px;
    border: 1px dashed var(--border-color);
    border-radius: 12px;
    text-align: center;
  }

  .empty p {
    margin: 0;
  }

  .empty-title {
    color: var(--text-primary);
    font-weight: 600;
  }

  .empty-text {
    color: var(--text-secondary);
    font-size: 0.86em;
    margin-bottom: 6px !important;
  }

  /* Default browser */
  .fallback {
    display: grid;
    grid-template-columns: 9.5em minmax(0, 1fr);
    align-items: start;
    gap: 10px;
    /* Line up with the rule rows: grip column on the left, delete column on the right. */
    padding: 12px 48px 12px 50px;
    background: var(--card-bg);
    border: 1px solid var(--card-border);
    border-radius: 12px;
  }

  .fallback-fixed {
    display: flex;
    align-items: baseline;
    gap: 10px;
    padding-top: 6px;
    font-size: 0.9em;
  }

  .fallback-note {
    color: var(--text-secondary);
    font-size: 0.9em;
  }
</style>
