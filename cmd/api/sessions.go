package main

import (
	"encoding/gob"
	"field_eyes/data"
	"net/http"

	"github.com/gomodule/redigo/redis"
)

// SessionManager is a simplified session manager
type SessionManager struct {
	CookieName string
	MaxAge     int
	Secure     bool
	Pool       *redis.Pool
}

// InitSession initializes the session manager
func InitSession() *SessionManager {
	// Register types that will be stored in the session
	gob.Register(data.User{})

	// Get Redis configuration from environment
	return &SessionManager{
		CookieName: "field_eyes_session",
		MaxAge:     86400, // 24 hours
		Secure:     true,
	}
}

// LoadAndSave is a middleware that loads and saves session data for the current request
func (sm *SessionManager) LoadAndSave(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This is a simplified implementation
		// In a real implementation, we would:
		// 1. Get the session ID from the cookie
		// 2. Load the session data from Redis
		// 3. Call the next handler
		// 4. Save any changes back to Redis
		// 5. Set the cookie with the session ID

		// For now, just call the next handler
		next.ServeHTTP(w, r)
	})
}
