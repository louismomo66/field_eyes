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
	// Check if we're in a cloud environment
	isCloudEnv := os.Getenv("RENDER") == "true" || os.Getenv("CLOUD_ENV") == "true"

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
			app.InfoLog.Printf("Constructed DATABASE_URL from individual parameters")
		} else {
			app.ErrorLog.Println("⚠️ Neither DATABASE_URL nor individual DB_* variables are set, database features will not work ⚠️")

			// In cloud environments, log and continue instead of failing
			if isCloudEnv {
				app.InfoLog.Println("Running in cloud environment - continuing without database")
				return nil
			}
			return nil
		}
	}

	// Customize retry logic for cloud environments
	maxRetries := 5
	if isCloudEnv {
		maxRetries = 3 // Fewer retries on cloud platforms to avoid startup delays
	}
	retryDelay := 5 * time.Second

	// Connect to database with retries
	var db *sql.DB
	var err error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		app.InfoLog.Printf("Connecting to database (attempt %d/%d)...", attempt, maxRetries)

		// Try different hostnames if connection fails (for Docker/cloud environments)
		currentURL := dbURL
		if attempt > 1 && strings.Contains(dbURL, "localhost") && !isCloudEnv {
			// On retry, try with docker container name if using localhost
			currentURL = strings.Replace(dbURL, "localhost", "postgres", 1)
			app.InfoLog.Printf("Retrying with alternate hostname: postgres")
		} else if attempt > 2 && strings.Contains(dbURL, "localhost") && !isCloudEnv {
			// Try with host.docker.internal on third attempt
			currentURL = strings.Replace(dbURL, "localhost", "host.docker.internal", 1)
			app.InfoLog.Printf("Retrying with alternate hostname: host.docker.internal")
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

	// In cloud environments, log warning and continue without database
	if isCloudEnv {
		app.InfoLog.Println("Running in cloud environment - continuing without database")
	}

	return nil
}
