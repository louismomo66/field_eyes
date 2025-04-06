package main

import (
	"net/http"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
)

func (app *Config) routes() http.Handler {
	mux := chi.NewRouter()
	//set up middleware
	mux.Use(middleware.Recoverer)
	mux.Use(app.EnableCORS)
	mux.Route("/api", func(r chi.Router) {
		// User account related endpoints
		r.Post("/signup", app.Signup)                  // Endpoint for user signup
		r.Post("/login", app.Login)                    // Endpoint for user login
		r.Post("/forgot-password", app.ForgotPassword) // Endpoint to request password reset
		r.Post("/reset-password", app.ResetPassword)   // Endpoint to reset password with OTP

		// Device-related endpoints
		r.Post("/register-device", app.RegisterDevice) // Endpoint to register a device
		r.Post("/log-device-data", app.LogDeviceData)  // Endpoint to log device data
		r.Get("/get-device-logs", app.GetDeviceLogs)   // Endpoint to fetch device logs

		// Analysis endpoints
		r.Get("/analyze-device", app.AnalyzeDeviceData) // Endpoint for ML analysis of device data
	})
	return mux
}
