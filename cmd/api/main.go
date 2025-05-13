package main

import (
	"errors"
	"field_eyes/data"
	"field_eyes/pkg/email"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

const webPort = "9002"

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

	// Initialize Session Manager
	app.Sessions = InitSession()
	app.InfoLog.Println("Session manager initialized")

	// connect to the database
	db := app.initDB()
	app.DB = db

	// Initialize data models
	app.Models = data.New(db)

	// Ensure system user exists for device auto-registration
	if err := app.ensureSystemUserExists(); err != nil {
		app.ErrorLog.Printf("Warning: Failed to create system user: %v", err)
		app.ErrorLog.Println("Auto-registration of devices may not work properly")
	} else {
		app.InfoLog.Println("System user verified")
	}

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

	// Start background notification generator
	go app.startBackgroundNotificationGenerator()

	// Start the server
	err = http.ListenAndServe(fmt.Sprintf(":%s", webPort), app.routes())
	if err != nil {
		app.ErrorLog.Fatal(err)
	}
}

// startBackgroundNotificationGenerator starts a goroutine that periodically generates notifications for all devices
func (app *Config) startBackgroundNotificationGenerator() {
	// Define how often to check for and generate notifications (e.g., every 30 minutes)
	notificationInterval := 1 * time.Hour // Increase to 1 hour to prevent too frequent notifications

	app.InfoLog.Printf("Starting background notification generator with interval of %v", notificationInterval)

	// Create a ticker that triggers at the defined interval
	ticker := time.NewTicker(notificationInterval)
	defer ticker.Stop()

	// Run the first check immediately
	app.checkAllDevicesForNotifications()

	// Then run periodically based on the ticker
	for range ticker.C {
		app.checkAllDevicesForNotifications()
	}
}

// checkAllDevicesForNotifications checks all devices for conditions that would trigger notifications
func (app *Config) checkAllDevicesForNotifications() {
	app.InfoLog.Println("Starting periodic notification generation for all devices")

	// Get all devices from the database
	devices, err := app.Models.Device.GetAll()
	if err != nil {
		app.ErrorLog.Printf("Error fetching devices for notification generation: %v", err)
		return
	}

	app.InfoLog.Printf("Checking %d devices for notification conditions", len(devices))

	// Use a channel with a reasonable buffer to control concurrent processing
	type deviceResult struct {
		serialNumber  string
		notifications int
		err           error
	}

	resultChan := make(chan deviceResult, 5)
	activeWorkers := 0
	maxConcurrentWorkers := 5 // Control concurrency

	// Process each device concurrently with controlled concurrency
	for _, device := range devices {
		// Skip devices without a serial number
		if device.SerialNumber == "" {
			continue
		}

		// Launch worker goroutines with controlled concurrency
		activeWorkers++
		if activeWorkers <= maxConcurrentWorkers {
			go func(d *data.Device) {
				// Generate notifications for this device using its owner's user ID
				notificationCount, err := app.generateNotificationsForDevice(d.UserID, d.SerialNumber)

				// Send results through channel
				resultChan <- deviceResult{
					serialNumber:  d.SerialNumber,
					notifications: notificationCount,
					err:           err,
				}
			}(device)
		} else {
			// Wait for a worker to finish before starting a new one
			result := <-resultChan
			activeWorkers--

			if result.err != nil {
				app.ErrorLog.Printf("Error generating notifications for device %s: %v",
					result.serialNumber, result.err)
			} else {
				app.InfoLog.Printf("Generated %d notifications for device %s",
					result.notifications, result.serialNumber)
			}

			// Launch the next worker
			go func(d *data.Device) {
				notificationCount, err := app.generateNotificationsForDevice(d.UserID, d.SerialNumber)
				resultChan <- deviceResult{
					serialNumber:  d.SerialNumber,
					notifications: notificationCount,
					err:           err,
				}
			}(device)
			activeWorkers++
		}
	}

	// Collect remaining results
	for i := 0; i < activeWorkers; i++ {
		result := <-resultChan
		if result.err != nil {
			app.ErrorLog.Printf("Error generating notifications for device %s: %v",
				result.serialNumber, result.err)
		} else {
			app.InfoLog.Printf("Generated %d notifications for device %s",
				result.notifications, result.serialNumber)
		}
	}

	app.InfoLog.Println("Completed periodic notification generation for all devices")
}

// ensureSystemUserExists ensures that a system user exists for device auto-registration
func (app *Config) ensureSystemUserExists() error {
	// Define the system user - using ID 1 since auto-increment typically starts at 1
	systemUser := &data.User{
		Username: "system",
		Email:    "system@fieldeyes.internal", // Use a special internal email that won't conflict with real users
		Password: "notaccessible",             // This user should never be logged into
		Role:     "system",
	}

	// Check if the system user already exists
	existingUser, err := app.Models.User.GetByEmail(systemUser.Email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to check if system user exists: %w", err)
	}

	// If the user already exists, we're good
	if existingUser != nil {
		app.InfoLog.Printf("System user already exists with ID: %d", existingUser.ID)
		return nil
	}

	// Create the system user
	app.InfoLog.Println("Creating system user for device auto-registration")
	systemUser.TempPassword = systemUser.Password // Needed for the Insert function which expects the password in TempPassword

	// Insert the system user - the ID will be assigned by the database
	systemID, err := app.Models.User.Insert(systemUser)
	if err != nil {
		return fmt.Errorf("failed to create system user: %w", err)
	}

	app.InfoLog.Printf("System user created with ID: %d", systemID)
	return nil
}
