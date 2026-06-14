package service

import (
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// Fork feature: Anthropic group-level upstream path variable.
//
// An Anthropic group may carry a static path segment (Group.PathVariable). When
// set, requests served by API-key accounts in that group are forwarded to
// https://<base_url>/<seg>/v1/messages; when empty, the upstream URL is unchanged
// (https://<base_url>/v1/messages). The gateway handler copies the resolved
// segment into the gin context so the upstream URL builders can splice it in.

// CtxKeyGroupPathVar is the gin context key under which the gateway handler stores
// the request group's path variable for consumption by the upstream URL builders.
const CtxKeyGroupPathVar = "group_path_var"

const maxGroupPathVarLen = 128

// IsValidGroupPathVariable reports whether a configured group path variable is a
// single, safe URL path segment. Only a conservative charset is allowed — no
// slashes, dot-dot, percent-encoding or userinfo — so the value cannot perform
// path traversal / SSRF when reflected into the upstream URL.
func IsValidGroupPathVariable(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > maxGroupPathVarLen {
		return false
	}
	if s == "." || s == ".." {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-', r == '_', r == '.', r == '~':
		default:
			return false
		}
	}
	return true
}

// normalizeGroupPathVariable cleans an admin-supplied path variable for storage:
// surrounding whitespace and slashes are trimmed, and anything that is empty or
// fails validation collapses to "" (treated as "no path variable" / disabled).
func normalizeGroupPathVariable(s string) string {
	s = strings.Trim(strings.TrimSpace(s), "/")
	if s == "" || !IsValidGroupPathVariable(s) {
		return ""
	}
	return s
}

// groupUpstreamPathSegment returns the validated leading path segment to splice
// into the upstream URL (e.g. "/myseg"), or "" when the group has no path variable
// configured, the stored value fails validation (defense in depth), or the account
// base_url already ends with that segment (dedup: avoid .../seg/seg/v1/messages).
// The value is stashed in the gin context by the gateway handler from
// apiKey.Group.PathVariable; validatedBaseURL is the account's validated base_url.
func groupUpstreamPathSegment(c *gin.Context, validatedBaseURL string) string {
	if c == nil {
		return ""
	}
	raw, ok := c.Get(CtxKeyGroupPathVar)
	if !ok {
		return ""
	}
	seg, _ := raw.(string)
	seg = strings.Trim(strings.TrimSpace(seg), "/")
	if seg == "" || !IsValidGroupPathVariable(seg) {
		return ""
	}
	// Dedup: if base_url already ends with this exact path segment, don't append
	// it again (e.g. base_url ".../newcache1m" + seg "newcache1m").
	if baseURLLastSegment(validatedBaseURL) == seg {
		return ""
	}
	return "/" + seg
}

// baseURLLastSegment returns the last non-empty path segment of a base URL
// (e.g. "http://host:port/a/b" -> "b"), or "" when the URL carries no path.
func baseURLLastSegment(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	path := ""
	if u, err := url.Parse(rawURL); err == nil && u.Host != "" {
		path = u.Path
	} else if idx := strings.Index(rawURL, "://"); idx >= 0 {
		if slash := strings.IndexByte(rawURL[idx+3:], '/'); slash >= 0 {
			path = rawURL[idx+3+slash:]
		}
	}
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[i+1:]
	}
	return path
}
