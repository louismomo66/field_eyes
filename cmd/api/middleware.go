package main

import (
	"errors"
	"field_eyes/data"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
)

// getJWTSecret returns the JWT secret from environment variables
// with appropriate fallback and warning messages
func (app *Config) getJWTSecret() string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		// Check alternative spelling that might be used
		secret = os.Getenv("JWT_SECRETE")
		if secret == "" {
			app.ErrorLog.Println("WARNING: JWT_SECRET environment variable not set! Using default secret. This is insecure for production.")
			return "fieldeyes_default_jwt_secret_key"
		}
		app.InfoLog.Println("Using JWT_SECRETE instead of JWT_SECRET - consider standardizing to JWT_SECRET")
	}
	return secret
}

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

		mySigningKey := app.getJWTSecret()

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
				r.Header.Set("UserID", strconv.Itoa(int(userID)))
			}
		}

		// Call the next handler
		handler.ServeHTTP(w, r)
	}
}
func (app *Config) GenerateJWT(user data.User) (string, error) {
	mySigningKey := app.getJWTSecret()

	claims := jwt.MapClaims{
		"user_id": user.ID,
		"email":   user.Email,
		"role":    user.Role,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(mySigningKey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// GetUserInfoFromToken extracts user information from the JWT token in the Authorization header.
func (app *Config) GetUserInfoFromToken(r *http.Request) (uint, string, string, error) {
	// Get the Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return 0, "", "", errors.New("no token found")
	}

	// Extract the token from the "Bearer <token>" format
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		return 0, "", "", errors.New("invalid token format")
	}

	// Get the JWT secret
	mySigningKey := app.getJWTSecret()

	// Parse and validate the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Ensure the signing method is HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return []byte(mySigningKey), nil
	})
	if err != nil || !token.Valid {
		return 0, "", "", errors.New("invalid or expired token")
	}

	// Extract claims
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userID, _ := claims["user_id"].(float64)
		email, _ := claims["email"].(string)
		role, _ := claims["role"].(string)
		return uint(userID), email, role, nil
	}

	return 0, "", "", errors.New("invalid token claims")
}

// EnableCORS is a middleware to allow cross-origin requests
func (app *Config) EnableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers - allow all origins, methods, and headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "3600")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent) // 204 status is preferred for OPTIONS
			return
		}

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}
