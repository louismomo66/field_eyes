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

	// Health check endpoint for cloud providers
	mux.Get("/health", app.HealthCheck)

	mux.Route("/api", func(r chi.Router) {
		// User account related endpoints
		r.Post("/signup", app.Signup)                  // Endpoint for user signup
		r.Post("/login", app.Login)                    // Endpoint for user login
		r.Post("/forgot-password", app.ForgotPassword) // Endpoint to request password reset
		r.Post("/reset-password", app.ResetPassword)   // Endpoint to reset password with OTP

		// Device-related endpoints
		r.Post("/register-device", app.RegisterDevice)       // Endpoint to register a device
		r.Post("/log-device-data", app.LogDeviceData)        // Endpoint to log device data
		r.Get("/get-device-logs", app.GetDeviceLogs)         // Endpoint to fetch device logs
		r.Get("/unclaimed-devices", app.GetUnclaimedDevices) // Endpoint to fetch unclaimed devices
		r.Post("/claim-device", app.ClaimDevice)             // Endpoint to claim a device
		r.Get("/user-devices", app.GetUserDevices)           // Endpoint to fetch user's devices

		// Analysis endpoints
		r.Get("/analyze-device", app.AnalyzeDeviceData) // Endpoint for ML analysis of device data

		// Notification endpoints
		r.Get("/notifications", app.GetNotifications)                      // Get all notifications for a user
		r.Post("/notifications", app.CreateNotification)                   // Create a new notification
		r.Put("/notifications/read", app.MarkNotificationAsRead)           // Mark a notification as read
		r.Put("/notifications/read-all", app.MarkAllNotificationsAsRead)   // Mark all notifications as read
		r.Delete("/notifications", app.DeleteNotification)                 // Delete a notification
		r.Post("/notifications/generate", app.GenerateDeviceNotifications) // Generate notifications from device data
	})
	return mux
}
