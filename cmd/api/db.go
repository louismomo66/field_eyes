package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func (app *Config) initDB() *sql.DB {
	// Check for development mode
	devMode := os.Getenv("DEV_MODE")
	if devMode == "true" {
		log.Println("⚠️ Running in DEVELOPMENT MODE without database ⚠️")
		return nil
	}

	// Get database URL from environment
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Println("⚠️ DATABASE_URL not set, database features will not work ⚠️")
		return nil
	}

	// Connect to the database
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Printf("⚠️ Failed to connect to database: %v ⚠️", err)
		return nil
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		log.Printf("⚠️ Failed to ping database: %v ⚠️", err)
		return nil
	}

	// Configure connection pool
	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(100)

	log.Println("Database connected successfully")
	return db
}
