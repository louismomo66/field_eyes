package mqtt

import (
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type MQTTClient struct {
	client mqtt.Client
}

func NewMQTTClient(brokerURL, clientID string) (*MQTTClient, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(brokerURL)
	opts.SetClientID(clientID)
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(1 * time.Second)
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		fmt.Printf("Connection lost: %v\n", err)
	})

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("failed to connect: %v", token.Error())
	}

	return &MQTTClient{client: client}, nil
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

func (m *MQTTClient) Disconnect() {
	if m.client != nil && m.client.IsConnected() {
		m.client.Disconnect(250)
	}
}

func (m *MQTTClient) IsConnected() bool {
	return m.client != nil && m.client.IsConnected()
}
