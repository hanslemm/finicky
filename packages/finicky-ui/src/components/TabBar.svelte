<script lang="ts">
  import type { Component } from "svelte";
  import PreferencesIcon from "./icons/Preferences.svelte";
  import OverviewIcon from "./icons/Overview.svelte";
  import LogsIcon from "./icons/Logs.svelte";
  import AboutIcon from "./icons/About.svelte";
  import RulesIcon from "./icons/Rules.svelte";
  import { router, type Tab } from "../lib/router.svelte";

  let { numErrors = 0, needsAttention = false }: { numErrors?: number; needsAttention?: boolean } = $props();

  type TabDef = { tab: Tab; label: string; icon: Component; showErrors?: boolean; attention?: boolean };

  const tabs: TabDef[] = $derived([
    { tab: "overview", label: "Overview", icon: OverviewIcon, attention: needsAttention },
    { tab: "rules", label: "Rules", icon: RulesIcon },
    { tab: "settings", label: "Settings", icon: PreferencesIcon },
  ]);

  const bottomTabs: TabDef[] = [
    { tab: "logs", label: "Logs", icon: LogsIcon, showErrors: true },
    { tab: "about", label: "About", icon: AboutIcon },
  ];
</script>

{#snippet tabLink(t: TabDef, small: boolean)}
  {@const active = router.tab === t.tab}
  <a
    href="#{t.tab}"
    class="tab-content"
    class:active
    class:tab-small={small}
    aria-current={active ? "page" : undefined}
  >
    <span class="icon-container">
      <t.icon />
      {#if t.showErrors && numErrors > 0}
        <span class="error-badge" aria-hidden="true">{numErrors}</span>
      {/if}
    </span>
    <span>{t.label}</span>
    {#if t.attention}
      <span class="attention-dot" aria-hidden="true"></span>
      <span class="sr-only">, needs attention</span>
    {/if}
    {#if t.showErrors && numErrors > 0}
      <span class="sr-only">, {numErrors} {numErrors === 1 ? "error" : "errors"}</span>
    {/if}
  </a>
{/snippet}

<div class="tab-container">
  <nav class="tab-bar" aria-label="Sections">
    {#each tabs as t}
      {@render tabLink(t, false)}
    {/each}
    <div class="spacer"></div>
    <div class="bottom-tabs">
      {#each bottomTabs as t}
        {@render tabLink(t, true)}
      {/each}
    </div>
  </nav>
</div>

<style>
  .tab-container {
    width: 210px;
    background: var(--bg-nav);
    position: sticky;
    top: 0;
    z-index: 10;
    padding: 0.75rem 0.625rem;
    display: flex;
    flex: 0 0 auto;
    flex-direction: column;
    align-items: stretch;
    justify-content: flex-start;
    border-right: 1px solid var(--border-color);
    height: 100%;
    box-sizing: border-box;
  }

  .tab-bar {
    display: flex;
    flex-direction: column;
    gap: 2px;
    flex: 1;
    background: transparent;
    padding: 0;
    margin: 0;
  }

  .spacer {
    flex: 1;
  }

  .bottom-tabs {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding-top: 6px;
    border-top: 1px solid var(--border-color);
  }

  .tab-content {
    user-select: none;
    padding: 0.6rem 0.875rem;
    border-radius: 0.5rem;
    display: flex;
    align-items: center;
    flex-direction: row;
    gap: 10px;
    color: var(--nav-text);
    background: transparent;
    transition: background 0.1s ease;
    width: 100%;
    text-align: left;
    font-size: 0.925em;
    font-weight: 400;
    text-decoration: none;
    box-sizing: border-box;

    &:hover {
      background: var(--nav-hover);
    }

    &.active {
      background: var(--nav-active);
      color: var(--nav-active-text);
      font-weight: 500;
    }

    &.tab-small {
      font-size: 0.78em;
      padding: 0.35rem 0.75rem;
      color: var(--nav-text-secondary);

      &.active {
        color: var(--nav-active-text);
      }
    }
  }

  .icon-container {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: flex-start;
  }

  .attention-dot {
    width: 7px;
    height: 7px;
    margin-left: auto;
    border-radius: 50%;
    background: var(--log-warning);
  }

  .error-badge {
    position: absolute;
    top: -8px;
    right: -8px;
    background: var(--log-error);
    color: #fff;
    border-radius: 9px;
    min-width: 18px;
    height: 18px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 12px;
    font-weight: bold;
    padding: 0 4px;
    border: 2px solid var(--bg-nav);
  }
</style>
