import { writable } from "svelte/store";
import type { MatchSource } from "../types";

export interface TestUrlResult {
  browser: string;
  url: string;
  openInBackground: boolean | null;
  profile?: string;
  source?: MatchSource;
  ruleIndex?: number | null;
  handlerIndex?: number | null;
  error?: string;
}

export const testUrlResult = writable<TestUrlResult | null>(null);
export const testUrlInput = writable<string>("");
