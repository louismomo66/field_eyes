package main

import (
	"field_eyes/data"
	"field_eyes/pkg/email"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
)

const webPort = "9004"

func (app *Config) serve() {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", webPort),
		Handler: app.routes(),
	}
	app.InfoLog.Println("Starting web server...")
	err := srv.ListenAndServe()
	if err != nil {
		log.Panic(err)
	}
}
func main() {
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

	// Initialize the mailer
	// In production, use SMTPMailer
	// app.Mailer = email.NewSMTPMailer()

	// For development/testing, use MockMailer
	app.Mailer = &email.MockMailer{}

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

	go app.listenForErrors()
	app.serve()
}
