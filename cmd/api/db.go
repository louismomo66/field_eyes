package main

import (
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func (app *Config) initDB() *gorm.DB {
	conn := connectToDB()
	if conn == nil {
		log.Panic("can't connect to database")
	}

	// Auto-migrate the schema
	if err := conn.AutoMigrate(&app.Models.User, &app.Models.Device); err != nil {
		log.Panic("failed to migrate database:", err)
	}
	log.Println("Database migration completed successfully")

	return conn
}
func connectToDB() *gorm.DB {
	counts := 0
	dsn := os.Getenv("DSN")

	for {
		connection, err := openDB(dsn)
		if err != nil {
			log.Println("postgres not yet ready...")
		} else {
			log.Print("connected to database!")
			return connection
		}

		if counts > 10 {
			return nil
		}

		log.Print("Backing off for 1 second")
		time.Sleep(1 * time.Second)
		counts++
	}
}

func openDB(dsn string) (*gorm.DB, error) {
	config := &gorm.Config{
		// You can add GORM configurations here
		// For example:
		// Logger: logger.Default.LogMode(logger.Info),
		// PrepareStmt: true,
	}

	db, err := gorm.Open(postgres.Open(dsn), config)
	if err != nil {
		return nil, err
	}

	// Get the underlying *sql.DB instance
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Test the connection
	err = sqlDB.Ping()
	if err != nil {
		return nil, err
	}

	return db, nil
}
