package main

import (
	"errors"
	"field_eyes/data"
	"net/http"
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
	var device data.Device
	if err := app.ReadJSON(w, r, &device); err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}

	// Validate required fields
	if device.DeviceType == "" || device.SerialNumber == "" {
		app.errorJSON(w, errors.New("device type and serial number are required"), http.StatusBadRequest)
		app.ErrorLog.Println("device type and serial number are required")
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
		if d.SerialNumber == device.SerialNumber {
			app.errorJSON(w, errors.New("user already has a device with this serial number"), http.StatusBadRequest)
			app.ErrorLog.Println("user already has a device with this serial number")
			return
		}
	}

	// Check if the device with the same serial number is already linked to another user
	existingDevice, err := app.Models.Device.GetBySerialNumber(device.SerialNumber)
	if err != nil {
		app.errorJSON(w, errors.New("failed to check device serial number"), http.StatusInternalServerError)
		app.ErrorLog.Println(err)
		return
	}
	if existingDevice != nil && existingDevice.UserID != userID {
		app.errorJSON(w, errors.New("device with this serial number is already linked to another user"), http.StatusBadRequest)
		app.ErrorLog.Println("device with this serial number is already linked to another user")
		return
	}

	// Assign the device to the user
	device.UserID = userID
	err = app.Models.Device.AssignDevice(userID, &device)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		app.ErrorLog.Println(err)
		return
	}

	// Respond with success
	app.writeJSON(w, http.StatusCreated, map[string]string{"message": "device registered successfully"})
}
func (app *Config) LogDeviceData(w http.ResponseWriter, r *http.Request) {
	// Parse the request body into a DeviceData struct
	var logEntry data.DeviceData
	if err := app.ReadJSON(w, r, &logEntry); err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}

	// Validate that the serial number exists
	device, err := app.Models.Device.GetBySerialNumber(logEntry.SerialNumber)
	if err != nil {
		app.errorJSON(w, errors.New("device not found"), http.StatusNotFound)
		app.ErrorLog.Println("device not found:", err)
		return
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

	// Respond with success
	app.writeJSON(w, http.StatusCreated, map[string]string{"message": "device data logged successfully"})
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

	// Retrieve the logs for the device
	logs, err := app.Models.DeviceData.GetLogsByDeviceID(device.ID)
	if err != nil {
		app.errorJSON(w, errors.New("failed to retrieve device logs"), http.StatusInternalServerError)
		app.ErrorLog.Println("failed to retrieve device logs:", err)
		return
	}

	// Respond with the logs
	app.writeJSON(w, http.StatusOK, logs)
}
