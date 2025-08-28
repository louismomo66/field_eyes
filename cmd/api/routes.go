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
		// Public endpoints (no auth required)
		r.Post("/signup", app.Signup)                  // Endpoint for user signup
		r.Post("/login", app.Login)                    // Endpoint for user login
		r.Post("/forgot-password", app.ForgotPassword) // Endpoint to request password reset
		r.Post("/reset-password", app.ResetPassword)   // Endpoint to reset password with OTP
		r.Post("/log-device-data", app.LogDeviceData)
		// Protected routes (require authentication)
		r.Group(func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return app.IsAuthenticated(next.ServeHTTP)
			})

			// Device-related endpoints
			r.Post("/register-device", app.RegisterDevice) // Endpoint to register a device
			// Endpoint to log device data
			r.Get("/get-device-logs", app.GetDeviceLogs)         // Endpoint to fetch device logs
			r.Get("/unclaimed-devices", app.GetUnclaimedDevices) // Endpoint to fetch unclaimed devices
			r.Post("/claim-device", app.ClaimDevice)             // Endpoint to claim a device
			r.Get("/user-devices", app.GetUserDevices)           // Endpoint to fetch user's devices
			r.Get("/latest-device-log", app.GetLatestDeviceLog)  // Endpoint to fetch only the latest log for a device
			r.Delete("/delete-device", app.DeleteDevice)         // Endpoint to delete a device by serial number
			r.Put("/update-device-name", app.UpdateDeviceName)   // New endpoint for updating device name

			// Analysis endpoints
			r.Get("/analyze-device", app.AnalyzeDeviceData)                       // Endpoint for ML analysis of device data
			r.Post("/reports/basic-soil-analysis", app.GenerateBasicSoilAnalysis) // Endpoint for basic soil analysis report

			// Notification endpoints
			r.Get("/notifications", app.GetNotifications)                      // Get all notifications for a user
			r.Post("/notifications", app.CreateNotification)                   // Create a new notification
			r.Put("/notifications/read", app.MarkNotificationAsRead)           // Mark a notification as read
			r.Put("/notifications/read-all", app.MarkAllNotificationsAsRead)   // Mark all notifications as read
			r.Delete("/notifications", app.DeleteNotification)                 // Delete a notification
			r.Delete("/notifications/delete-all", app.DeleteAllNotifications)  // Delete all notifications for a user
			r.Post("/notifications/generate", app.GenerateDeviceNotifications) // Generate notifications from device data
			r.Get("/devices/notifications", app.GenerateDeviceNotifications)   // Generate notifications for a specific device by serial_number

			// Admin-only routes
			r.Group(func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return app.IsAdmin(next.ServeHTTP)
				})

				r.Get("/admin/devices", app.GetAllDevicesForAdmin)                                  // Get all devices for admin
				r.Get("/admin/device-logs", app.GetDeviceLogsForAdmin)                              // Get device logs for admin
				r.Get("/admin/latest-device-log", app.GetLatestDeviceLogForAdmin)                   // Get latest device log for admin
				r.Get("/admin/download-device-data", app.DownloadDeviceData)                        // Download device data as CSV
				r.Post("/admin/reports/basic-soil-analysis", app.GenerateBasicSoilAnalysisForAdmin) // Generate basic soil analysis for admin
			})
		})
	})
	return mux
}
