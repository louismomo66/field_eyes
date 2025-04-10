package main

import (
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

func (app *Config) initDB() *sql.DB {
	// Check for development mode
	devMode := os.Getenv("DEV_MODE")
	if devMode == "true" {
		log.Println("⚠️ Running in DEVELOPMENT MODE without database ⚠️")
		log.Println("⚠️ Database features will not work ⚠️")
		return nil
	}

	conn := connectToDB()
	if conn == nil {
		log.Println("⚠️ Failed to connect to database ⚠️")
		log.Println("⚠️ Continuing in limited mode - database features will not work ⚠️")
		return nil
	}

	// Test the connection
	if err := conn.Ping(); err != nil {
		log.Printf("⚠️ Failed to ping database: %v ⚠️", err)
		log.Println("⚠️ Continuing in limited mode - database features will not work ⚠️")
		return nil
	}

	log.Println("Database connected successfully")
	return conn
}

func connectToDB() *sql.DB {
	// First try to connect using DATABASE_URL (preferred for Docker and Render)
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		log.Printf("Found DATABASE_URL, attempting to connect")
		db, err := sql.Open("postgres", dbURL)
		if err == nil {
			// Test the connection
			err = db.Ping()
			if err == nil {
				log.Print("Connected to database using DATABASE_URL!")

				// Configure connection pool
				db.SetMaxIdleConns(10)
				db.SetMaxOpenConns(100)
				db.SetConnMaxLifetime(time.Hour)

				return db
			}
			log.Printf("Connection ping error: %v", err)
		}
		log.Printf("Connection error using DATABASE_URL: %v", err)
	}

	// // Fall back to DSN if DATABASE_URL not set or failed
	// dsn := os.Getenv("DSN")
	// if dsn == "" {
	// 	// Default DSN for Docker
	// 	dsn = "host=db port=5432 user=postgres password=postgres123456 dbname=field_eyes sslmode=disable"
	// }
	// log.Printf("Using DSN format")

	// // Try to connect with retries
	// var db *sql.DB
	// var err error

	// for attempts := 0; attempts < 10; attempts++ {
	// 	db, err = sql.Open("postgres", dsn)
	// 	if err == nil {
	// 		// Test the connection
	// 		err = db.Ping()
	// 		if err == nil {
	// 			log.Print("Connected to database using DSN!")

	// 			// Configure connection pool
	// 			db.SetMaxIdleConns(10)
	// 			db.SetMaxOpenConns(100)
	// 			db.SetConnMaxLifetime(time.Hour)

	// 			return db
	// 		}
	// 	}

	// 	log.Printf("Attempt %d: Database connection failed: %v", attempts+1, err)
	// 	time.Sleep(1 * time.Second)
	// }

	log.Println("Failed to connect to database after multiple attempts")
	return nil
}
