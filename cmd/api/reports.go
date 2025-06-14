package main

import (
	"errors"
	"field_eyes/data"
	"net/http"
	"time"
)

type BasicSoilAnalysisReport struct {
	DeviceName  string              `json:"device_name"`
	Parameters  []ParameterAnalysis `json:"parameters"`
	GeneratedAt time.Time           `json:"generated_at"`
	StartDate   time.Time           `json:"start_date"`
	EndDate     time.Time           `json:"end_date"`
}

type ParameterAnalysis struct {
	Name     string  `json:"name"`
	Unit     string  `json:"unit"`
	IdealMin float64 `json:"ideal_min"`
	IdealMax float64 `json:"ideal_max"`
	Average  float64 `json:"average"`
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
	Rating   string  `json:"rating"`
	CEC      float64 `json:"cec,omitempty"` // Cation Exchange Capacity
}

func (app *Config) GenerateBasicSoilAnalysis(w http.ResponseWriter, r *http.Request) {
	// Extract user information from the token
	userID, _, _, err := app.GetUserInfoFromToken(r)
	if err != nil {
		app.ErrorLog.Printf("Authentication failed: %v", err)
		app.errorJSON(w, errors.New("unauthorized: invalid or missing token"), http.StatusUnauthorized)
		return
	}

	// Parse request parameters
	var request struct {
		SerialNumber string    `json:"serial_number"`
		StartDate    time.Time `json:"start_date"`
		EndDate      time.Time `json:"end_date"`
	}

	if err := app.ReadJSON(w, r, &request); err != nil {
		app.ErrorLog.Printf("Failed to parse request body: %v", err)
		app.errorJSON(w, err, http.StatusBadRequest)
		return
	}

	app.InfoLog.Printf("Processing report request for device %s, date range: %s to %s",
		request.SerialNumber, request.StartDate.Format(time.RFC3339), request.EndDate.Format(time.RFC3339))

	// Validate device ownership
	device, err := app.Models.Device.GetBySerialNumber(request.SerialNumber)
	if err != nil || device == nil {
		app.ErrorLog.Printf("Device not found: %s, error: %v", request.SerialNumber, err)
		app.errorJSON(w, errors.New("device not found"), http.StatusNotFound)
		return
	}
	if device.UserID != userID {
		app.ErrorLog.Printf("Unauthorized access: device %s belongs to user %d, but request from user %d",
			request.SerialNumber, device.UserID, userID)
		app.errorJSON(w, errors.New("unauthorized: device does not belong to the user"), http.StatusUnauthorized)
		return
	}

	// Get device logs for the specified period
	logs, err := app.Models.DeviceData.GetLogsBySerialNumber(request.SerialNumber)
	if err != nil {
		app.ErrorLog.Printf("Failed to retrieve device logs for %s: %v", request.SerialNumber, err)
		app.errorJSON(w, errors.New("failed to retrieve device logs"), http.StatusInternalServerError)
		return
	}

	app.InfoLog.Printf("Retrieved %d logs for device %s", len(logs), request.SerialNumber)

	// Filter logs by date range
	filteredLogs := make([]*data.DeviceData, 0)
	for _, log := range logs {
		if log.CreatedAt.After(request.StartDate) && log.CreatedAt.Before(request.EndDate) {
			filteredLogs = append(filteredLogs, log)
		}
	}

	app.InfoLog.Printf("Filtered to %d logs within date range", len(filteredLogs))

	if len(filteredLogs) == 0 {
		app.ErrorLog.Printf("No data available for device %s between %s and %s",
			request.SerialNumber, request.StartDate.Format(time.RFC3339), request.EndDate.Format(time.RFC3339))
		app.errorJSON(w, errors.New("no data available for the selected period"), http.StatusNotFound)
		return
	}

	// Calculate statistics for each parameter
	report := BasicSoilAnalysisReport{
		DeviceName:  device.SerialNumber, // Use serial number as name if no custom name set
		GeneratedAt: time.Now(),
		StartDate:   request.StartDate,
		EndDate:     request.EndDate,
		Parameters: []ParameterAnalysis{
			calculateParameterStats(filteredLogs, "pH", "pH", 6.0, 7.5),
			calculateParameterStats(filteredLogs, "Nitrogen", "mg/kg", 20, 40),
			calculateParameterStats(filteredLogs, "Phosphorous", "mg/kg", 20, 40),
			calculateParameterStats(filteredLogs, "Potassium", "mg/kg", 150, 250),
			calculateParameterStats(filteredLogs, "Soil Moisture", "%", 20, 60),
			calculateParameterStats(filteredLogs, "Electrical Conductivity", "µS/cm", 200, 800),
		},
	}

	// Calculate CEC based on available parameters
	calculateCEC(&report)

	app.InfoLog.Printf("Successfully generated report for device %s", request.SerialNumber)
	app.writeJSON(w, http.StatusOK, report)
}

func calculateParameterStats(logs []*data.DeviceData, paramName, unit string, idealMin, idealMax float64) ParameterAnalysis {
	var sum, min, max float64
	count := 0
	min = -1 // Use -1 as sentinel value

	for _, log := range logs {
		var value float64
		switch paramName {
		case "pH":
			value = log.PH
		case "Nitrogen":
			value = log.Nitrogen
		case "Phosphorous":
			value = log.Phosphorous
		case "Potassium":
			value = log.Potassium
		case "Soil Moisture":
			value = log.SoilMoisture
		case "Electrical Conductivity":
			value = log.ElectricalConductivity
		}

		if value != 0 { // Skip zero values as they might indicate missing data
			sum += value
			count++
			if min == -1 || value < min {
				min = value
			}
			if value > max {
				max = value
			}
		}
	}

	if count == 0 {
		return ParameterAnalysis{
			Name:     paramName,
			Unit:     unit,
			IdealMin: idealMin,
			IdealMax: idealMax,
		}
	}

	avg := sum / float64(count)
	rating := calculateRating(avg, idealMin, idealMax)

	return ParameterAnalysis{
		Name:     paramName,
		Unit:     unit,
		IdealMin: idealMin,
		IdealMax: idealMax,
		Average:  avg,
		Min:      min,
		Max:      max,
		Rating:   rating,
	}
}

func calculateRating(value, idealMin, idealMax float64) string {
	if value < idealMin*0.5 {
		return "Very Low"
	} else if value < idealMin {
		return "Low"
	} else if value >= idealMin && value <= idealMax {
		return "Optimum"
	} else if value <= idealMax*1.5 {
		return "High"
	} else {
		return "Very High"
	}
}

func calculateCEC(report *BasicSoilAnalysisReport) {
	// Find the parameters we need for CEC calculation
	var k float64
	for _, param := range report.Parameters {
		switch param.Name {
		case "Potassium":
			k = param.Average
		}
	}

	// Simple CEC calculation based on available parameters
	// Note: This is a simplified calculation as we don't have all parameters
	cec := (k * 0.00256) // Convert ppm to cmol/kg
	if cec > 0 {
		for i, param := range report.Parameters {
			if param.Name == "Potassium" {
				report.Parameters[i].CEC = cec
			}
		}
	}
}
