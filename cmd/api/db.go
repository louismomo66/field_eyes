package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

func (app *Config) initDB() *sql.DB {
	// Check for development mode
	devMode := os.Getenv("DEV_MODE")
	if devMode == "true" {
		app.InfoLog.Println("⚠️ Running in DEVELOPMENT MODE without database ⚠️")
		return nil
	}

	// Get database URL from environment
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// Try to construct from individual variables if DATABASE_URL is not set
		dbHost := os.Getenv("DB_HOST")
		dbPort := os.Getenv("DB_PORT")
		dbUser := os.Getenv("DB_USER")
		dbPass := os.Getenv("DB_PASSWORD")
		dbName := os.Getenv("DB_NAME")

		if dbHost != "" && dbUser != "" && dbName != "" {
			if dbPort == "" {
				dbPort = "5432" // Default PostgreSQL port
			}
			dbURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
				dbUser, dbPass, dbHost, dbPort, dbName)
			app.InfoLog.Printf("Constructed DATABASE_URL from individual parameters using host %s", dbHost)
		} else {
			app.ErrorLog.Println("⚠️ Neither DATABASE_URL nor individual DB_* variables are set, database features will not work ⚠️")
			return nil
		}
	}

	// When running in Docker, we should prefer service names over localhost
	if os.Getenv("DOCKER_ENV") == "true" && strings.Contains(dbURL, "localhost") {
		dbURL = strings.Replace(dbURL, "localhost", "postgres", 1)
		app.InfoLog.Printf("Docker environment detected: Using postgres service instead of localhost")
	}

	// Connect to database with retries
	var db *sql.DB
	var err error
	maxRetries := 5
	retryDelay := 5 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		app.InfoLog.Printf("Connecting to database (attempt %d/%d) at %s...", attempt, maxRetries, anonymizeConnectionString(dbURL))

		// Try different hostnames if connection fails (for Docker environments)
		currentURL := dbURL
		if attempt > 1 && strings.Contains(dbURL, "localhost") {
			// On retry, try with docker container name if using localhost
			currentURL = strings.Replace(dbURL, "localhost", "postgres", 1)
			app.InfoLog.Printf("Retrying with postgres service: %s", anonymizeConnectionString(currentURL))
		}

		db, err = sql.Open("postgres", currentURL)
		if err == nil {
			// Test the connection
			err = db.Ping()
			if err == nil {
				// Configure connection pool
				db.SetMaxIdleConns(10)
				db.SetMaxOpenConns(100)
				db.SetConnMaxLifetime(time.Hour)

				app.InfoLog.Println("✅ Database connected successfully")
				return db
			}
			app.ErrorLog.Printf("Database ping failed: %v", err)
		} else {
			app.ErrorLog.Printf("Database connection failed: %v", err)
		}

		// Log the error and retry after delay
		app.InfoLog.Printf("Retrying in %v...", retryDelay)
		time.Sleep(retryDelay)
	}

	app.ErrorLog.Printf("⚠️ Failed to connect to database after %d attempts ⚠️", maxRetries)
	return nil
}

// anonymizeConnectionString returns a connection string with password replaced by asterisks
func anonymizeConnectionString(connStr string) string {
	// Find the password part and replace it with asterisks
	parts := strings.Split(connStr, "@")
	if len(parts) != 2 {
		return connStr
	}

	credParts := strings.Split(parts[0], ":")
	if len(credParts) < 3 {
		return connStr
	}

	// Replace the password with asterisks
	credParts[2] = "********"
	parts[0] = strings.Join(credParts, ":")

	return strings.Join(parts, "@")
}
