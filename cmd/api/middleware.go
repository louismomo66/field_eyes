package main

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt"
)

func (app *Config) IsAuthenticated(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			app.errorJSON(w, errors.New("no token found"), http.StatusUnauthorized)
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			app.errorJSON(w, errors.New("invalid token format"), http.StatusUnauthorized)
			return
		}
		mySigningKey := os.Getenv("JWT_SECRET")
		if mySigningKey == "" {
			app.errorJSON(w, errors.New("internal server error: missing JWT secret"), http.StatusInternalServerError)
			return
		}
		// Parse and validate the token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Ensure the signing method is HMAC
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("invalid signing method")
			}
			return []byte(mySigningKey), nil
		})
		if err != nil || !token.Valid {
			app.errorJSON(w, errors.New("invalid or expired token"), http.StatusUnauthorized)
			return
		}

		// Extract claims and set user info in the request header
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			if role, ok := claims["role"].(string); ok {
				r.Header.Set("Role", role)
			}
			if userID, ok := claims["user_id"].(float64); ok {
				r.Header.Set("UserID", string(int(userID)))
			}
		}

		// Call the next handler
		handler.ServeHTTP(w, r)
	}
}
