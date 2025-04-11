package main

import (
	"context"
	"field_eyes/data"
	"field_eyes/pkg/email"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

const webPort = "9004"

func (app *Config) serve() {
	// Get the port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = webPort // Default to webPort constant
	}

	// Check if we're in a cloud environment
	isCloudEnv := os.Getenv("RENDER") == "true" || os.Getenv("CLOUD_ENV") == "true"
	if isCloudEnv {
		app.InfoLog.Printf("Running in cloud environment on port %s", port)
	}

	// Create a new server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      app.routes(),
		IdleTimeout:  120 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start the server in a goroutine
	go func() {
		app.InfoLog.Printf("Starting web server on port %s (listening on all interfaces)...", port)

		err := srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			if strings.Contains(err.Error(), "address already in use") {
				app.ErrorLog.Printf("Port %s is already in use. Try setting a different PORT in your environment variables.", port)
			} else {
				app.ErrorLog.Printf("Failed to start server: %v", err)
			}
			app.ErrorChan <- err
		}
	}()

	// If on cloud environment (like Render), don't wait for OS signals
	if isCloudEnv {
		// For Render and similar platforms, we need to keep the main goroutine alive
		select {}
	}

	// Create a channel to listen for OS signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive a signal
	<-quit
	app.InfoLog.Println("Shutting down server...")

	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(ctx); err != nil {
		app.ErrorLog.Printf("Server forced to shutdown: %v", err)
	}

	app.InfoLog.Println("Server exited properly")
}

// loadEnvFile loads the environment variables from .env file
func loadEnvFile() bool {
	// Try loading from the app directory first (where the binary runs)
	err := godotenv.Load(".env")
	if err == nil {
		log.Println("Loaded .env file from current directory")
		return true
	}

	// Try loading from the project root directory
	err = godotenv.Load("../../.env")
	if err == nil {
		log.Println("Loaded .env file from project root directory")
		return true
	}

	// Try loading from absolute path if PWD is set
	if pwd := os.Getenv("PWD"); pwd != "" {
		// Try in current directory based on PWD
		err = godotenv.Load(filepath.Join(pwd, ".env"))
		if err == nil {
			log.Println("Loaded .env file from PWD directory")
			return true
		}

		// Try to go up one directory
		parentDir := filepath.Dir(pwd)
		err = godotenv.Load(filepath.Join(parentDir, ".env"))
		if err == nil {
			log.Println("Loaded .env file from parent directory")
			return true
		}
	}

	log.Println("Warning: No .env file found. Using environment variables.")
	return false
}

// Wait for background processes to finish with a timeout
func (app *Config) waitForBackgroundProcesses(timeout time.Duration) {
	// Create a channel for the wait group
	done := make(chan struct{})

	// Wait for the wait group in a goroutine
	go func() {
		app.Wait.Wait()
		close(done)
	}()

	// Wait with timeout
	select {
	case <-done:
		app.InfoLog.Println("All background processes completed successfully")
	case <-time.After(timeout):
		app.InfoLog.Println("Timed out waiting for background processes to complete")
	}
}

func main() {
	// Load environment variables from .env file
	envLoaded := loadEnvFile()

	// Check if JWT_SECRET is set
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecrete := os.Getenv("JWT_SECRETE") // Check alternate spelling
		if jwtSecrete == "" {
			log.Println("Warning: Neither JWT_SECRET nor JWT_SECRETE environment variable is set")
		} else {
			log.Println("JWT_SECRETE environment variable loaded successfully (consider standardizing to JWT_SECRET)")
		}
	} else {
		log.Println("JWT_SECRET environment variable loaded successfully")
	}

	//setup logs
	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stdout, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)
	app := Config{
		InfoLog:       infoLog,
		ErrorLog:      errorLog,
		Wait:          &sync.WaitGroup{},
		ErrorChan:     make(chan error),
		ErrorChanDone: make(chan bool),
	}

	// Additional debugging info about the environment
	if envLoaded {
		app.InfoLog.Println("Environment variables loaded from .env file")
	} else {
		app.InfoLog.Println("Using system environment variables (no .env file loaded)")
	}

	// Print key configuration values
	app.InfoLog.Printf("Using PORT=%s", os.Getenv("PORT"))
	app.InfoLog.Printf("Using DB_HOST=%s", os.Getenv("DB_HOST"))
	app.InfoLog.Printf("Using REDIS_HOST=%s", os.Getenv("REDIS_HOST"))

	// Initialize the mailer
	// In production, use SMTPMailer
	app.Mailer = email.NewSMTPMailer()

	// Initialize Redis client
	redisClient, err := NewRedisClient()
	if err != nil {
		app.ErrorLog.Printf("Warning: Failed to connect to Redis: %v", err)
		app.ErrorLog.Println("Continuing without Redis caching...")
	} else {
		app.Redis = redisClient
		app.InfoLog.Println("Connected to Redis successfully")

		// Defer closing the Redis connection
		defer func() {
			if err := redisClient.Close(); err != nil {
				app.ErrorLog.Printf("Error closing Redis connection: %v", err)
			}
		}()
	}

	// connect to the database
	db := app.initDB()
	app.DB = db

	// Close database connection when app exits
	if db != nil {
		defer func() {
			app.InfoLog.Println("Closing database connection...")
			if err := db.Close(); err != nil {
				app.ErrorLog.Printf("Error closing database connection: %v", err)
			}
		}()
	}

	// Initialize data models
	app.Models = data.New(db)

	// Initialize MQTT client
	mqttClient, err := NewMQTTClient(&app)
	if err != nil {
		app.ErrorLog.Printf("Warning: Failed to connect to MQTT broker: %v", err)
		app.ErrorLog.Println("Continuing without MQTT functionality...")
	} else {
		app.MQTT = mqttClient
		app.InfoLog.Println("Connected to MQTT broker successfully")

		// Start the MQTT device data listener
		if err := mqttClient.StartDeviceDataListener(); err != nil {
			app.ErrorLog.Printf("Failed to start MQTT device data listener: %v", err)
		} else {
			app.InfoLog.Println("MQTT device data listener started successfully")
		}

		// Defer closing the MQTT connection
		defer mqttClient.CloseConnection()
	}

	// Start error listener
	go app.listenForErrors()

	// Start the server
	app.serve()

	// Wait for background processes to complete with a timeout
	app.waitForBackgroundProcesses(10 * time.Second)

	// Signal the error listener to exit
	app.ErrorChanDone <- true

	app.InfoLog.Println("Application shutdown complete")
}
