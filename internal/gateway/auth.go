package gateway

import (
	"crypto/subtle"
	"os"

	"github.com/soyeahso/hunter3/internal/config"
)

// AuthResult is the outcome of an authentication attempt.
type AuthResult struct {
	OK     bool   `json:"ok"`
	Method string `json:"method,omitempty"` // "token" | "password"
	Reason string `json:"reason,omitempty"`
}

// ResolvedAuth holds the resolved auth configuration for the gateway.
type ResolvedAuth struct {
	Mode     string
	Token    string
	Password string
}

// ResolveAuth resolves authentication credentials from config and environment.
// Precedence: config value → env variable → empty.
func ResolveAuth(cfg config.GatewayAuth) ResolvedAuth {
	auth := ResolvedAuth{Mode: cfg.Mode}

	// Resolve token
	auth.Token = cfg.Token
	if auth.Token == "" {
		auth.Token = os.Getenv("HUNTER3_GATEWAY_TOKEN")
	}

	// Resolve password
	auth.Password = cfg.Password
	if auth.Password == "" {
		auth.Password = os.Getenv("HUNTER3_GATEWAY_PASSWORD")
	}

	// Default mode
	if auth.Mode == "" {
		if auth.Password != "" {
			auth.Mode = "password"
		} else {
			auth.Mode = "token"
		}
	}

	return auth
}

// Authorize checks the provided ConnectAuth against the resolved server auth.
func Authorize(serverAuth ResolvedAuth, clientAuth *ConnectAuth) AuthResult {
	if clientAuth == nil {
		return AuthResult{OK: false, Reason: "no credentials provided"}
	}

	switch serverAuth.Mode {
	case "token":
		if serverAuth.Token == "" {
			return AuthResult{OK: false, Reason: "server token not configured"}
		}
		if clientAuth.Token == "" {
			return AuthResult{OK: false, Reason: "token required"}
		}
		if !safeEqual(clientAuth.Token, serverAuth.Token) {
			return AuthResult{OK: false, Reason: "token_mismatch"}
		}
		return AuthResult{OK: true, Method: "token"}

	case "password":
		if serverAuth.Password == "" {
			return AuthResult{OK: false, Reason: "server password not configured"}
		}
		if clientAuth.Password == "" {
			return AuthResult{OK: false, Reason: "password required"}
		}
		if !safeEqual(clientAuth.Password, serverAuth.Password) {
			return AuthResult{OK: false, Reason: "password_mismatch"}
		}
		return AuthResult{OK: true, Method: "password"}

	default:
		return AuthResult{OK: false, Reason: "unknown auth mode: " + serverAuth.Mode}
	}
}

// safeEqual performs a constant-time string comparison to prevent timing attacks.
// It avoids early-return on length mismatch to prevent leaking secret length via timing.
func safeEqual(a, b string) bool {
	lenMatch := subtle.ConstantTimeEq(int32(len(a)), int32(len(b)))
	// ConstantTimeCompare returns 0 for different lengths, but we check
	// length separately with ConstantTimeEq to avoid leaking length info.
	cmp := subtle.ConstantTimeCompare([]byte(a), []byte(b))
	return subtle.ConstantTimeSelect(lenMatch, cmp, 0) == 1
}
