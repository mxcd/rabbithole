package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type APIKeyEntry struct {
	Key   string
	Label string
}

type APIKeyAuth struct {
	Keys map[string]string // key → label
}

func NewAPIKeyAuth(keysConfig string) *APIKeyAuth {
	auth := &APIKeyAuth{
		Keys: make(map[string]string),
	}
	if keysConfig == "" {
		return auth
	}
	for _, entry := range strings.Split(keysConfig, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, ":", 2)
		key := strings.TrimSpace(parts[0])
		label := key
		if len(parts) == 2 {
			label = strings.TrimSpace(parts[1])
		}
		auth.Keys[key] = label
		log.Info().Str("label", label).Msg("registered API key")
	}
	return auth
}

func (a *APIKeyAuth) Enabled() bool {
	return len(a.Keys) > 0
}

func (a *APIKeyAuth) Validate(key string) (string, bool) {
	label, ok := a.Keys[key]
	return label, ok
}

func (a *APIKeyAuth) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !a.Enabled() {
			c.Next()
			return
		}

		key := extractAPIKey(c.Request)
		if key == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key required"})
			return
		}

		label, ok := a.Validate(key)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
			return
		}

		c.Set("apiKeyLabel", label)
		c.Next()
	}
}

func extractAPIKey(r *http.Request) string {
	// Check Authorization header: Bearer <key>
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	// Check query parameter (for WebSocket connections)
	if key := r.URL.Query().Get("key"); key != "" {
		return key
	}

	return ""
}

func ExtractAPIKeyFromRequest(r *http.Request) string {
	return extractAPIKey(r)
}
