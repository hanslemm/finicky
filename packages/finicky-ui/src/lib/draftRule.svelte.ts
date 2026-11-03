// A rule that another page asks the Rules page to add (for example from a
// recent link). The Rules page takes it once and clears it.
export interface DraftRule {
  pattern: string;
  browser: string;
  profile?: string;
}

export const draft = $state<{ rule: DraftRule | null }>({ rule: null });

/** The wildcard pattern for all pages of a URL's host: "github.com/*". */
export function patternForUrl(url: string): string {
  try {
    const u = new URL(url);
    const host = u.hostname.replace(/^www\./, "");
    return `*${host}/*`;
  } catch {
    return url;
  }
}
