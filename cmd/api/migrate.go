//go:build ignore
// +build ignore

package main

import (
	"field_eyes/data"
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Set up logging
	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stdout, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	// Get database connection details from environment variables
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}

	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}

	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "postgres"
	}

	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "postgres"
	}

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "field_eyes"
	}

	// Construct the DSN string
	dsn := os.Getenv("DSN")
	if dsn == "" {
		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			dbHost, dbPort, dbUser, dbPassword, dbName)
	}

	// Connect to the database
	infoLog.Println("Connecting to database...")
	db, err := connectToDatabase(dsn, errorLog)
	if err != nil {
		errorLog.Fatalf("Could not connect to database: %v", err)
	}
	infoLog.Println("Connected to database!")

	// Create models
	models := data.New(db)

	// Migrate the database
	infoLog.Println("Running migrations...")
	if err := db.AutoMigrate(&models.User, &models.Device, &models.DeviceData); err != nil {
		errorLog.Fatalf("Migration failed: %v", err)
	}
	infoLog.Println("Migrations completed successfully!")
}

func connectToDatabase(dsn string, errorLog *log.Logger) (*gorm.DB, error) {
	// Try to connect to the database with retries
	var db *gorm.DB
	var err error
	maxRetries := 10

	for attempts := 1; attempts <= maxRetries; attempts++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			// Configure connection pool
			sqlDB, err := db.DB()
			if err != nil {
				return nil, err
			}

			sqlDB.SetMaxIdleConns(10)
			sqlDB.SetMaxOpenConns(100)
			sqlDB.SetConnMaxLifetime(time.Hour)

			// Test the connection
			err = sqlDB.Ping()
			if err == nil {
				return db, nil
			}
		}

		errorLog.Printf("Database connection attempt %d failed: %v. Retrying in 2 seconds...", attempts, err)
		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("failed to connect to database after %d attempts", maxRetries)
}
