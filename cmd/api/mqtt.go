package main

import (
	"encoding/json"
	"field_eyes/data"
	"fmt"
	"os"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTTClient represents the MQTT client with its associated configuration
type MQTTClient struct {
	Client     mqtt.Client
	BrokerURL  string
	ClientID   string
	Username   string
	Password   string
	TopicRoot  string
	Qos        byte
	app        *Config // Reference to the app configuration
	bufferSize int     // Maximum recommended message size
}

// Message buffer for reassembling multi-part messages
type messageBuffer struct {
	Parts        map[int][]byte
	TotalParts   int
	ReceivedTime time.Time
	IsComplete   bool
}

// Map to store message buffers by device serial number
var messageBuffers = make(map[string]*messageBuffer)

// NewMQTTClient creates a new MQTT client with the provided configuration
func NewMQTTClient(app *Config) (*MQTTClient, error) {
	// Get configuration from environment variables
	brokerURL := os.Getenv("MQTT_BROKER_URL")
	if brokerURL == "" {
		brokerURL = "tcp://localhost:1883" // Default
	}

	// In Docker environment, use the service name instead of localhost
	if os.Getenv("DOCKER_ENV") == "true" && strings.Contains(brokerURL, "localhost") {
		brokerURL = strings.Replace(brokerURL, "localhost", "mosquitto", 1)
		app.InfoLog.Printf("Docker environment detected: Using mosquitto service instead of localhost")
	} else if strings.Contains(brokerURL, "localhost") {
		// Fix localhost references to use IPv4 instead of IPv6
		brokerURL = strings.Replace(brokerURL, "localhost", "127.0.0.1", 1)
		app.InfoLog.Printf("Using IPv4 address for MQTT: %s", brokerURL)
	}

	username := os.Getenv("MQTT_USERNAME")
	password := os.Getenv("MQTT_PASSWORD")
	clientID := os.Getenv("MQTT_CLIENT_ID")
	if clientID == "" {
		clientID = "field_eyes_server"
	}

	topicRoot := os.Getenv("MQTT_TOPIC_ROOT")
	if topicRoot == "" {
		topicRoot = "field_eyes/devices"
	}

	// Buffer size in bytes (default: 4KB)
	bufferSize := 4096

	// Create MQTT client options
	opts := mqtt.NewClientOptions()
	opts.AddBroker(brokerURL)
	opts.SetClientID(clientID)
	if username != "" {
		opts.SetUsername(username)
		opts.SetPassword(password)
	}

	// Set up auto reconnect
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(1 * time.Minute)
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		app.ErrorLog.Printf("MQTT connection lost: %v", err)
	})
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		app.InfoLog.Printf("MQTT client connected to broker at %s", brokerURL)
	})

	// Connect with timeout handling
	var client mqtt.Client
	client = mqtt.NewClient(opts)

	// Set appropriate timeout
	connectTimeout := 10 * time.Second

	// Connect with timeout
	app.InfoLog.Printf("Connecting to MQTT broker at %s", brokerURL)
	connectChan := make(chan error, 1)
	go func() {
		token := client.Connect()
		token.Wait()
		connectChan <- token.Error()
	}()

	// Wait for connection with timeout
	var err error
	select {
	case err = <-connectChan:
		if err != nil {
			return nil, fmt.Errorf("failed to connect to MQTT broker: %v", err)
		}
		app.InfoLog.Printf("MQTT connection successful")
	case <-time.After(connectTimeout):
		app.InfoLog.Printf("MQTT connection timed out after %v, continuing without MQTT", connectTimeout)
		return nil, fmt.Errorf("MQTT connection timed out")
	}

	mqttClient := &MQTTClient{
		Client:     client,
		BrokerURL:  brokerURL,
		ClientID:   clientID,
		Username:   username,
		Password:   password,
		TopicRoot:  topicRoot,
		Qos:        1, // Default QoS level
		app:        app,
		bufferSize: bufferSize,
	}

	return mqttClient, nil
}

// StartDeviceDataListener starts the MQTT subscription for device data
func (m *MQTTClient) StartDeviceDataListener() error {
	// Subscribe to standard single-message data format
	dataTopic := fmt.Sprintf("%s/+/data", m.TopicRoot)
	if token := m.Client.Subscribe(dataTopic, m.Qos, m.handleDeviceData); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to subscribe to topic %s: %v", dataTopic, token.Error())
	}
	m.app.InfoLog.Printf("MQTT client subscribed to topic: %s", dataTopic)

	// Subscribe to chunked data format
	chunkedTopic := fmt.Sprintf("%s/+/chunked/#", m.TopicRoot)
	if token := m.Client.Subscribe(chunkedTopic, m.Qos, m.handleChunkedData); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to subscribe to topic %s: %v", chunkedTopic, token.Error())
	}
	m.app.InfoLog.Printf("MQTT client subscribed to topic: %s", chunkedTopic)

	// Start a goroutine to clean up stale message buffers
	go m.cleanupStaleBuffers()

	return nil
}

// handleDeviceData processes incoming device data messages (single message format)
func (m *MQTTClient) handleDeviceData(client mqtt.Client, msg mqtt.Message) {
	// Log received message
	m.app.InfoLog.Printf("Received MQTT message on topic: %s", msg.Topic())

	// Parse the device data from the message payload
	var logEntry data.DeviceData
	if err := json.Unmarshal(msg.Payload(), &logEntry); err != nil {
		m.app.ErrorLog.Printf("Error unmarshaling device data: %v", err)
		return
	}

	// Process the device data using the same logic as the HTTP endpoint
	m.processDeviceData(&logEntry)
}

// handleChunkedData processes incoming chunked device data messages
func (m *MQTTClient) handleChunkedData(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	m.app.InfoLog.Printf("Received chunked MQTT message on topic: %s", topic)

	// Parse the topic to extract information (format: root/serialnumber/chunked/total/part)
	// Example: field_eyes/devices/SN12345678/chunked/3/1
	parts := strings.Split(topic, "/")
	if len(parts) < 6 {
		m.app.ErrorLog.Printf("Invalid chunked topic format: %s", topic)
		return
	}

	serialNumber := parts[2]
	totalParts := 0
	partNumber := 0

	fmt.Sscanf(parts[4], "%d", &totalParts)
	fmt.Sscanf(parts[5], "%d", &partNumber)

	if totalParts <= 0 || partNumber <= 0 || partNumber > totalParts {
		m.app.ErrorLog.Printf("Invalid chunking parameters: total=%d, part=%d", totalParts, partNumber)
		return
	}

	// Get or create a buffer for this message
	buffer, exists := messageBuffers[serialNumber]
	if !exists || buffer.TotalParts != totalParts {
		messageBuffers[serialNumber] = &messageBuffer{
			Parts:        make(map[int][]byte),
			TotalParts:   totalParts,
			ReceivedTime: time.Now(),
			IsComplete:   false,
		}
		buffer = messageBuffers[serialNumber]
	}

	// Store this part
	buffer.Parts[partNumber] = msg.Payload()

	// Check if we have all parts
	if len(buffer.Parts) == buffer.TotalParts {
		// Reassemble the message
		var completeMessage []byte
		for i := 1; i <= buffer.TotalParts; i++ {
			completeMessage = append(completeMessage, buffer.Parts[i]...)
		}

		// Process the complete message
		var logEntry data.DeviceData
		if err := json.Unmarshal(completeMessage, &logEntry); err != nil {
			m.app.ErrorLog.Printf("Error unmarshaling reassembled device data: %v", err)
			return
		}

		// Mark buffer as complete and process the data
		buffer.IsComplete = true
		m.app.InfoLog.Printf("Successfully reassembled %d chunks for device %s", totalParts, serialNumber)
		m.processDeviceData(&logEntry)
	}
}

// cleanupStaleBuffers removes old incomplete message buffers
func (m *MQTTClient) cleanupStaleBuffers() {
	ticker := time.NewTicker(10 * time.Minute)
	for range ticker.C {
		now := time.Now()
		for serialNumber, buffer := range messageBuffers {
			// If buffer is older than 1 hour and not complete, remove it
			if now.Sub(buffer.ReceivedTime) > time.Hour && !buffer.IsComplete {
				m.app.InfoLog.Printf("Cleaning up stale message buffer for device %s", serialNumber)
				delete(messageBuffers, serialNumber)
			}

			// If buffer is complete and older than 5 minutes, remove it
			if buffer.IsComplete && now.Sub(buffer.ReceivedTime) > 5*time.Minute {
				delete(messageBuffers, serialNumber)
			}
		}
	}
}

// processDeviceData handles the device data logging logic (reusing logic from HTTP endpoint)
func (m *MQTTClient) processDeviceData(logEntry *data.DeviceData) {
	// Check if the device exists by serial number
	device, err := m.app.Models.Device.GetBySerialNumber(logEntry.SerialNumber)
	if err != nil || device == nil {
		// Device doesn't exist, auto-register it
		m.app.InfoLog.Printf("Device with serial number %s not found, auto-registering", logEntry.SerialNumber)

		// Create new device with no user association
		newDevice := data.Device{
			DeviceType:   "auto_registered", // Default device type
			SerialNumber: logEntry.SerialNumber,
			// No UserID - will be assigned when a user claims it
		}

		// Save device without assigning to a user
		if err := m.app.Models.Device.CreateDevice(&newDevice); err != nil {
			m.app.ErrorLog.Printf("Failed to auto-register device: %v", err)
			return
		}

		m.app.InfoLog.Printf("Successfully auto-registered device with serial number: %s", logEntry.SerialNumber)

		// Retrieve the newly created device
		device, err = m.app.Models.Device.GetBySerialNumber(logEntry.SerialNumber)
		if err != nil {
			m.app.ErrorLog.Printf("Failed to retrieve auto-registered device: %v", err)
			return
		}
	}

	// Link the log entry to the device using its DeviceID
	logEntry.DeviceID = device.ID

	// Save the log entry
	err = m.app.Models.DeviceData.CreateLog(logEntry)
	if err != nil {
		m.app.ErrorLog.Printf("Failed to log device data: %v", err)
		return
	}

	// Invalidate the cache for this device's logs if Redis is available
	if m.app.Redis != nil {
		// Invalidate by device ID
		if err := m.app.Redis.InvalidateDeviceLogsCache(device.ID); err != nil {
			m.app.ErrorLog.Printf("Failed to invalidate device logs cache: %v", err)
		} else {
			m.app.InfoLog.Printf("Successfully invalidated cache for device %s", logEntry.SerialNumber)
		}

		// Also invalidate any cache by serial number
		keyToInvalidate := fmt.Sprintf("device_logs_serial:%s", logEntry.SerialNumber)
		if err := m.app.Redis.InvalidateCache(keyToInvalidate); err != nil {
			m.app.ErrorLog.Printf("Failed to invalidate device logs by serial cache: %v", err)
		}
	}

	m.app.InfoLog.Printf("Successfully logged MQTT device data for device %s", logEntry.SerialNumber)
}

// CloseConnection gracefully closes the MQTT connection
func (m *MQTTClient) CloseConnection() {
	if m.Client.IsConnected() {
		m.Client.Disconnect(250) // Wait 250ms for graceful disconnect
	}
}
