package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"field_eyes/data"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTTClient represents the MQTT client with its associated configuration
type MQTTClient struct {
	client     mqtt.Client
	app        *Config
	topicRoot  string
	bufferSize int
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
	brokerURL := fmt.Sprintf("tcp://%s:%s", os.Getenv("MQTT_BROKER"), os.Getenv("MQTT_PORT"))
	clientID := fmt.Sprintf("field_eyes_server_%d_%d", time.Now().UnixNano(), os.Getpid())
	topicRoot := os.Getenv("MQTT_TOPIC_ROOT")
	if topicRoot == "" {
		topicRoot = "field_eyes/devices"
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(brokerURL)
	opts.SetClientID(clientID)
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(1 * time.Second)
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		app.ErrorLog.Printf("MQTT connection lost: %v", err)
	})

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("failed to connect: %v", token.Error())
	}

	return &MQTTClient{
		client:     client,
		app:        app,
		topicRoot:  topicRoot,
		bufferSize: 4096, // 4KB buffer size
	}, nil
}

// StartDeviceDataListener starts the MQTT subscription for device data
func (m *MQTTClient) StartDeviceDataListener() error {
	// Subscribe to standard single-message data format
	dataTopic := fmt.Sprintf("%s/+/data", m.topicRoot)
	if err := m.Subscribe(dataTopic, m.handleDeviceData); err != nil {
		return fmt.Errorf("failed to subscribe to topic %s: %v", dataTopic, err)
	}
	m.app.InfoLog.Printf("MQTT client subscribed to topic: %s", dataTopic)

	// Subscribe to chunked data format
	chunkedTopic := fmt.Sprintf("%s/+/chunked/#", m.topicRoot)
	if err := m.Subscribe(chunkedTopic, m.handleChunkedData); err != nil {
		return fmt.Errorf("failed to subscribe to topic %s: %v", chunkedTopic, err)
	}
	m.app.InfoLog.Printf("MQTT client subscribed to topic: %s", chunkedTopic)

	// Start a goroutine to clean up stale message buffers
	go m.cleanupStaleBuffers()

	return nil
}

// handleDeviceData processes incoming device data messages (single message format)
func (m *MQTTClient) handleDeviceData(client mqtt.Client, msg mqtt.Message) {
	var logEntry data.DeviceData
	if err := json.Unmarshal(msg.Payload(), &logEntry); err != nil {
		m.app.ErrorLog.Printf("Error unmarshaling device data: %v", err)
		return
	}

	if err := m.processDeviceData(&logEntry); err != nil {
		m.app.ErrorLog.Printf("Error processing device data: %v", err)
	}
}

// handleChunkedData processes incoming chunked device data messages
func (m *MQTTClient) handleChunkedData(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	m.app.InfoLog.Printf("Received chunked MQTT message on topic: %s", topic)

	// Parse the topic to extract information (format: root/serialnumber/chunked/total/part)
	parts := strings.Split(topic, "/")
	if len(parts) < 6 {
		m.app.ErrorLog.Printf("Invalid chunked topic format: %s", topic)
		return
	}

	var logEntry data.DeviceData
	if err := json.Unmarshal(msg.Payload(), &logEntry); err != nil {
		m.app.ErrorLog.Printf("Error unmarshaling device data: %v", err)
		return
	}

	if err := m.processDeviceData(&logEntry); err != nil {
		m.app.ErrorLog.Printf("Error processing chunked device data: %v", err)
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
				fmt.Printf("Cleaning up stale message buffer for device %s\n", serialNumber)
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
func (m *MQTTClient) processDeviceData(logEntry *data.DeviceData) error {
	// Check if the device exists
	device, err := m.app.Models.Device.GetBySerialNumber(logEntry.SerialNumber)
	if err != nil {
		// Auto-register the device
		device = &data.Device{
			DeviceType:   "auto_registered",
			SerialNumber: logEntry.SerialNumber,
		}
		if err := m.app.Models.Device.CreateDevice(device); err != nil {
			return fmt.Errorf("failed to auto-register device: %v", err)
		}
		m.app.InfoLog.Printf("Auto-registered device: %s", logEntry.SerialNumber)
	}

	// Link the log entry to the device
	logEntry.DeviceID = device.ID

	// Save the log entry
	if err := m.app.Models.DeviceData.CreateLog(logEntry); err != nil {
		return fmt.Errorf("failed to save device data: %v", err)
	}

	// Invalidate cache if Redis is available
	if m.app.Redis != nil {
		if err := m.app.Redis.InvalidateDeviceLogsCache(device.ID); err != nil {
			m.app.ErrorLog.Printf("Failed to invalidate device logs cache: %v", err)
		}
		cacheKey := fmt.Sprintf("device_logs_serial:%s", logEntry.SerialNumber)
		if err := m.app.Redis.InvalidateCache(cacheKey); err != nil {
			m.app.ErrorLog.Printf("Failed to invalidate device logs by serial cache: %v", err)
		}
	}

	m.app.InfoLog.Printf("Successfully logged data for device: %s", logEntry.SerialNumber)
	return nil
}

// CloseConnection gracefully closes the MQTT connection
func (m *MQTTClient) CloseConnection() {
	if m.client.IsConnected() {
		m.client.Disconnect(250) // Wait 250ms for graceful disconnect
	}
}

func (app *Config) connectMQTT() error {
	mqttClient, err := NewMQTTClient(app)
	if err != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %v", err)
	}

	app.MQTT = mqttClient
	app.InfoLog.Printf("Connected to MQTT broker successfully")

	// Start listening for device data
	if err := mqttClient.StartDeviceDataListener(); err != nil {
		return fmt.Errorf("failed to start device data listener: %v", err)
	}

	return nil
}

func (m *MQTTClient) Subscribe(topic string, handler mqtt.MessageHandler) error {
	if token := m.client.Subscribe(topic, 0, handler); token.Wait() && token.Error() != nil {
		return fmt.Errorf("subscribe error: %v", token.Error())
	}
	return nil
}

func (m *MQTTClient) Publish(topic string, payload interface{}) error {
	if token := m.client.Publish(topic, 0, false, payload); token.Wait() && token.Error() != nil {
		return fmt.Errorf("publish error: %v", token.Error())
	}
	return nil
}

func (m *MQTTClient) IsConnected() bool {
	return m.client != nil && m.client.IsConnected()
}
