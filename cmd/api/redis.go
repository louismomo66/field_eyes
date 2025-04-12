package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/gomodule/redigo/redis"
)

// Redis cache expiration times
const (
	// Short-lived cache for frequently changing data
	ShortCacheDuration = 5 * time.Minute
	// Medium-lived cache for data that changes occasionally
	MediumCacheDuration = 30 * time.Minute
	// Long-lived cache for relatively static data
	LongCacheDuration = 24 * time.Hour
)

// DeviceDataForCache represents the device data for caching
type DeviceDataForCache struct {
	ID              uint      `json:"id"`
	DeviceID        uint      `json:"device_id"`
	SerialNumber    string    `json:"serial_number"`
	Temperature     float64   `json:"temperature"`
	Humidity        float64   `json:"humidity"`
	Nitrogen        float64   `json:"nitrogen"`
	Phosphorous     float64   `json:"phosphorous"`
	Potassium       float64   `json:"potassium"`
	PH              float64   `json:"ph"`
	SoilMoisture    float64   `json:"soil_moisture"`
	SoilTemperature float64   `json:"soil_temperature"`
	SoilHumidity    float64   `json:"soil_humidity"`
	Longitude       float64   `json:"longitude"`
	Latitude        float64   `json:"latitude"`
	CreatedAt       time.Time `json:"created_at"`
}

// DeviceForCache represents the device for caching
type DeviceForCache struct {
	ID           uint      `json:"id"`
	DeviceType   string    `json:"device_type"`
	SerialNumber string    `json:"serial_number"`
	UserID       uint      `json:"user_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// RedisClient wraps the Redis connection pool with application-specific methods
type RedisClient struct {
	Pool *redis.Pool
}

// NewRedisClient creates and initializes a new Redis client connection pool
func NewRedisClient() (*RedisClient, error) {
	// Get Redis connection parameters from environment variables or use defaults
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}

	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}

	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisAddr := fmt.Sprintf("%s:%s", redisHost, redisPort)

	// Create a Redis connection pool
	pool := &redis.Pool{
		MaxIdle:     10,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", redisAddr)
			if err != nil {
				return nil, err
			}
			if redisPassword != "" {
				if _, err := c.Do("AUTH", redisPassword); err != nil {
					c.Close()
					return nil, err
				}
			}
			return c, nil
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, err := c.Do("PING")
			return err
		},
	}

	// Test connection
	conn := pool.Get()
	defer conn.Close()

	_, err := conn.Do("PING")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisClient{Pool: pool}, nil
}

// Close closes the Redis client connection pool
func (r *RedisClient) Close() error {
	return r.Pool.Close()
}

// CacheDeviceLogs stores device logs in Redis with expiration
func (r *RedisClient) CacheDeviceLogs(deviceID uint, logs []*DeviceDataForCache) error {
	key := fmt.Sprintf("device_logs:%d", deviceID)
	data, err := json.Marshal(logs)
	if err != nil {
		return err
	}

	conn := r.Pool.Get()
	defer conn.Close()

	_, err = conn.Do("SETEX", key, int(MediumCacheDuration.Seconds()), data)
	return err
}

// GetCachedDeviceLogs retrieves device logs from Redis cache
func (r *RedisClient) GetCachedDeviceLogs(deviceID uint) ([]*DeviceDataForCache, error) {
	key := fmt.Sprintf("device_logs:%d", deviceID)

	conn := r.Pool.Get()
	defer conn.Close()

	data, err := redis.Bytes(conn.Do("GET", key))
	if err != nil {
		if err == redis.ErrNil {
			return nil, nil // Cache miss, not an error
		}
		return nil, err
	}

	var logs []*DeviceDataForCache
	err = json.Unmarshal(data, &logs)
	if err != nil {
		return nil, err
	}

	return logs, nil
}

// CacheDeviceLogsBySerial stores device logs by serial number in Redis
func (r *RedisClient) CacheDeviceLogsBySerial(serialNumber string, logs []*DeviceDataForCache) error {
	key := fmt.Sprintf("device_logs_serial:%s", serialNumber)
	data, err := json.Marshal(logs)
	if err != nil {
		return err
	}

	conn := r.Pool.Get()
	defer conn.Close()

	_, err = conn.Do("SETEX", key, int(MediumCacheDuration.Seconds()), data)
	return err
}

// GetCachedDeviceLogsBySerial retrieves device logs by serial number from Redis
func (r *RedisClient) GetCachedDeviceLogsBySerial(serialNumber string) ([]*DeviceDataForCache, error) {
	key := fmt.Sprintf("device_logs_serial:%s", serialNumber)

	conn := r.Pool.Get()
	defer conn.Close()

	data, err := redis.Bytes(conn.Do("GET", key))
	if err != nil {
		if err == redis.ErrNil {
			return nil, nil // Cache miss, not an error
		}
		return nil, err
	}

	var logs []*DeviceDataForCache
	err = json.Unmarshal(data, &logs)
	if err != nil {
		return nil, err
	}

	return logs, nil
}

// CacheUserDevices stores a user's devices in Redis
func (r *RedisClient) CacheUserDevices(userID uint, devices []*DeviceForCache) error {
	key := fmt.Sprintf("user_devices:%d", userID)
	data, err := json.Marshal(devices)
	if err != nil {
		return err
	}

	conn := r.Pool.Get()
	defer conn.Close()

	_, err = conn.Do("SETEX", key, int(MediumCacheDuration.Seconds()), data)
	return err
}

// GetCachedUserDevices retrieves a user's devices from Redis
func (r *RedisClient) GetCachedUserDevices(userID uint) ([]*DeviceForCache, error) {
	key := fmt.Sprintf("user_devices:%d", userID)

	conn := r.Pool.Get()
	defer conn.Close()

	data, err := redis.Bytes(conn.Do("GET", key))
	if err != nil {
		if err == redis.ErrNil {
			return nil, nil // Cache miss, not an error
		}
		return nil, err
	}

	var devices []*DeviceForCache
	err = json.Unmarshal(data, &devices)
	if err != nil {
		return nil, err
	}

	return devices, nil
}

// CacheMLAnalysisResults stores ML analysis results in Redis
func (r *RedisClient) CacheMLAnalysisResults(deviceID uint, analysisType string, results interface{}) error {
	key := fmt.Sprintf("ml_analysis:%d:%s", deviceID, analysisType)
	data, err := json.Marshal(results)
	if err != nil {
		return err
	}

	conn := r.Pool.Get()
	defer conn.Close()

	_, err = conn.Do("SETEX", key, int(ShortCacheDuration.Seconds()), data)
	return err
}

// GetCachedMLAnalysisResults retrieves ML analysis results from Redis
func (r *RedisClient) GetCachedMLAnalysisResults(deviceID uint, analysisType string, result interface{}) error {
	key := fmt.Sprintf("ml_analysis:%d:%s", deviceID, analysisType)

	conn := r.Pool.Get()
	defer conn.Close()

	data, err := redis.Bytes(conn.Do("GET", key))
	if err != nil {
		if err == redis.ErrNil {
			return err // Cache miss
		}
		return err
	}

	return json.Unmarshal(data, result)
}

// InvalidateCache removes cached data for a specific key
func (r *RedisClient) InvalidateCache(key string) error {
	conn := r.Pool.Get()
	defer conn.Close()

	_, err := conn.Do("DEL", key)
	return err
}

// InvalidateDeviceLogsCache removes cached device logs
func (r *RedisClient) InvalidateDeviceLogsCache(deviceID uint) error {
	key := fmt.Sprintf("device_logs:%d", deviceID)

	conn := r.Pool.Get()
	defer conn.Close()

	_, err := conn.Do("DEL", key)
	return err
}

// InvalidateUserDevicesCache removes cached user devices
func (r *RedisClient) InvalidateUserDevicesCache(userID uint) error {
	key := fmt.Sprintf("user_devices:%d", userID)

	conn := r.Pool.Get()
	defer conn.Close()

	_, err := conn.Do("DEL", key)
	return err
}
