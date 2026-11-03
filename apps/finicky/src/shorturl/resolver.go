package shorturl

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

//go:embed shortener_domains.json
var shortenerDomainsFS embed.FS

// Common URL shortener domains
var shortenerDomains []string

func init() {
	// Load shortener domains from embedded JSON file
	data, err := shortenerDomainsFS.ReadFile("shortener_domains.json")
	if err != nil {
		slog.Error("Failed to read shortener domains file", "error", err)
		// Fallback to empty list if file cannot be read
		shortenerDomains = []string{}
		return
	}

	if err := json.Unmarshal(data, &shortenerDomains); err != nil {
		slog.Error("Failed to parse shortener domains JSON", "error", err)
		// Fallback to empty list if JSON is invalid
		shortenerDomains = []string{}
	}
}

// totalTimeout bounds the whole lookup (HEAD, optional GET, all redirects).
// The user waits for this before the browser opens.
const totalTimeout = 750 * time.Millisecond

// transport is shared so that repeated short links reuse connections.
var transport = http.DefaultTransport.(*http.Transport).Clone()

// isShortenerHost reports whether host is a known shortener domain or a
// subdomain of one. "notbit.ly" does not match "bit.ly".
func isShortenerHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	for _, domain := range shortenerDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

// ResolveURL resolves a potentially shortened URL to its final destination by following HTTP redirects, so
// the matcher can match the final URL instead of the short URL.
func ResolveURL(originalURL string) (string, error) {
	parsedURL, err := url.Parse(originalURL)
	if err != nil {
		return originalURL, fmt.Errorf("failed to parse URL: %v", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return originalURL, nil
	}

	if !isShortenerHost(parsedURL.Host) {
		return originalURL, nil
	}

	slog.Debug("URL host looks like a short URL", "host", parsedURL.Host)

	ctx, cancel := context.WithTimeout(context.Background(), totalTimeout)
	defer cancel()

	var lastUrl string

	// Helper to get the best available URL
	getReturnUrl := func() string {
		if lastUrl != "" && lastUrl != originalURL {
			slog.Debug("Falling back to last known URL from redirects", "url", lastUrl)
			return lastUrl
		}
		return originalURL
	}

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			lastUrl = req.URL.String()
			slog.Debug("Redirected to", "url", lastUrl)
			// Allow up to 3 redirects
			if len(via) >= 3 {
				return fmt.Errorf("stopped after 3 redirects")
			}
			return nil
		},
	}

	// HEAD first (no body). Some shorteners reject HEAD, so fall back to GET.
	for _, method := range []string{http.MethodHead, http.MethodGet} {
		req, err := http.NewRequestWithContext(ctx, method, originalURL, nil)
		if err != nil {
			return getReturnUrl(), fmt.Errorf("failed to create %s request: %v", method, err)
		}
		req.Header.Set("User-Agent", "Finicky/4.0")

		resp, err := client.Do(req)
		if err != nil {
			slog.Debug("Short URL request failed", "method", method, "url", originalURL, "error", err)
			if ctx.Err() != nil {
				break // the time budget is spent; do not try GET
			}
			continue
		}
		// Never read the body: only the final URL matters.
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			slog.Debug("Got a successful response", "url", resp.Request.URL.String())
			return resp.Request.URL.String(), nil
		}
	}

	return getReturnUrl(), fmt.Errorf("failed to resolve URL: no successful response")
}
