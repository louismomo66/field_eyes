package main

import (
	"errors"
	"field_eyes/data"
	"fmt"
	"net/http"
	"time"

	"gorm.io/gorm"
)

func (app *Config) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	// Extract user information from the token
	userID, _, _, err := app.GetUserInfoFromToken(r)
	if err != nil {
		app.errorJSON(w, errors.New("unauthorized: invalid or missing token"), http.StatusUnauthorized)
		app.ErrorLog.Println(err)
		return
	}

	// Parse the request body into a Device struct
	var request struct {
		DeviceType   string `json:"device_type"`
		SerialNumber string `json:"serial_number"`
	}

	if err := app.ReadJSON(w, r, &request); err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}

	// Validate required fields
	if request.SerialNumber == "" {
		app.errorJSON(w, errors.New("serial number is required"), http.StatusBadRequest)
		app.ErrorLog.Println("serial number is required")
		return
	}

	// Check if the user already has a device with the same serial number
	devices, err := app.Models.Device.GetByUserID(userID)
	if err != nil {
		app.errorJSON(w, errors.New("failed to retrieve user's devices"), http.StatusInternalServerError)
		app.ErrorLog.Println(err)
		return
	}
	for _, d := range devices {
		if d.SerialNumber == request.SerialNumber {
			app.errorJSON(w, errors.New("user already has a device with this serial number"), http.StatusBadRequest)
			app.ErrorLog.Println("user already has a device with this serial number")
			return
		}
	}

	// Check if the device already exists in the system
	existingDevice, err := app.Models.Device.GetBySerialNumber(request.SerialNumber)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		app.errorJSON(w, errors.New("failed to check device serial number"), http.StatusInternalServerError)
		app.ErrorLog.Println(err)
		return
	}

	if existingDevice != nil {
		// Device exists, check if it's already assigned to another user
		if existingDevice.UserID != 0 && existingDevice.UserID != userID {
			app.errorJSON(w, errors.New("device is already assigned to another user"), http.StatusBadRequest)
			app.ErrorLog.Println("device is already assigned to another user")
			return
		}

		// Device exists and is either unassigned or already belongs to this user
		if existingDevice.UserID == userID {
			// Already registered to this user
			app.writeJSON(w, http.StatusOK, map[string]string{
				"message":       "device is already registered to your account",
				"device_id":     fmt.Sprintf("%d", existingDevice.ID),
				"serial_number": existingDevice.SerialNumber,
			})
			return
		}

		// Claim the auto-registered device
		app.InfoLog.Printf("User %d claiming auto-registered device %s", userID, request.SerialNumber)

		// Update device type if provided
		if request.DeviceType != "" {
			existingDevice.DeviceType = request.DeviceType
		}

		// Assign to user
		existingDevice.UserID = userID

		// Update the device in the database
		if err := app.Models.Device.Update(existingDevice); err != nil {
			app.errorJSON(w, errors.New("failed to assign device to user"), http.StatusInternalServerError)
			app.ErrorLog.Printf("Failed to assign device to user: %v", err)
			return
		}

		// Invalidate cache if Redis is available
		if app.Redis != nil {
			go func(userID uint) {
				if err := app.Redis.InvalidateUserDevicesCache(userID); err != nil {
					app.ErrorLog.Printf("Failed to invalidate user devices cache: %v", err)
				}
			}(userID)
		}

		app.writeJSON(w, http.StatusOK, map[string]string{
			"message":       "device assigned to your account successfully",
			"device_id":     fmt.Sprintf("%d", existingDevice.ID),
			"serial_number": existingDevice.SerialNumber,
		})
		return
	}

	// Device doesn't exist, create a new one
	deviceType := request.DeviceType
	if deviceType == "" {
		deviceType = "user_registered" // Default type if none provided
	}

	newDevice := data.Device{
		DeviceType:   deviceType,
		SerialNumber: request.SerialNumber,
		UserID:       userID,
	}

	// Save the new device
	err = app.Models.Device.AssignDevice(userID, &newDevice)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		app.ErrorLog.Println(err)
		return
	}

	// Respond with success
	app.writeJSON(w, http.StatusCreated, map[string]string{
		"message":       "new device registered successfully",
		"serial_number": request.SerialNumber,
	})
}

func (app *Config) LogDeviceData(w http.ResponseWriter, r *http.Request) {
	// Parse the request body into a DeviceData struct
	var logEntry data.DeviceData
	if err := app.ReadJSON(w, r, &logEntry); err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}

	// Check if the device exists by serial number
	device, err := app.Models.Device.GetBySerialNumber(logEntry.SerialNumber)
	if err != nil || device == nil {
		// Device doesn't exist, auto-register it
		app.InfoLog.Printf("Device with serial number %s not found, auto-registering", logEntry.SerialNumber)

		// Create new device with no user association
		newDevice := data.Device{
			DeviceType:   "auto_registered", // Default device type
			SerialNumber: logEntry.SerialNumber,
			// No UserID - will be assigned when a user claims it
		}

		// Save device without assigning to a user
		if err := app.Models.Device.CreateDevice(&newDevice); err != nil {
			app.errorJSON(w, errors.New("failed to auto-register device"), http.StatusInternalServerError)
			app.ErrorLog.Printf("Failed to auto-register device: %v", err)
			return
		}

		app.InfoLog.Printf("Successfully auto-registered device with serial number: %s", logEntry.SerialNumber)

		// Retrieve the newly created device
		device, err = app.Models.Device.GetBySerialNumber(logEntry.SerialNumber)
		if err != nil {
			app.errorJSON(w, errors.New("failed to retrieve auto-registered device"), http.StatusInternalServerError)
			app.ErrorLog.Println(err)
			return
		}
	}

	// Link the log entry to the device using its DeviceID
	logEntry.DeviceID = device.ID

	// Save the log entry
	err = app.Models.DeviceData.CreateLog(&logEntry)
	if err != nil {
		app.errorJSON(w, errors.New("failed to log device data"), http.StatusInternalServerError)
		app.ErrorLog.Println("failed to log device data:", err)
		return
	}

	// Invalidate the cache for this device's logs if Redis is available
	if app.Redis != nil {
		go func(deviceID uint, serialNumber string) {
			// Invalidate by device ID
			if err := app.Redis.InvalidateDeviceLogsCache(deviceID); err != nil {
				app.ErrorLog.Printf("Failed to invalidate device logs cache: %v", err)
			} else {
				app.InfoLog.Printf("Successfully invalidated cache for device %s", serialNumber)
			}

			// Also invalidate any cache by serial number
			keyToInvalidate := fmt.Sprintf("device_logs_serial:%s", serialNumber)
			if err := app.Redis.InvalidateCache(keyToInvalidate); err != nil {
				app.ErrorLog.Printf("Failed to invalidate device logs by serial cache: %v", err)
			}
		}(device.ID, logEntry.SerialNumber)
	}

	// Respond with success
	userAssigned := "false"
	if device.UserID > 0 {
		userAssigned = "true"
	}

	app.writeJSON(w, http.StatusCreated, map[string]string{
		"message":       "device data logged successfully",
		"device_id":     fmt.Sprintf("%d", device.ID),
		"serial_number": device.SerialNumber,
		"registered":    "true",
		"user_assigned": userAssigned,
	})
}

func (app *Config) GetDeviceLogs(w http.ResponseWriter, r *http.Request) {
	// Extract user information from the token
	userID, _, _, err := app.GetUserInfoFromToken(r)
	if err != nil {
		app.errorJSON(w, errors.New("unauthorized: invalid or missing token"), http.StatusUnauthorized)
		app.ErrorLog.Println(err)
		return
	}

	// Get the device serial number from the query parameters
	serialNumber := r.URL.Query().Get("serial_number")
	if serialNumber == "" {
		app.errorJSON(w, errors.New("missing device serial number"), http.StatusBadRequest)
		app.ErrorLog.Println("missing device serial number")
		return
	}

	// Validate that the device exists and belongs to the user
	device, err := app.Models.Device.GetBySerialNumber(serialNumber)
	if err != nil {
		app.errorJSON(w, errors.New("device not found"), http.StatusNotFound)
		app.ErrorLog.Println("device not found:", err)
		return
	}
	if device.UserID != userID {
		app.errorJSON(w, errors.New("unauthorized: device does not belong to the user"), http.StatusUnauthorized)
		app.ErrorLog.Println("unauthorized: device does not belong to the user")
		return
	}

	var logs []*data.DeviceData
	var cacheHit bool = false

	// Try to get logs from Redis cache if Redis is available
	if app.Redis != nil {
		cachedLogs, err := app.Redis.GetCachedDeviceLogs(device.ID)
		if err != nil {
			app.ErrorLog.Printf("Redis cache error: %v", err)
		} else if cachedLogs != nil {
			// Convert the cached logs to device data objects
			logs = make([]*data.DeviceData, len(cachedLogs))
			for i, cl := range cachedLogs {
				logs[i] = &data.DeviceData{
					DeviceID:        cl.DeviceID,
					SerialNumber:    cl.SerialNumber,
					Temperature:     cl.Temperature,
					Humidity:        cl.Humidity,
					Nitrogen:        cl.Nitrogen,
					Phosphorous:     cl.Phosphorous,
					Potassium:       cl.Potassium,
					PH:              cl.PH,
					SoilMoisture:    cl.SoilMoisture,
					SoilTemperature: cl.SoilTemperature,
					SoilHumidity:    cl.SoilHumidity,
					Longitude:       cl.Longitude,
					Latitude:        cl.Latitude,
					CreatedAt:       cl.CreatedAt,
				}
			}
			cacheHit = true
			app.InfoLog.Printf("Cache hit for device logs: %s (ID: %d), found %d logs",
				serialNumber, device.ID, len(logs))
		}
	}

	// If not found in cache, retrieve from database
	if !cacheHit {
		app.InfoLog.Printf("Cache miss for device logs: %s (ID: %d)", serialNumber, device.ID)
		logs, err = app.Models.DeviceData.GetLogsByDeviceID(device.ID)
		if err != nil {
			app.errorJSON(w, errors.New("failed to retrieve device logs"), http.StatusInternalServerError)
			app.ErrorLog.Println("failed to retrieve device logs:", err)
			return
		}

		// Log the number of logs retrieved
		app.InfoLog.Printf("Retrieved %d logs for device %s (ID: %d) from database",
			len(logs), serialNumber, device.ID)

		// Store in cache for future requests if Redis is available
		if app.Redis != nil && len(logs) > 0 {
			go func() {
				// Convert from DeviceData to DeviceDataForCache for caching
				cacheableLogs := make([]*DeviceDataForCache, len(logs))
				for i, log := range logs {
					cacheableLogs[i] = &DeviceDataForCache{
						ID:              log.ID,
						DeviceID:        log.DeviceID,
						SerialNumber:    log.SerialNumber,
						Temperature:     log.Temperature,
						Humidity:        log.Humidity,
						Nitrogen:        log.Nitrogen,
						Phosphorous:     log.Phosphorous,
						Potassium:       log.Potassium,
						PH:              log.PH,
						SoilMoisture:    log.SoilMoisture,
						SoilTemperature: log.SoilTemperature,
						SoilHumidity:    log.SoilHumidity,
						Longitude:       log.Longitude,
						Latitude:        log.Latitude,
						CreatedAt:       log.CreatedAt,
					}
				}

				if err := app.Redis.CacheDeviceLogs(device.ID, cacheableLogs); err != nil {
					app.ErrorLog.Printf("Failed to cache device logs: %v", err)
				} else {
					app.InfoLog.Printf("Successfully cached %d logs for device %s",
						len(logs), serialNumber)
				}
			}()
		}
	}

	// Respond with the logs
	app.writeJSON(w, http.StatusOK, logs)
}

// AnalyzeDeviceData performs machine learning analysis on device data
func (app *Config) AnalyzeDeviceData(w http.ResponseWriter, r *http.Request) {
	// Extract user information from the token
	userID, _, _, err := app.GetUserInfoFromToken(r)
	if err != nil {
		app.errorJSON(w, errors.New("unauthorized: invalid or missing token"), http.StatusUnauthorized)
		app.ErrorLog.Println(err)
		return
	}

	// Get the device serial number and analysis type from the query parameters
	serialNumber := r.URL.Query().Get("serial_number")
	if serialNumber == "" {
		app.errorJSON(w, errors.New("missing device serial number"), http.StatusBadRequest)
		return
	}

	analysisType := r.URL.Query().Get("type")
	if analysisType == "" {
		analysisType = "soil" // Default analysis type
	}

	// Validate supported analysis types
	validTypes := map[string]bool{
		"soil":        true,
		"temperature": true,
		"moisture":    true,
		"nutrient":    true,
	}

	if !validTypes[analysisType] {
		app.errorJSON(w, errors.New("unsupported analysis type"), http.StatusBadRequest)
		return
	}

	// Validate that the device exists and belongs to the user
	device, err := app.Models.Device.GetBySerialNumber(serialNumber)
	if err != nil {
		app.errorJSON(w, errors.New("device not found"), http.StatusNotFound)
		return
	}
	if device.UserID != userID {
		app.errorJSON(w, errors.New("unauthorized: device does not belong to the user"), http.StatusUnauthorized)
		return
	}

	// Define result structure depending on analysis type
	type AnalysisResult struct {
		DeviceID        uint               `json:"device_id"`
		SerialNumber    string             `json:"serial_number"`
		AnalysisType    string             `json:"analysis_type"`
		Recommendations []string           `json:"recommendations"`
		Predictions     map[string]float64 `json:"predictions"`
		Trends          map[string]string  `json:"trends"`
		LastUpdated     time.Time          `json:"last_updated"`
	}

	var result AnalysisResult

	// Try to get analysis results from cache if Redis is available
	cacheHit := false
	if app.Redis != nil {
		err := app.Redis.GetCachedMLAnalysisResults(device.ID, analysisType, &result)
		if err == nil {
			cacheHit = true
			app.InfoLog.Printf("Cache hit for ML analysis: %s, device: %s", analysisType, serialNumber)
		}
	}

	// If not found in cache, perform the analysis
	if !cacheHit {
		app.InfoLog.Printf("Cache miss for ML analysis: %s, device: %s", analysisType, serialNumber)

		// Get device logs for analysis
		logs, err := app.Models.DeviceData.GetLogsByDeviceID(device.ID)
		if err != nil {
			app.errorJSON(w, errors.New("failed to retrieve device logs for analysis"), http.StatusInternalServerError)
			return
		}

		if len(logs) == 0 {
			app.errorJSON(w, errors.New("insufficient data for analysis"), http.StatusBadRequest)
			return
		}

		// Initialize the result structure
		result = AnalysisResult{
			DeviceID:        device.ID,
			SerialNumber:    serialNumber,
			AnalysisType:    analysisType,
			Recommendations: []string{},
			Predictions:     make(map[string]float64),
			Trends:          make(map[string]string),
			LastUpdated:     time.Now(),
		}

		// Perform the appropriate analysis based on the type
		switch analysisType {
		case "soil":
			// Simplified mock soil analysis
			result.Recommendations = append(result.Recommendations, "Based on soil pH and moisture levels, consider adjusting irrigation schedule")
			result.Predictions["optimal_ph"] = 6.5
			result.Trends["soil_moisture"] = "decreasing"

			// Add soil-specific analysis
			avgPH := calculateAverage(logs, func(l *data.DeviceData) float64 { return l.PH })
			if avgPH < 5.5 {
				result.Recommendations = append(result.Recommendations, "Soil is too acidic, consider adding lime")
			} else if avgPH > 7.5 {
				result.Recommendations = append(result.Recommendations, "Soil is too alkaline, consider adding sulfur")
			}

		case "temperature":
			// Simplified mock temperature analysis
			result.Recommendations = append(result.Recommendations, "Temperature variations suggest implementing shading during peak hours")
			result.Predictions["optimal_temp"] = 24.5
			result.Trends["temperature"] = "stable"

		case "moisture":
			// Simplified mock moisture analysis
			result.Recommendations = append(result.Recommendations, "Current moisture trends indicate potential over-irrigation")
			result.Predictions["optimal_irrigation"] = 45.0
			result.Trends["humidity"] = "increasing"

		case "nutrient":
			// Simplified mock nutrient analysis
			result.Recommendations = append(result.Recommendations, "Nitrogen levels are low, consider adding nitrogen-rich fertilizer")

			// Calculate nutrient levels
			avgN := calculateAverage(logs, func(l *data.DeviceData) float64 { return l.Nitrogen })
			avgP := calculateAverage(logs, func(l *data.DeviceData) float64 { return l.Phosphorous })
			avgK := calculateAverage(logs, func(l *data.DeviceData) float64 { return l.Potassium })

			result.Predictions["optimal_nitrogen"] = avgN + 15
			result.Predictions["optimal_phosphorous"] = avgP + 10
			result.Predictions["optimal_potassium"] = avgK + 5

			if avgN < 40 {
				result.Recommendations = append(result.Recommendations, "Nitrogen deficiency detected")
			}
			if avgP < 30 {
				result.Recommendations = append(result.Recommendations, "Phosphorous deficiency detected")
			}
			if avgK < 20 {
				result.Recommendations = append(result.Recommendations, "Potassium deficiency detected")
			}
		}

		// Store in cache for future requests if Redis is available
		if app.Redis != nil {
			go func() {
				if err := app.Redis.CacheMLAnalysisResults(device.ID, analysisType, result); err != nil {
					app.ErrorLog.Printf("Failed to cache ML analysis results: %v", err)
				} else {
					app.InfoLog.Printf("Successfully cached ML analysis for device %s", serialNumber)
				}
			}()
		}
	}

	// Respond with the analysis results
	app.writeJSON(w, http.StatusOK, result)
}

// Helper function to calculate the average of a specific field in device logs
func calculateAverage(logs []*data.DeviceData, valueFunc func(*data.DeviceData) float64) float64 {
	if len(logs) == 0 {
		return 0
	}

	sum := 0.0
	for _, log := range logs {
		sum += valueFunc(log)
	}

	return sum / float64(len(logs))
}

// ClaimDevice allows users to claim an auto-registered device
func (app *Config) ClaimDevice(w http.ResponseWriter, r *http.Request) {
	// Extract user information from the token
	userID, _, _, err := app.GetUserInfoFromToken(r)
	if err != nil {
		app.errorJSON(w, errors.New("unauthorized: invalid or missing token"), http.StatusUnauthorized)
		app.ErrorLog.Println(err)
		return
	}

	// Parse the request
	var request struct {
		SerialNumber string `json:"serial_number"`
	}

	if err := app.ReadJSON(w, r, &request); err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}

	// Validate that serial number is provided
	if request.SerialNumber == "" {
		app.errorJSON(w, errors.New("serial number is required"), http.StatusBadRequest)
		return
	}

	// Get the device by serial number
	device, err := app.Models.Device.GetBySerialNumber(request.SerialNumber)
	if err != nil || device == nil {
		app.errorJSON(w, errors.New("device not found"), http.StatusNotFound)
		app.ErrorLog.Printf("Device with serial number %s not found", request.SerialNumber)
		return
	}

	// Check if device is already claimed
	if device.UserID != 0 {
		// If already claimed by this user, return success
		if device.UserID == userID {
			app.writeJSON(w, http.StatusOK, map[string]string{
				"message":       "device is already registered to your account",
				"device_id":     fmt.Sprintf("%d", device.ID),
				"serial_number": device.SerialNumber,
			})
			return
		}

		// If claimed by another user, return error
		app.errorJSON(w, errors.New("device is already claimed by another user"), http.StatusBadRequest)
		return
	}

	// Update the device with the user ID
	device.UserID = userID

	// Perform update in the database
	if err := app.Models.Device.Update(device); err != nil {
		app.errorJSON(w, errors.New("failed to claim device"), http.StatusInternalServerError)
		app.ErrorLog.Printf("Failed to claim device: %v", err)
		return
	}

	// Invalidate any cached user device lists if Redis is available
	if app.Redis != nil {
		go func(userID uint) {
			if err := app.Redis.InvalidateUserDevicesCache(userID); err != nil {
				app.ErrorLog.Printf("Failed to invalidate user devices cache: %v", err)
			}
		}(userID)
	}

	// Return success
	app.writeJSON(w, http.StatusOK, map[string]string{
		"message":       "device claimed successfully",
		"device_id":     fmt.Sprintf("%d", device.ID),
		"serial_number": device.SerialNumber,
		"device_type":   device.DeviceType,
	})
}

// GetUnclaimedDevices returns a list of all auto-registered devices that haven't been claimed yet
func (app *Config) GetUnclaimedDevices(w http.ResponseWriter, r *http.Request) {
	// Extract user information from the token
	_, _, _, err := app.GetUserInfoFromToken(r)
	if err != nil {
		app.errorJSON(w, errors.New("unauthorized: invalid or missing token"), http.StatusUnauthorized)
		app.ErrorLog.Println(err)
		return
	}

	// Retrieve all unclaimed devices
	devices, err := app.Models.Device.GetUnclaimedDevices()
	if err != nil {
		app.errorJSON(w, errors.New("failed to retrieve unclaimed devices"), http.StatusInternalServerError)
		app.ErrorLog.Printf("Failed to retrieve unclaimed devices: %v", err)
		return
	}

	// If no unclaimed devices found, return empty array
	if devices == nil {
		devices = []*data.Device{}
	}

	// Return the list of unclaimed devices
	app.writeJSON(w, http.StatusOK, devices)
}

// GetUserDevices returns all devices belonging to the authenticated user
func (app *Config) GetUserDevices(w http.ResponseWriter, r *http.Request) {
	// Extract user information from the token
	userID, _, _, err := app.GetUserInfoFromToken(r)
	if err != nil {
		app.errorJSON(w, errors.New("unauthorized: invalid or missing token"), http.StatusUnauthorized)
		app.ErrorLog.Println(err)
		return
	}

	app.InfoLog.Printf("Retrieving devices for user ID: %d", userID)

	// Get devices directly from the database (not using Redis)
	devices, err := app.Models.Device.GetByUserID(userID)
	if err != nil {
		app.errorJSON(w, errors.New("failed to retrieve user's devices"), http.StatusInternalServerError)
		app.ErrorLog.Printf("Failed to retrieve devices for user %d: %v", userID, err)
		return
	}

	// Log details about each device
	app.InfoLog.Printf("Retrieved %d devices from database for user %d", len(devices), userID)
	for i, device := range devices {
		app.InfoLog.Printf("Device %d: ID=%d, SerialNumber=%s, Type=%s",
			i+1, device.ID, device.SerialNumber, device.DeviceType)
	}

	// If no devices found, return empty array
	if devices == nil {
		devices = []*data.Device{}
		app.InfoLog.Printf("No devices found for user %d", userID)
	}

	// Return list of devices
	app.writeJSON(w, http.StatusOK, devices)
}
