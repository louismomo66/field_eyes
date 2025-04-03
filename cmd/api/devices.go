package main

import (
	"errors"
	"field_eyes/data"
	"net/http"
)

func (app *Config) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	var device data.Device
	if err := app.errorJSON(w, app.ReadJSON(w, r, &device)); err != nil {
		app.ErrorLog.Println(err)
		return
	}
	if device.DeviceType == "" || device.SerialNumber == "" {
		app.ErrorLog.Println("fill all fields")
		app.errorJSON(w, errors.New("fill all fields"), http.StatusBadRequest)
		return

	}

	err := app.Models.Device.AssignDevice(device.UserID, &device)
	if err != nil {
		if err.Error() == "device with this serial number already exists" {
			app.ErrorLog.Println("device with this serial number already eists")
			app.errorJSON(w, errors.New("device with this serial number already exists"), http.StatusBadRequest)
		} else {
			app.ErrorLog.Println(err)
			app.errorJSON(w, err, http.StatusBadRequest)
		}
		return
	}
	app.writeJSON(w, http.StatusCreated, "device created successfully")

}
