package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/soyeahso/hunter3/internal/config"
	"github.com/stretchr/testify/assert"
)

// --- safeEqual tests ---

func TestSafeEqual_Match(t *testing.T) {
	assert.True(t, safeEqual("secret", "secret"))
}

func TestSafeEqual_Mismatch(t *testing.T) {
	assert.False(t, safeEqual("secret", "wrong"))
}

func TestSafeEqual_DifferentLengths(t *testing.T) {
	assert.False(t, safeEqual("short", "longer-string"))
}

func TestSafeEqual_BothEmpty(t *testing.T) {
	assert.True(t, safeEqual("", ""))
}

func TestSafeEqual_OneEmpty(t *testing.T) {
	assert.False(t, safeEqual("secret", ""))
	assert.False(t, safeEqual("", "secret"))
}

// --- ResolveAuth tests ---

func TestResolveAuth_TokenFromConfig(t *testing.T) {
	auth := ResolveAuth(config.GatewayAuth{
		Mode:  "token",
		Token: "config-token",
	})
	assert.Equal(t, "token", auth.Mode)
	assert.Equal(t, "config-token", auth.Token)
}

func TestResolveAuth_PasswordFromConfig(t *testing.T) {
	auth := ResolveAuth(config.GatewayAuth{
		Mode:     "password",
		Password: "config-pass",
	})
	assert.Equal(t, "password", auth.Mode)
	assert.Equal(t, "config-pass", auth.Password)
}

func TestResolveAuth_DefaultsToTokenMode(t *testing.T) {
	auth := ResolveAuth(config.GatewayAuth{
		Token: "my-token",
	})
	assert.Equal(t, "token", auth.Mode)
}

func TestResolveAuth_DefaultsToPasswordModeWhenPasswordSet(t *testing.T) {
	auth := ResolveAuth(config.GatewayAuth{
		Password: "my-pass",
	})
	assert.Equal(t, "password", auth.Mode)
}

func TestResolveAuth_TokenFromEnv(t *testing.T) {
	t.Setenv("HUNTER3_GATEWAY_TOKEN", "env-token")
	auth := ResolveAuth(config.GatewayAuth{Mode: "token"})
	assert.Equal(t, "env-token", auth.Token)
}

func TestResolveAuth_PasswordFromEnv(t *testing.T) {
	t.Setenv("HUNTER3_GATEWAY_PASSWORD", "env-pass")
	auth := ResolveAuth(config.GatewayAuth{Mode: "password"})
	assert.Equal(t, "env-pass", auth.Password)
}

func TestResolveAuth_ConfigOverridesEnv(t *testing.T) {
	t.Setenv("HUNTER3_GATEWAY_TOKEN", "env-token")
	auth := ResolveAuth(config.GatewayAuth{
		Mode:  "token",
		Token: "config-token",
	})
	assert.Equal(t, "config-token", auth.Token)
}

func TestResolveAuth_EmptyFallsToEnv(t *testing.T) {
	t.Setenv("HUNTER3_GATEWAY_TOKEN", "env-token")
	t.Setenv("HUNTER3_GATEWAY_PASSWORD", "env-pass")
	auth := ResolveAuth(config.GatewayAuth{Mode: "token"})
	assert.Equal(t, "env-token", auth.Token)
	assert.Equal(t, "env-pass", auth.Password)
}

// --- Authorize tests ---

func TestAuthorize_TokenSuccess(t *testing.T) {
	result := Authorize(
		ResolvedAuth{Mode: "token", Token: "secret"},
		&ConnectAuth{Token: "secret"},
	)
	assert.True(t, result.OK)
	assert.Equal(t, "token", result.Method)
	assert.Empty(t, result.Reason)
}

func TestAuthorize_TokenMismatch(t *testing.T) {
	result := Authorize(
		ResolvedAuth{Mode: "token", Token: "secret"},
		&ConnectAuth{Token: "wrong"},
	)
	assert.False(t, result.OK)
	assert.Equal(t, "token_mismatch", result.Reason)
}

func TestAuthorize_TokenEmpty(t *testing.T) {
	result := Authorize(
		ResolvedAuth{Mode: "token", Token: "secret"},
		&ConnectAuth{Token: ""},
	)
	assert.False(t, result.OK)
	assert.Equal(t, "token required", result.Reason)
}

func TestAuthorize_ServerTokenNotConfigured(t *testing.T) {
	result := Authorize(
		ResolvedAuth{Mode: "token", Token: ""},
		&ConnectAuth{Token: "client-token"},
	)
	assert.False(t, result.OK)
	assert.Contains(t, result.Reason, "server token not configured")
}

func TestAuthorize_PasswordSuccess(t *testing.T) {
	result := Authorize(
		ResolvedAuth{Mode: "password", Password: "pass123"},
		&ConnectAuth{Password: "pass123"},
	)
	assert.True(t, result.OK)
	assert.Equal(t, "password", result.Method)
}

func TestAuthorize_PasswordMismatch(t *testing.T) {
	result := Authorize(
		ResolvedAuth{Mode: "password", Password: "pass123"},
		&ConnectAuth{Password: "wrong"},
	)
	assert.False(t, result.OK)
	assert.Equal(t, "password_mismatch", result.Reason)
}

func TestAuthorize_PasswordEmpty(t *testing.T) {
	result := Authorize(
		ResolvedAuth{Mode: "password", Password: "pass123"},
		&ConnectAuth{Password: ""},
	)
	assert.False(t, result.OK)
	assert.Equal(t, "password required", result.Reason)
}

func TestAuthorize_ServerPasswordNotConfigured(t *testing.T) {
	result := Authorize(
		ResolvedAuth{Mode: "password", Password: ""},
		&ConnectAuth{Password: "client-pass"},
	)
	assert.False(t, result.OK)
	assert.Contains(t, result.Reason, "server password not configured")
}

func TestAuthorize_NilCredentials(t *testing.T) {
	result := Authorize(
		ResolvedAuth{Mode: "token", Token: "secret"},
		nil,
	)
	assert.False(t, result.OK)
	assert.Equal(t, "no credentials provided", result.Reason)
}

func TestAuthorize_UnknownAuthMode(t *testing.T) {
	result := Authorize(
		ResolvedAuth{Mode: "oauth"},
		&ConnectAuth{Token: "whatever"},
	)
	assert.False(t, result.OK)
	assert.Contains(t, result.Reason, "unknown auth mode")
}

// --- authRateLimiter tests ---

func TestAuthRateLimiter_AllowInitial(t *testing.T) {
	limiter := newAuthRateLimiter()
	assert.True(t, limiter.allow("192.168.1.1:12345"))
}

func TestAuthRateLimiter_AllowAfterFewFailures(t *testing.T) {
	limiter := newAuthRateLimiter()

	for i := 0; i < 5; i++ {
		limiter.recordFailure("192.168.1.1:12345")
	}
	assert.True(t, limiter.allow("192.168.1.1:12345"))
}

func TestAuthRateLimiter_BlockAfterMaxFailures(t *testing.T) {
	limiter := newAuthRateLimiter()

	for i := 0; i < authRateMaxFails; i++ {
		limiter.recordFailure("192.168.1.1:12345")
	}
	assert.False(t, limiter.allow("192.168.1.1:12345"))
}

func TestAuthRateLimiter_DifferentIPs(t *testing.T) {
	limiter := newAuthRateLimiter()

	for i := 0; i < authRateMaxFails; i++ {
		limiter.recordFailure("192.168.1.1:12345")
	}

	// Different IP should still be allowed
	assert.True(t, limiter.allow("192.168.1.2:12345"))
}

func TestAuthRateLimiter_IPWithoutPort(t *testing.T) {
	limiter := newAuthRateLimiter()

	for i := 0; i < authRateMaxFails; i++ {
		limiter.recordFailure("192.168.1.1")
	}
	assert.False(t, limiter.allow("192.168.1.1"))
}

func TestAuthRateLimiter_ExpiredFailures(t *testing.T) {
	limiter := newAuthRateLimiter()

	// Add old failures (before the window)
	limiter.mu.Lock()
	host := "192.168.1.1"
	oldTime := time.Now().Add(-authRateWindow - time.Minute)
	for i := 0; i < authRateMaxFails; i++ {
		limiter.failures[host] = append(limiter.failures[host], oldTime)
	}
	limiter.mu.Unlock()

	// Old failures should be cleaned up, so allow should return true
	assert.True(t, limiter.allow("192.168.1.1:12345"))
}

// --- checkWebSocketOrigin tests ---

func originRequest(origin string) *http.Request {
	req := httptest.NewRequest("GET", "/ws", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	return req
}

func TestCheckWebSocketOrigin_NoOriginHeader(t *testing.T) {
	check := checkWebSocketOrigin(nil)
	assert.True(t, check(originRequest("")))
}

func TestCheckWebSocketOrigin_EmptyAllowedList(t *testing.T) {
	check := checkWebSocketOrigin(nil)
	assert.False(t, check(originRequest("http://evil.com")))
}

func TestCheckWebSocketOrigin_Wildcard(t *testing.T) {
	check := checkWebSocketOrigin([]string{"*"})
	assert.True(t, check(originRequest("http://anything.com")))
}

func TestCheckWebSocketOrigin_SpecificMatch(t *testing.T) {
	check := checkWebSocketOrigin([]string{"http://allowed.com"})
	assert.True(t, check(originRequest("http://allowed.com")))
}

func TestCheckWebSocketOrigin_SpecificNoMatch(t *testing.T) {
	check := checkWebSocketOrigin([]string{"http://allowed.com"})
	assert.False(t, check(originRequest("http://evil.com")))
}

func TestCheckWebSocketOrigin_MultipleAllowed(t *testing.T) {
	check := checkWebSocketOrigin([]string{"http://one.com", "http://two.com"})
	assert.True(t, check(originRequest("http://one.com")))
	assert.True(t, check(originRequest("http://two.com")))
	assert.False(t, check(originRequest("http://three.com")))
}
