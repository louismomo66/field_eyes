package main

import (
	"field_eyes/data"
	"field_eyes/pkg/email"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

const webPort = "9004"

func (app *Config) serve() {
	// Get the port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = webPort // Default to webPort constant
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: app.routes(),
	}

	app.InfoLog.Printf("Starting web server on port %s...", port)

	err := srv.ListenAndServe()
	if err != nil {
		if strings.Contains(err.Error(), "address already in use") {
			app.ErrorLog.Printf("Port %s is already in use. Try setting a different PORT in your environment variables.", port)
			os.Exit(1)
		} else {
			app.ErrorLog.Printf("Failed to start server: %v", err)
			os.Exit(1)
		}
	}
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

	//setup loggs
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

	// Initialize the mailer
	// In production, use SMTPMailer
	app.Mailer = email.NewSMTPMailer()

	// For development/testing, use MockMailer
	// app.Mailer = &email.MockMailer{}

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

	// Initialize data models - using simplified version
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

	go app.listenForErrors()
	app.serve()
}
