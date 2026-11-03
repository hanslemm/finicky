// Minimal tab router. The window has five fixed pages, so a hash and a $state
// value replace a routing library. WKWebView loads the page from
// finicky-assets://local/, where history-based routing has nothing to resolve.

export type Tab = "overview" | "rules" | "settings" | "logs" | "about";

const TABS: readonly Tab[] = ["overview", "rules", "settings", "logs", "about"];

function fromHash(): Tab {
  const name = location.hash.replace(/^#/, "");
  return (TABS as readonly string[]).includes(name) ? (name as Tab) : "overview";
}

export const router = $state({ tab: fromHash() });

export function navigate(tab: Tab) {
  router.tab = tab;
  if (location.hash !== `#${tab}`) {
    location.hash = tab;
  }
}

window.addEventListener("hashchange", () => {
  router.tab = fromHash();
});
