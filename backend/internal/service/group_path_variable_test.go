//go:build unit

package service

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestIsValidGroupPathVariable(t *testing.T) {
	t.Parallel()

	valid := []string{"foo", "foo-bar_baz.1~2", "ABC123", "a", strings.Repeat("a", 128)}
	for _, s := range valid {
		require.Truef(t, IsValidGroupPathVariable(s), "expected valid: %q", s)
	}

	invalid := []string{
		"", " ", ".", "..", "foo/bar", "a b", "foo%2f", "foo?x=1",
		"a@b", "../x", "foo#frag", "foo:bar", strings.Repeat("a", 129),
	}
	for _, s := range invalid {
		require.Falsef(t, IsValidGroupPathVariable(s), "expected invalid: %q", s)
	}
}

func TestNormalizeGroupPathVariable(t *testing.T) {
	t.Parallel()

	require.Equal(t, "myseg", normalizeGroupPathVariable("myseg"))
	require.Equal(t, "myseg", normalizeGroupPathVariable("  myseg  "))
	require.Equal(t, "myseg", normalizeGroupPathVariable("/myseg/"))
	// empty / invalid collapse to "" (disabled)
	require.Equal(t, "", normalizeGroupPathVariable(""))
	require.Equal(t, "", normalizeGroupPathVariable("bad/seg"))
	require.Equal(t, "", normalizeGroupPathVariable(".."))
	require.Equal(t, "", normalizeGroupPathVariable("a b"))
}

func TestGroupUpstreamPathSegment(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	newCtx := func(set bool, val string) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		if set {
			c.Set(CtxKeyGroupPathVar, val)
		}
		return c
	}

	// configured + valid, base_url has no matching tail → spliced with leading slash
	require.Equal(t, "/myseg", groupUpstreamPathSegment(newCtx(true, "myseg"), "https://host"))
	require.Equal(t, "/myseg", groupUpstreamPathSegment(newCtx(true, "myseg"), "http://host:8080/other"))
	// not configured (no context key) → empty
	require.Equal(t, "", groupUpstreamPathSegment(newCtx(false, ""), "https://host"))
	// configured but invalid stored value → dropped (defense in depth)
	require.Equal(t, "", groupUpstreamPathSegment(newCtx(true, "bad/seg"), "https://host"))
	require.Equal(t, "", groupUpstreamPathSegment(newCtx(true, ".."), "https://host"))
	// dedup: base_url already ends with the segment → not appended again
	require.Equal(t, "", groupUpstreamPathSegment(newCtx(true, "newcache1m"), "http://23.94.169.215:62311/newcache1m"))
	require.Equal(t, "", groupUpstreamPathSegment(newCtx(true, "newcache1m"), "http://host/a/newcache1m"))
	// partial/different tail must still append (no false dedup)
	require.Equal(t, "/newcache1m", groupUpstreamPathSegment(newCtx(true, "newcache1m"), "http://host/newcache1m2"))
	// nil context
	require.Equal(t, "", groupUpstreamPathSegment(nil, "https://host"))
}

func TestBaseURLLastSegment(t *testing.T) {
	t.Parallel()
	require.Equal(t, "newcache1m", baseURLLastSegment("http://23.94.169.215:62311/newcache1m"))
	require.Equal(t, "newcache1m", baseURLLastSegment("http://host/a/b/newcache1m/"))
	require.Equal(t, "b", baseURLLastSegment("https://host/a/b"))
	require.Equal(t, "", baseURLLastSegment("https://host"))
	require.Equal(t, "", baseURLLastSegment("https://host/"))
	require.Equal(t, "", baseURLLastSegment(""))
}
