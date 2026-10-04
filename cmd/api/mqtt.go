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
	clientID := fmt.Sprintf("field_eyes_server_%d", time.Now().UnixNano())
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
	if username := os.Getenv("MQTT_USERNAME"); username != "" {
		opts.SetUsername(username)
		opts.SetPassword(os.Getenv("MQTT_PASSWORD"))
	}
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

	// Valve devices report their position here (the schedule itself goes out on <serial>/valve/config)
	valveTopic := fmt.Sprintf("%s/+/valve", m.topicRoot)
	if err := m.Subscribe(valveTopic, m.handleValveStatus); err != nil {
		return fmt.Errorf("failed to subscribe to topic %s: %v", valveTopic, err)
	}
	m.app.InfoLog.Printf("MQTT client subscribed to topic: %s", valveTopic)

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
		// Get system user for auto-registration
		systemUser, err := m.app.Models.User.GetByEmail("system@fieldeyes.internal")
		if err != nil || systemUser == nil {
			return fmt.Errorf("failed to get system user for auto-registration: %v", err)
		}

		// Auto-register the device without assigning to a user
		device = &data.Device{
			DeviceType:   "auto_registered",
			SerialNumber: logEntry.SerialNumber,
			UserID:       systemUser.ID, // Assign to system user so it shows up as unclaimed
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

// PublishRetained publishes at QoS 1 with the retain flag, so the broker keeps the message
// and delivers it to a device whenever it next subscribes. It gives up after 5 seconds.
func (m *MQTTClient) PublishRetained(topic string, payload []byte) error {
	token := m.client.Publish(topic, 1, true, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("publish to %s timed out", topic)
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("publish error: %v", err)
	}
	return nil
}

// handleValveStatus records what a valve device reports on <root>/<serial>/valve.
func (m *MQTTClient) handleValveStatus(client mqtt.Client, msg mqtt.Message) {
	parts := strings.Split(msg.Topic(), "/")
	if len(parts) < 2 {
		return
	}
	serialNumber := parts[len(parts)-2]

	var status struct {
		SerialNumber string `json:"serial_number"`
		Valve        string `json:"valve"`
		Mode         string `json:"mode"`
		Version      uint   `json:"version"`
		ChangedAt    int64  `json:"changed_at"`
	}
	if err := json.Unmarshal(msg.Payload(), &status); err != nil {
		m.app.ErrorLog.Printf("Ignoring malformed valve status on %s: %v", msg.Topic(), err)
		return
	}
	// The broker is open, so only accept reports for valve devices, from themselves
	if status.SerialNumber != serialNumber {
		return
	}
	cd, err := m.app.Models.ControlDevice.GetBySerialNumber(serialNumber)
	if err != nil || cd == nil || cd.Kind != data.ControlKindValve {
		return
	}
	valve := strings.ToUpper(status.Valve)
	if valve != "OPEN" && valve != "CLOSED" && valve != "UNKNOWN" {
		return
	}

	var changedAt *time.Time
	if status.ChangedAt > 1735689600 { // after 2025-01-01: the device's clock was set
		t := time.Unix(status.ChangedAt, 0).UTC()
		changedAt = &t
	}
	if err := m.app.Models.ValveConfig.UpdateReport(serialNumber, valve, status.Mode, status.Version, time.Now().UTC(), changedAt); err != nil {
		m.app.ErrorLog.Printf("Could not record valve status for %s: %v", serialNumber, err)
		return
	}
	m.app.InfoLog.Printf("Valve %s reports %s (mode %s, config v%d)", serialNumber, valve, status.Mode, status.Version)
}

func (m *MQTTClient) IsConnected() bool {
	return m.client != nil && m.client.IsConnected()
}
