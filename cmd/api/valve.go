package main

import (
	"errors"
	"field_eyes/data"
	"net/http"
	"os"
)

// ---- Valve scheduling ----
//
// The schedule and the manual override live in one config per valve device. Because
// the device is battery powered it sleeps most of the time, so the config is published
// as a retained MQTT message: the broker hands it over the moment the device next
// connects, and the device then runs the schedule on its own clock.

func (app *Config) valveTopicRoot() string {
	if app.MQTT != nil && app.MQTT.topicRoot != "" {
		return app.MQTT.topicRoot
	}
	if root := os.Getenv("MQTT_TOPIC_ROOT"); root != "" {
		return root
	}
	return "field_eyes/devices"
}

func (app *Config) valveConfigTopic(serialNumber string) string {
	return app.valveTopicRoot() + "/" + serialNumber + "/valve/config"
}

// publishValveConfig sends the config to the device as a retained message.
func (app *Config) publishValveConfig(cfg *data.ValveConfig) error {
	if app.MQTT == nil || !app.MQTT.IsConnected() {
		return errors.New("MQTT broker not connected")
	}
	payload, err := cfg.DevicePayload()
	if err != nil {
		return err
	}
	return app.MQTT.PublishRetained(app.valveConfigTopic(cfg.SerialNumber), payload)
}

// authorizeValve checks that the caller is signed in and may manage this valve: an
// admin, or the owner of the device. It writes the error response itself.
func (app *Config) authorizeValve(w http.ResponseWriter, r *http.Request, serialNumber string) (uint, bool) {
	userID, _, _, err := app.GetUserInfoFromToken(r)
	if err != nil {
		app.errorJSON(w, errors.New("unauthorized"), http.StatusUnauthorized)
		return 0, false
	}
	if serialNumber == "" {
		app.errorJSON(w, errors.New("serial_number is required"), http.StatusBadRequest)
		return 0, false
	}

	cd, err := app.Models.ControlDevice.GetBySerialNumber(serialNumber)
	if err != nil {
		app.errorJSON(w, errors.New("failed to look up the control device"), http.StatusInternalServerError)
		app.ErrorLog.Println("valve: control device lookup failed:", err)
		return 0, false
	}
	if cd == nil || cd.Kind != data.ControlKindValve {
		app.errorJSON(w, errors.New("this device is not a valve control device"), http.StatusNotFound)
		return 0, false
	}

	user, err := app.Models.User.GetOne(userID)
	if err != nil || user == nil {
		app.errorJSON(w, errors.New("unauthorized"), http.StatusUnauthorized)
		return 0, false
	}
	if user.Role != "admin" {
		device, err := app.Models.Device.GetBySerialNumber(serialNumber)
		if err != nil || device == nil || device.UserID != userID {
			app.errorJSON(w, errors.New("you do not have access to this device"), http.StatusForbidden)
			return 0, false
		}
	}
	return userID, true
}

// GetValveConfig returns the valve's schedule, manual override and what the device last reported.
func (app *Config) GetValveConfig(w http.ResponseWriter, r *http.Request) {
	serialNumber := r.URL.Query().Get("serial_number")
	if _, ok := app.authorizeValve(w, r, serialNumber); !ok {
		return
	}

	cfg, err := app.Models.ValveConfig.Get(serialNumber)
	if err != nil {
		app.errorJSON(w, errors.New("failed to load the valve configuration"), http.StatusInternalServerError)
		app.ErrorLog.Println("valve: load failed:", err)
		return
	}
	if cfg == nil {
		cfg = &data.ValveConfig{SerialNumber: serialNumber, Mode: data.ValveModeAuto}
	}
	if cfg.Schedule == nil {
		cfg.Schedule = []data.ScheduleEntry{}
	}
	app.writeJSON(w, http.StatusOK, cfg)
}

// SaveValveConfig stores a new schedule / manual override and sends it to the device.
func (app *Config) SaveValveConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SerialNumber string               `json:"serial_number"`
		Mode         string               `json:"mode"`
		UTCOffsetMin int                  `json:"utc_offset_min"`
		Schedule     []data.ScheduleEntry `json:"schedule"`
	}
	if err := app.ReadJSON(w, r, &req); err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		return
	}
	userID, ok := app.authorizeValve(w, r, req.SerialNumber)
	if !ok {
		return
	}

	cfg := &data.ValveConfig{
		SerialNumber: req.SerialNumber,
		Mode:         req.Mode,
		UTCOffsetMin: req.UTCOffsetMin,
		Schedule:     req.Schedule,
		UpdatedByID:  userID,
	}
	if err := cfg.Validate(); err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		return
	}
	if err := app.Models.ValveConfig.Save(cfg); err != nil {
		app.errorJSON(w, errors.New("failed to save the valve configuration"), http.StatusInternalServerError)
		app.ErrorLog.Println("valve: save failed:", err)
		return
	}

	if err := app.publishValveConfig(cfg); err != nil {
		// Saved, but the device cannot be told yet; it is sent again when the server restarts
		app.ErrorLog.Printf("valve: saved config v%d for %s but could not publish it: %v", cfg.Version, cfg.SerialNumber, err)
		app.errorJSON(w, errors.New("saved, but the message broker could not be reached, so the device has not been told yet - please save again in a moment"), http.StatusServiceUnavailable)
		return
	}

	app.InfoLog.Printf("User %d saved valve config v%d for %s (mode %s, %d schedule entries)", userID, cfg.Version, cfg.SerialNumber, cfg.Mode, len(cfg.Schedule))
	app.writeJSON(w, http.StatusOK, cfg)
}

// setUpValveConfig gives a newly registered valve a safe default (follow an empty
// schedule, i.e. stay closed) and sends it to the device.
func (app *Config) setUpValveConfig(serialNumber string, userID uint) {
	cfg, err := app.Models.ValveConfig.Get(serialNumber)
	if err != nil {
		app.ErrorLog.Println("valve: could not check for an existing config:", err)
		return
	}
	if cfg == nil {
		cfg = &data.ValveConfig{SerialNumber: serialNumber, Mode: data.ValveModeAuto, Schedule: []data.ScheduleEntry{}, UpdatedByID: userID}
		if err := app.Models.ValveConfig.Save(cfg); err != nil {
			app.ErrorLog.Println("valve: could not create the default config:", err)
			return
		}
	}
	if err := app.publishValveConfig(cfg); err != nil {
		app.ErrorLog.Printf("valve: could not publish the config for %s: %v", serialNumber, err)
	}
}

// retireValveConfig is used when a device stops being a valve control device: it
// tells the device to stop following any schedule, then forgets the config.
func (app *Config) retireValveConfig(serialNumber string) {
	cfg, err := app.Models.ValveConfig.Get(serialNumber)
	if err != nil || cfg == nil {
		return
	}
	cfg.Mode = data.ValveModeAuto
	cfg.Schedule = []data.ScheduleEntry{}
	if err := app.Models.ValveConfig.Save(cfg); err == nil {
		if err := app.publishValveConfig(cfg); err != nil {
			app.ErrorLog.Printf("valve: could not publish the cleared config for %s: %v", serialNumber, err)
		}
	}
	if err := app.Models.ValveConfig.Delete(serialNumber); err != nil {
		app.ErrorLog.Println("valve: could not delete the config:", err)
	}
}

// republishValveConfigs sends every valve's config to the broker again. Run at start-up,
// so a broker that lost its retained messages does not leave devices without a schedule.
func (app *Config) republishValveConfigs() {
	controls, err := app.Models.ControlDevice.GetAll()
	if err != nil {
		app.ErrorLog.Println("valve: could not list control devices:", err)
		return
	}
	sent := 0
	for _, cd := range controls {
		if cd.Kind != data.ControlKindValve {
			continue
		}
		cfg, err := app.Models.ValveConfig.Get(cd.SerialNumber)
		if err != nil || cfg == nil {
			continue
		}
		if err := app.publishValveConfig(cfg); err != nil {
			app.ErrorLog.Printf("valve: could not republish the config for %s: %v", cd.SerialNumber, err)
			continue
		}
		sent++
	}
	if sent > 0 {
		app.InfoLog.Printf("Republished %d valve config(s) to the broker", sent)
	}
}
