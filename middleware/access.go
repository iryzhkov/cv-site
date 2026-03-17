package middleware

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
)

type contextKey string

const (
	CtxCompany   contextKey = "company"
	CtxIsLocal   contextKey = "is_local"
	CtxHasLive   contextKey = "has_live"
)

// Token maps a company token to a company name.
type Token struct {
	Token   string `json:"token"`
	Company string `json:"company"`
	Active  bool   `json:"active"`
}

var tokenStore = struct {
	sync.RWMutex
	tokens []Token
}{tokens: []Token{}}

// LoadTokens reads tokens from data/tokens.json.
func LoadTokens() {
	data, err := os.ReadFile("data/tokens.json")
	if err != nil {
		return
	}
	tokenStore.Lock()
	defer tokenStore.Unlock()
	json.Unmarshal(data, &tokenStore.tokens)
}

// SaveTokens writes tokens to data/tokens.json.
func SaveTokens() error {
	tokenStore.RLock()
	defer tokenStore.RUnlock()
	data, err := json.MarshalIndent(tokenStore.tokens, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("data/tokens.json", data, 0644)
}

// GetTokens returns a copy of all tokens.
func GetTokens() []Token {
	tokenStore.RLock()
	defer tokenStore.RUnlock()
	cp := make([]Token, len(tokenStore.tokens))
	copy(cp, tokenStore.tokens)
	return cp
}

// AddToken adds a new token.
func AddToken(token, company string) {
	tokenStore.Lock()
	defer tokenStore.Unlock()
	tokenStore.tokens = append(tokenStore.tokens, Token{Token: token, Company: company, Active: true})
}

// RevokeToken deactivates a token.
func RevokeToken(token string) {
	tokenStore.Lock()
	defer tokenStore.Unlock()
	for i := range tokenStore.tokens {
		if tokenStore.tokens[i].Token == token {
			tokenStore.tokens[i].Active = false
		}
	}
}

// DeleteToken removes a token entirely.
func DeleteToken(token string) {
	tokenStore.Lock()
	defer tokenStore.Unlock()
	filtered := tokenStore.tokens[:0]
	for _, t := range tokenStore.tokens {
		if t.Token != token {
			filtered = append(filtered, t)
		}
	}
	tokenStore.tokens = filtered
}

// lookupToken finds a company for the given token.
func lookupToken(t string) (string, bool) {
	tokenStore.RLock()
	defer tokenStore.RUnlock()
	for _, tok := range tokenStore.tokens {
		if tok.Token == t && tok.Active {
			return tok.Company, true
		}
	}
	return "", false
}

// localNetworks defines the IP ranges considered "local".
var localNetworks = []string{
	"192.168.", "10.", "172.16.", "172.17.", "172.18.", "172.19.",
	"172.20.", "172.21.", "172.22.", "172.23.", "172.24.", "172.25.",
	"172.26.", "172.27.", "172.28.", "172.29.", "172.30.", "172.31.",
	"127.", "::1", "fd", "fe80:",
}

func isLocalIP(ip string) bool {
	for _, prefix := range localNetworks {
		if strings.HasPrefix(ip, prefix) {
			return true
		}
	}
	return false
}

func getClientIP(r *http.Request) string {
	// Cloudflare sets the real client IP in CF-Connecting-IP
	// This is the most reliable source when behind Cloudflare Tunnel
	if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
		return cfIP
	}
	// Fallback to X-Forwarded-For (first entry is the original client)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

// AccessControl is middleware that:
// - Detects local vs external access
// - Checks for company tokens
// - Sets context values for downstream handlers
func AccessControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := getClientIP(r)
		isLocal := isLocalIP(clientIP)

		// Check for company token
		var company string
		hasLive := isLocal // Local always gets live access

		// Check URL param first, then cookie
		t := r.URL.Query().Get("t")
		if t == "" {
			if c, err := r.Cookie("cv_token"); err == nil {
				t = c.Value
			}
		}

		tokenFromURL := r.URL.Query().Get("t") != ""

		if t != "" {
			if c, ok := lookupToken(t); ok {
				company = c
				hasLive = true
				// Set cookie so URL can be bookmarked
				http.SetCookie(w, &http.Cookie{
					Name:     "cv_token",
					Value:    t,
					Path:     "/",
					MaxAge:   86400 * 30,
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
				})
				// Redirect to clean URL (remove ?t= param)
				if tokenFromURL {
					cleanURL := *r.URL
					q := cleanURL.Query()
					q.Del("t")
					cleanURL.RawQuery = q.Encode()
					http.Redirect(w, r, cleanURL.String(), http.StatusFound)
					return
				}
			}
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, CtxCompany, company)
		ctx = context.WithValue(ctx, CtxIsLocal, isLocal)
		ctx = context.WithValue(ctx, CtxHasLive, hasLive)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// IsLocal returns whether the request is from the local network.
func IsLocal(r *http.Request) bool {
	v, _ := r.Context().Value(CtxIsLocal).(bool)
	return v
}

// HasLiveAccess returns whether the request has live LLM access.
func HasLiveAccess(r *http.Request) bool {
	v, _ := r.Context().Value(CtxHasLive).(bool)
	return v
}

// GetCompany returns the company name from the request context.
func GetCompany(r *http.Request) string {
	v, _ := r.Context().Value(CtxCompany).(string)
	return v
}

// NotFoundHandler can be set by the main package to use the custom 404 page.
var NotFoundHandler http.HandlerFunc

// LocalOnly wraps a handler to only allow local network access.
func LocalOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsLocal(r) {
			if NotFoundHandler != nil {
				NotFoundHandler(w, r)
			} else {
				http.NotFound(w, r)
			}
			return
		}
		next(w, r)
	}
}
