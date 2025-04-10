package main

import (
	"bytes"
	"errors"
	"field_eyes/data"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"gorm.io/gorm"
)

func (app *Config) Signup(w http.ResponseWriter, r *http.Request) {
	// Log the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}
	app.InfoLog.Printf("Received signup request body: %s", string(body))
	// Restore the body for further reading
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	var user data.User
	if err := app.ReadJSON(w, r, &user); err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}

	// Log the parsed user data
	app.InfoLog.Printf("Parsed user data: username=%s, email=%s, password=%s", user.Username, user.Email, user.TempPassword)

	// Validate required fields
	if user.Username == "" || user.Email == "" || user.TempPassword == "" {
		app.errorJSON(w, errors.New("username, email and password are required"), http.StatusBadRequest)
		app.ErrorLog.Println("username, email and password are empty")
		return
	}

	// Check if user exists
	existingUser, err := app.Models.User.GetByEmail(user.Email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		app.errorJSON(w, errors.New("database error"), http.StatusInternalServerError)
		app.ErrorLog.Println(err)
		return
	}

	// If user exists (not nil and has email)
	if existingUser != nil && existingUser.Email != "" {
		app.errorJSON(w, errors.New("user already exists"), http.StatusBadRequest)
		app.ErrorLog.Println("user already exists")
		return
	}

	// No need to hash password here, it's done in the Insert function
	id, err := app.Models.User.Insert(&user)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		app.ErrorLog.Println(err)
		return
	}

	// Return standardized response
	app.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message": fmt.Sprintf("User created successfully with id %d", id),
		"user_id": id,
	})
	app.InfoLog.Printf("User created successfully with id %d", id)
}

func (app *Config) Login(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := app.ReadJSON(w, r, &request); err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}

	// Validate required fields
	if request.Email == "" || request.Password == "" {
		app.errorJSON(w, errors.New("email and password are required"), http.StatusBadRequest)
		app.ErrorLog.Println("email and password are empty")
		return
	}

	user, err := app.Models.User.GetByEmail(request.Email)
	if err != nil {
		app.errorJSON(w, errors.New("invalid credentials"), http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}

	isMatch, err := app.Models.User.PasswordMatches(user, request.Password)
	if err != nil {
		app.errorJSON(w, errors.New("authentication error"), http.StatusInternalServerError)
		app.ErrorLog.Println(err)
		return
	}
	if !isMatch {
		app.errorJSON(w, errors.New("invalid credentials"), http.StatusBadRequest)
		app.ErrorLog.Println("Invalid password")
		return
	}

	token, err := app.GenerateJWT(*user)
	if err != nil {
		app.errorJSON(w, errors.New("failed to generate token"), http.StatusInternalServerError)
		app.ErrorLog.Println(err)
		return
	}

	// Create a user response without password
	userResponse := map[string]interface{}{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
		"role":     user.Role,
	}

	// Respond with the token and user
	app.writeJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user":  userResponse,
	})
}

// ForgotPassword handles the forgot password request by generating an OTP and sending it via email
func (app *Config) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	// Parse request to get email
	var request struct {
		Email string `json:"email"`
	}

	if err := app.ReadJSON(w, r, &request); err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}

	// Check if email is valid
	if request.Email == "" {
		app.errorJSON(w, errors.New("email is required"), http.StatusBadRequest)
		return
	}

	// Check if user exists
	user, err := app.Models.User.GetByEmail(request.Email)
	if err != nil || user == nil {
		// Don't reveal that the user doesn't exist for security
		app.writeJSON(w, http.StatusOK, map[string]string{
			"message": "If your email is registered, you will receive an OTP shortly",
		})
		app.InfoLog.Printf("Forgot password requested for non-existent email: %s", request.Email)
		return
	}

	// Generate OTP
	otp, err := app.Models.User.GenerateAndSaveOTP(request.Email)
	if err != nil {
		app.errorJSON(w, errors.New("failed to generate OTP"), http.StatusInternalServerError)
		app.ErrorLog.Printf("Failed to generate OTP: %v", err)
		return
	}

	// Send OTP via email in a goroutine
	app.SendEmailInBackground(request.Email, otp)

	// Respond to the user
	app.writeJSON(w, http.StatusOK, map[string]string{
		"message": "If your email is registered, you will receive an OTP shortly",
	})
	app.InfoLog.Printf("Forgot password OTP generated for email: %s", request.Email)
}

// ResetPassword handles the password reset using the OTP
func (app *Config) ResetPassword(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var request struct {
		Email       string `json:"email"`
		OTP         string `json:"otp"`
		NewPassword string `json:"new_password"`
	}

	if err := app.ReadJSON(w, r, &request); err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}

	// Validate request
	if request.Email == "" || request.OTP == "" || request.NewPassword == "" {
		app.errorJSON(w, errors.New("email, OTP, and new password are required"), http.StatusBadRequest)
		return
	}

	// Validate password strength
	if len(request.NewPassword) < 8 {
		app.errorJSON(w, errors.New("password must be at least 8 characters"), http.StatusBadRequest)
		return
	}

	// Try to reset password with OTP
	err := app.Models.User.ResetPasswordWithOTP(request.Email, request.OTP, request.NewPassword)
	if err != nil {
		app.errorJSON(w, errors.New("invalid or expired OTP"), http.StatusBadRequest)
		app.ErrorLog.Printf("Password reset failed: %v", err)
		return
	}

	// Return success
	app.writeJSON(w, http.StatusOK, map[string]string{
		"message": "Password has been reset successfully",
	})
	app.InfoLog.Printf("Password reset successful for email: %s", request.Email)
}

// GetNotifications returns all notifications for the authenticated user
func (app *Config) GetNotifications(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from JWT token
	userID, _, _, err := app.GetUserInfoFromToken(r)
	if err != nil {
		app.errorJSON(w, errors.New("unauthorized: invalid or missing token"), http.StatusUnauthorized)
		app.ErrorLog.Println(err)
		return
	}

	// Get query parameter for filtering unread notifications
	unreadOnly := r.URL.Query().Get("unread")

	var notifications []*data.Notification
	var fetchErr error

	// Fetch notifications based on filter
	if unreadOnly == "true" {
		notifications, fetchErr = app.Models.Notification.GetUnreadNotifications(userID)
	} else {
		notifications, fetchErr = app.Models.Notification.GetUserNotifications(userID)
	}

	if fetchErr != nil {
		app.errorJSON(w, errors.New("failed to fetch notifications"), http.StatusInternalServerError)
		app.ErrorLog.Printf("Failed to fetch notifications: %v", fetchErr)
		return
	}

	// Return notifications
	app.writeJSON(w, http.StatusOK, map[string]interface{}{
		"notifications": notifications,
		"count":         len(notifications),
	})
}

// CreateNotification creates a new notification
func (app *Config) CreateNotification(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from JWT token
	userID, _, _, err := app.GetUserInfoFromToken(r)
	if err != nil {
		app.errorJSON(w, errors.New("unauthorized: invalid or missing token"), http.StatusUnauthorized)
		app.ErrorLog.Println(err)
		return
	}

	// Parse request body
	var notificationRequest struct {
		Type       string `json:"type"`
		Message    string `json:"message"`
		DeviceID   uint   `json:"device_id"`
		DeviceName string `json:"device_name"`
	}

	if err := app.ReadJSON(w, r, &notificationRequest); err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}

	// Validate required fields
	if notificationRequest.Message == "" || notificationRequest.Type == "" {
		app.errorJSON(w, errors.New("message and type are required"), http.StatusBadRequest)
		return
	}

	// Create notification
	notification := data.Notification{
		Type:       notificationRequest.Type,
		Message:    notificationRequest.Message,
		DeviceID:   notificationRequest.DeviceID,
		DeviceName: notificationRequest.DeviceName,
		UserID:     userID,
		Read:       false,
	}

	if err := app.Models.Notification.CreateNotification(&notification); err != nil {
		app.errorJSON(w, errors.New("failed to create notification"), http.StatusInternalServerError)
		app.ErrorLog.Printf("Failed to create notification: %v", err)
		return
	}

	// Return success
	app.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message":      "Notification created successfully",
		"notification": notification,
	})
}

// MarkNotificationAsRead marks a notification as read
func (app *Config) MarkNotificationAsRead(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from JWT token
	_, _, _, err := app.GetUserInfoFromToken(r)
	if err != nil {
		app.errorJSON(w, errors.New("unauthorized: invalid or missing token"), http.StatusUnauthorized)
		app.ErrorLog.Println(err)
		return
	}

	// Get notification ID from request
	idParam := r.URL.Query().Get("id")
	if idParam == "" {
		app.errorJSON(w, errors.New("notification ID is required"), http.StatusBadRequest)
		return
	}

	// Convert ID to uint
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		app.errorJSON(w, errors.New("invalid notification ID"), http.StatusBadRequest)
		return
	}

	// Mark as read
	if err := app.Models.Notification.MarkAsRead(uint(id)); err != nil {
		app.errorJSON(w, errors.New("failed to mark notification as read"), http.StatusInternalServerError)
		app.ErrorLog.Printf("Failed to mark notification as read: %v", err)
		return
	}

	// Return success
	app.writeJSON(w, http.StatusOK, map[string]string{
		"message": "Notification marked as read",
	})
}

// MarkAllNotificationsAsRead marks all notifications for a user as read
func (app *Config) MarkAllNotificationsAsRead(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from JWT token
	userID, _, _, err := app.GetUserInfoFromToken(r)
	if err != nil {
		app.errorJSON(w, errors.New("unauthorized: invalid or missing token"), http.StatusUnauthorized)
		app.ErrorLog.Println(err)
		return
	}

	// Mark all as read
	if err := app.Models.Notification.MarkAllAsRead(userID); err != nil {
		app.errorJSON(w, errors.New("failed to mark all notifications as read"), http.StatusInternalServerError)
		app.ErrorLog.Printf("Failed to mark all notifications as read: %v", err)
		return
	}

	// Return success
	app.writeJSON(w, http.StatusOK, map[string]string{
		"message": "All notifications marked as read",
	})
}

// DeleteNotification deletes a notification
func (app *Config) DeleteNotification(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from JWT token
	_, _, _, err := app.GetUserInfoFromToken(r)
	if err != nil {
		app.errorJSON(w, errors.New("unauthorized: invalid or missing token"), http.StatusUnauthorized)
		app.ErrorLog.Println(err)
		return
	}

	// Get notification ID from request
	idParam := r.URL.Query().Get("id")
	if idParam == "" {
		app.errorJSON(w, errors.New("notification ID is required"), http.StatusBadRequest)
		return
	}

	// Convert ID to uint
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		app.errorJSON(w, errors.New("invalid notification ID"), http.StatusBadRequest)
		return
	}

	// Delete notification
	if err := app.Models.Notification.DeleteNotification(uint(id)); err != nil {
		app.errorJSON(w, errors.New("failed to delete notification"), http.StatusInternalServerError)
		app.ErrorLog.Printf("Failed to delete notification: %v", err)
		return
	}

	// Return success
	app.writeJSON(w, http.StatusOK, map[string]string{
		"message": "Notification deleted",
	})
}

// GenerateDeviceNotifications checks device data for conditions that should create notifications
func (app *Config) GenerateDeviceNotifications(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from JWT token
	userID, _, _, err := app.GetUserInfoFromToken(r)
	if err != nil {
		app.errorJSON(w, errors.New("unauthorized: invalid or missing token"), http.StatusUnauthorized)
		app.ErrorLog.Println(err)
		return
	}

	// Get user's devices
	devices, err := app.Models.Device.GetByUserID(userID)
	if err != nil {
		app.errorJSON(w, errors.New("failed to fetch devices"), http.StatusInternalServerError)
		app.ErrorLog.Printf("Failed to fetch devices: %v", err)
		return
	}

	notificationsGenerated := 0

	// Check each device for conditions that would trigger notifications
	for _, device := range devices {
		// Get latest log for device
		logs, err := app.Models.DeviceData.GetLogsByDeviceID(device.ID)
		if err != nil || len(logs) == 0 {
			continue
		}

		// Get most recent log
		latestLog := logs[0]

		// Create notification for low battery (if battery < 20%)
		if latestLog.SoilMoisture < 20 {
			notification := data.Notification{
				Type:       "warning",
				Message:    fmt.Sprintf("Soil moisture is critically low (%f%%)", latestLog.SoilMoisture),
				DeviceID:   device.ID,
				DeviceName: device.SerialNumber,
				UserID:     userID,
				Read:       false,
			}

			if err := app.Models.Notification.CreateNotification(&notification); err == nil {
				notificationsGenerated++
			}
		}

		// Check extreme temperature
		if latestLog.SoilTemperature > 35 || latestLog.SoilTemperature < 5 {
			notification := data.Notification{
				Type:       "alert",
				Message:    fmt.Sprintf("Extreme soil temperature detected: %f°C", latestLog.SoilTemperature),
				DeviceID:   device.ID,
				DeviceName: device.SerialNumber,
				UserID:     userID,
				Read:       false,
			}

			if err := app.Models.Notification.CreateNotification(&notification); err == nil {
				notificationsGenerated++
			}
		}

		// Check pH levels
		if latestLog.PH < 5.5 || latestLog.PH > 7.5 {
			notification := data.Notification{
				Type:       "info",
				Message:    fmt.Sprintf("pH level outside optimal range: %f", latestLog.PH),
				DeviceID:   device.ID,
				DeviceName: device.SerialNumber,
				UserID:     userID,
				Read:       false,
			}

			if err := app.Models.Notification.CreateNotification(&notification); err == nil {
				notificationsGenerated++
			}
		}
	}

	// Return success with count of notifications generated
	app.writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":             "Device notifications generated",
		"notifications_count": notificationsGenerated,
		"devices_checked":     len(devices),
	})
}

// HealthCheck is a simple health check endpoint that returns 200 OK if the service is running.
// This is used by cloud providers to check if the service is healthy.
func (app *Config) HealthCheck(w http.ResponseWriter, r *http.Request) {
	// Log detailed information about the health check request
	app.InfoLog.Printf("Health check request received from %s", r.RemoteAddr)
	app.InfoLog.Printf("Server is running and responding on port %s", webPort)

	// Log relevant environment variables for debugging
	app.InfoLog.Printf("Environment: DB_HOST=%s", os.Getenv("DB_HOST"))
	app.InfoLog.Printf("Environment: DB_PORT=%s", os.Getenv("DB_PORT"))
	app.InfoLog.Printf("Environment: Using DSN=%s", os.Getenv("DSN"))

	// Check database connection if available
	var dbStatus string
	if app.DB != nil {
		sqlDB, err := app.DB.DB()
		if err != nil || sqlDB.Ping() != nil {
			dbStatus = "disconnected"
		} else {
			dbStatus = "connected"
		}
	} else {
		dbStatus = "not configured"
	}

	// Send response
	app.writeJSON(w, http.StatusOK, map[string]string{
		"status":    "ok",
		"service":   "field_eyes_api",
		"timestamp": time.Now().Format(time.RFC3339),
		"db_status": dbStatus,
		"port":      webPort,
	})
}
