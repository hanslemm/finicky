// Compiled patterns, keyed by the wildcard pattern. A config has a fixed set
// of patterns, so this stays small; the cap only guards against configs that
// build patterns dynamically. `null` caches an invalid pattern.
const MAX_CACHED_PATTERNS = 1000;
const compiledPatterns = new Map<string, RegExp | null>();

export function matchWildcard(pattern: string, str: string): boolean {
  if (!pattern.includes("*")) {
    return pattern === str;
  }

  let regex = compiledPatterns.get(pattern);
  if (regex === undefined) {
    regex = compileWildcard(pattern);
    if (compiledPatterns.size >= MAX_CACHED_PATTERNS) {
      compiledPatterns.clear();
    }
    compiledPatterns.set(pattern, regex);
  }
  return regex !== null && regex.test(str);
}

function compileWildcard(pattern: string): RegExp | null {
  try {

    // First handle escaped asterisks by temporarily replacing them
    const ESCAPED_ASTERISK_PLACEHOLDER = "\u0000";
    const patternWithEscapedAsterisks = pattern.replace(
      /\\\*/g,
      ESCAPED_ASTERISK_PLACEHOLDER
    );

    // Then escape all special regex chars except asterisk
    let escaped = patternWithEscapedAsterisks.replace(
      /[.+?^${}()|[\]\\]/g,
      "\\$&"
    );

    // If pattern doesn't start with a protocol, make it match common protocols
    if (!/^\w+:/.test(pattern)) {
      // If pattern starts with *, don't add protocol matching
      if (!pattern.startsWith("*")) {
        escaped = `(?:https?:|ftp:|mailto:|file:|tel:|sms:|data:)?(?:\/\/)?${escaped}`;
      }
    } else {
      // If it's a protocol pattern, make sure to escape the forward slashes
      escaped = escaped.replace(/\//g, "\\/");
      // If it ends with //, make it match anything after
      if (escaped.endsWith("\\/\\/")) {
        escaped += ".*";
      }
    }

    // Replace unescaped asterisks with non-greedy match to prevent over-matching
    const regexPattern = escaped.replace(/\*/g, ".*?");

    // Finally, restore the escaped asterisks as literal asterisks
    const finalPattern = regexPattern.replace(
      new RegExp(ESCAPED_ASTERISK_PLACEHOLDER, "g"),
      "\\*"
    );

    // Add start and end anchors to ensure full string match
    return new RegExp(`^${finalPattern}$`);
  } catch (error) {
    console.warn("Invalid wildcard pattern:", pattern, error);
    return null;
  }
}
