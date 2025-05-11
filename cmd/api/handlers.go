package main

import (
	"bytes"
	"errors"
	"field_eyes/data"
	"fmt"
	"io"
	"net/http"
	"strconv"

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

	// Get query parameters for filtering
	unreadOnly := r.URL.Query().Get("unread")
	deviceIDParam := r.URL.Query().Get("device_id")
	deviceNameParam := r.URL.Query().Get("device_name")

	app.InfoLog.Printf("GetNotifications called with deviceIDParam=%s, deviceNameParam=%s, unreadOnly=%s, userID=%d",
		deviceIDParam, deviceNameParam, unreadOnly, userID)

	var notifications []*data.Notification
	var fetchErr error

	// First check if device_name is provided (highest priority for device identification)
	if deviceNameParam != "" {
		// Directly use device_name to query by serial number
		app.InfoLog.Printf("Getting notifications by explicit device name: %s", deviceNameParam)

		if unreadOnly == "true" {
			// First get all notifications for the device
			allNotifications, err := app.Models.Notification.GetNotificationsByDeviceName(userID, deviceNameParam)
			if err == nil {
				// Then filter to unread only in memory
				notifications = []*data.Notification{}
				for _, notification := range allNotifications {
					if !notification.Read {
						notifications = append(notifications, notification)
					}
				}
				app.InfoLog.Printf("Found %d unread notifications of %d total for device name %s",
					len(notifications), len(allNotifications), deviceNameParam)
			} else {
				fetchErr = err
				app.ErrorLog.Printf("Error getting notifications by device name: %v", err)
			}
		} else {
			// Get all notifications for the device
			notifications, fetchErr = app.Models.Notification.GetNotificationsByDeviceName(userID, deviceNameParam)
			if fetchErr != nil {
				app.ErrorLog.Printf("Error getting notifications by device name: %v", fetchErr)
			} else {
				app.InfoLog.Printf("Found %d notifications for device name %s", len(notifications), deviceNameParam)
			}
		}
	} else if deviceIDParam != "" {
		// If device_id is provided, get device-specific notifications
		// First, try to parse it as a number
		deviceID, parseErr := strconv.ParseUint(deviceIDParam, 10, 64)
		if parseErr == nil {
			// It's a numeric ID
			app.InfoLog.Printf("Getting notifications by numeric device ID: %d", deviceID)
			if unreadOnly == "true" {
				allNotifications, err := app.Models.Notification.GetNotificationsByDeviceID(userID, uint(deviceID))
				if err == nil {
					notifications = []*data.Notification{}
					for _, notification := range allNotifications {
						if !notification.Read {
							notifications = append(notifications, notification)
						}
					}
				} else {
					fetchErr = err
					app.ErrorLog.Printf("Error getting notifications by device ID: %v", err)
				}
			} else {
				notifications, fetchErr = app.Models.Notification.GetNotificationsByDeviceID(userID, uint(deviceID))
				if fetchErr != nil {
					app.ErrorLog.Printf("Error getting notifications by device ID: %v", fetchErr)
				}
			}
		} else {
			// Not a numeric ID, try matching by device name (serial number)
			app.InfoLog.Printf("Getting notifications by device serial number: %s", deviceIDParam)

			// Use our new dedicated function to get notifications by device name
			if unreadOnly == "true" {
				// First get all notifications for the device
				allNotifications, err := app.Models.Notification.GetNotificationsByDeviceName(userID, deviceIDParam)
				if err == nil {
					// Then filter to unread only in memory
					notifications = []*data.Notification{}
					for _, notification := range allNotifications {
						if !notification.Read {
							notifications = append(notifications, notification)
						}
					}
					app.InfoLog.Printf("Found %d unread notifications of %d total for device %s",
						len(notifications), len(allNotifications), deviceIDParam)
				} else {
					fetchErr = err
					app.ErrorLog.Printf("Error getting notifications by device name: %v", err)
				}
			} else {
				// Get all notifications for the device
				notifications, fetchErr = app.Models.Notification.GetNotificationsByDeviceName(userID, deviceIDParam)
				if fetchErr != nil {
					app.ErrorLog.Printf("Error getting notifications by device name: %v", fetchErr)
				} else {
					app.InfoLog.Printf("Found %d notifications for device %s", len(notifications), deviceIDParam)
				}
			}
		}
	} else {
		// Get all notifications for the user
		app.InfoLog.Printf("Getting all notifications for user %d", userID)
		if unreadOnly == "true" {
			notifications, fetchErr = app.Models.Notification.GetUnreadNotifications(userID)
		} else {
			notifications, fetchErr = app.Models.Notification.GetUserNotifications(userID)
		}
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

// DeleteAllNotifications deletes all notifications for a user
func (app *Config) DeleteAllNotifications(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from JWT token
	userID, _, _, err := app.GetUserInfoFromToken(r)
	if err != nil {
		app.errorJSON(w, errors.New("unauthorized: invalid or missing token"), http.StatusUnauthorized)
		app.ErrorLog.Println(err)
		return
	}

	// Delete all notifications for the user
	if err := app.Models.Notification.DeleteAllNotifications(userID); err != nil {
		app.errorJSON(w, errors.New("failed to delete all notifications"), http.StatusInternalServerError)
		app.ErrorLog.Printf("Failed to delete all notifications: %v", err)
		return
	}

	// Return success
	app.writeJSON(w, http.StatusOK, map[string]string{
		"message": "All notifications deleted successfully",
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

	// Get device serial number from the request - this is now required
	deviceSerialParam := r.URL.Query().Get("serial_number")
	if deviceSerialParam == "" {
		app.errorJSON(w, errors.New("serial_number parameter is required"), http.StatusBadRequest)
		return
	}

	// Start the notification generation process in a goroutine
	go func() {
		app.InfoLog.Printf("Starting notification generation for device %s, requested by user %d",
			deviceSerialParam, userID)

		// Generate notifications for the specific device
		count, err := app.generateNotificationsForDevice(userID, deviceSerialParam)
		if err != nil {
			app.ErrorLog.Printf("Error generating notifications for device %s: %v", deviceSerialParam, err)
		} else {
			app.InfoLog.Printf("Generated %d notifications for device %s", count, deviceSerialParam)
		}
	}()

	// Return immediate response
	app.writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"message": "Notification generation process started for device " + deviceSerialParam,
		"status":  "processing",
	})
}

// Helper method to check rate limits (basic implementation)
// In production, you would use Redis to track API call frequency
func (app *Config) checkRateLimitForUser(userID uint) bool {
	// TODO: Implement proper rate limiting with Redis
	// For now, always return true to allow the call
	return true
}

// generateNotificationsForDevice generates notifications for a specific device
func (app *Config) generateNotificationsForDevice(userID uint, serialNumber string) (int, error) {
	// Get the device by serial number
	device, err := app.Models.Device.GetBySerialNumber(serialNumber)
	if err != nil || device == nil {
		return 0, fmt.Errorf("device not found with serial number: %s, error: %v", serialNumber, err)
	}

	// Security check: Verify that the device belongs to the user
	if device.UserID != userID {
		app.ErrorLog.Printf("Security warning: User %d attempted to access device %s which belongs to user %d",
			userID, serialNumber, device.UserID)
		return 0, fmt.Errorf("unauthorized: device does not belong to user")
	}

	// Get logs directly by serial number
	logs, err := app.Models.DeviceData.GetLogsBySerialNumber(serialNumber)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch logs for device %s: %v", serialNumber, err)
	}

	if len(logs) == 0 {
		app.InfoLog.Printf("No logs found for device %s", serialNumber)
		return 0, nil
	}

	// Process the latest log for this device
	latestLog := logs[0]
	app.InfoLog.Printf("Latest log for device %s: SoilMoisture=%f, SoilTemperature=%f, PH=%f",
		serialNumber, latestLog.SoilMoisture, latestLog.SoilTemperature, latestLog.PH)

	// Check conditions and create notifications
	count := app.checkConditionsAndCreateNotifications(device, latestLog, userID)
	return count, nil
}

// generateNotificationsForAllDevices generates notifications for all devices belonging to a user
func (app *Config) generateNotificationsForAllDevices(userID uint) (int, int, error) {
	// Get all user's devices
	devices, err := app.Models.Device.GetByUserID(userID)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to fetch devices: %v", err)
	}

	app.InfoLog.Printf("Checking %d devices for notification conditions", len(devices))
	totalNotifications := 0
	processedDevices := 0

	// Check each device for conditions that would trigger notifications
	for _, device := range devices {
		// Double-check that the device belongs to the user (defense in depth)
		if device.UserID != userID {
			app.ErrorLog.Printf("Security warning: Device ID=%d appears to be associated with wrong user", device.ID)
			continue
		}

		app.InfoLog.Printf("Processing device ID=%d, SerialNumber=%s", device.ID, device.SerialNumber)
		processedDevices++

		// First try to get logs by device ID
		logs, err := app.Models.DeviceData.GetLogsByDeviceID(device.ID)
		if err != nil || len(logs) == 0 {
			app.InfoLog.Printf("No logs found by device ID for %s, trying by serial number", device.SerialNumber)

			// If no logs found by ID, try by serial number
			logs, err = app.Models.DeviceData.GetLogsBySerialNumber(device.SerialNumber)
			if err != nil || len(logs) == 0 {
				app.InfoLog.Printf("No logs found for device %s: %v", device.SerialNumber, err)
				continue
			}
		}

		// Get most recent log
		latestLog := logs[0]
		app.InfoLog.Printf("Latest log for device %s: SoilMoisture=%f, SoilTemperature=%f, PH=%f",
			device.SerialNumber, latestLog.SoilMoisture, latestLog.SoilTemperature, latestLog.PH)

		// Check conditions and create notifications
		count := app.checkConditionsAndCreateNotifications(device, latestLog, userID)
		totalNotifications += count
	}

	return totalNotifications, processedDevices, nil
}

// Helper function to check conditions and create notifications
func (app *Config) checkConditionsAndCreateNotifications(device *data.Device, latestLog *data.DeviceData, userID uint) int {
	notificationsGenerated := 0

	// Create notification for low soil moisture (if soil moisture < 20%)
	if latestLog.SoilMoisture < 20 {
		app.InfoLog.Printf("Device %s has low soil moisture (%f%%)", device.SerialNumber, latestLog.SoilMoisture)

		message := fmt.Sprintf("Soil moisture is critically low (%f%%)", latestLog.SoilMoisture)
		notificationType := "warning"

		// Check if similar notification already exists
		exists, err := app.Models.Notification.HasSimilarNotification(
			device.ID, device.SerialNumber, userID, notificationType, message)

		if err != nil {
			app.ErrorLog.Printf("Error checking for existing notifications: %v", err)
		}

		if !exists {
			notification := data.Notification{
				Type:       notificationType,
				Message:    message,
				DeviceID:   device.ID,
				DeviceName: device.SerialNumber,
				UserID:     userID,
				Read:       false,
			}

			if err := app.Models.Notification.CreateNotification(&notification); err == nil {
				app.InfoLog.Printf("Created low moisture notification for device %s", device.SerialNumber)
				notificationsGenerated++
			} else {
				app.ErrorLog.Printf("Failed to create notification for device %s: %v", device.SerialNumber, err)
			}
		} else {
			app.InfoLog.Printf("Skipping duplicate low moisture notification for device %s", device.SerialNumber)
		}
	}

	// Check extreme temperature
	if latestLog.SoilTemperature > 35 || latestLog.SoilTemperature < 5 {
		app.InfoLog.Printf("Device %s has extreme soil temperature (%f°C)", device.SerialNumber, latestLog.SoilTemperature)

		message := fmt.Sprintf("Extreme soil temperature detected: %f°C", latestLog.SoilTemperature)
		notificationType := "alert"

		// Check if similar notification already exists
		exists, err := app.Models.Notification.HasSimilarNotification(
			device.ID, device.SerialNumber, userID, notificationType, message)

		if err != nil {
			app.ErrorLog.Printf("Error checking for existing notifications: %v", err)
		}

		if !exists {
			notification := data.Notification{
				Type:       notificationType,
				Message:    message,
				DeviceID:   device.ID,
				DeviceName: device.SerialNumber,
				UserID:     userID,
				Read:       false,
			}

			if err := app.Models.Notification.CreateNotification(&notification); err == nil {
				app.InfoLog.Printf("Created temperature notification for device %s", device.SerialNumber)
				notificationsGenerated++
			} else {
				app.ErrorLog.Printf("Failed to create notification for device %s: %v", device.SerialNumber, err)
			}
		} else {
			app.InfoLog.Printf("Skipping duplicate temperature notification for device %s", device.SerialNumber)
		}
	}

	// Check pH levels
	if latestLog.PH < 5.5 || latestLog.PH > 7.5 {
		app.InfoLog.Printf("Device %s has pH outside optimal range (%f)", device.SerialNumber, latestLog.PH)

		message := fmt.Sprintf("pH level outside optimal range: %f", latestLog.PH)
		notificationType := "info"

		// Check if similar notification already exists
		exists, err := app.Models.Notification.HasSimilarNotification(
			device.ID, device.SerialNumber, userID, notificationType, message)

		if err != nil {
			app.ErrorLog.Printf("Error checking for existing notifications: %v", err)
		}

		if !exists {
			notification := data.Notification{
				Type:       notificationType,
				Message:    message,
				DeviceID:   device.ID,
				DeviceName: device.SerialNumber,
				UserID:     userID,
				Read:       false,
			}

			if err := app.Models.Notification.CreateNotification(&notification); err == nil {
				app.InfoLog.Printf("Created pH notification for device %s", device.SerialNumber)
				notificationsGenerated++
			} else {
				app.ErrorLog.Printf("Failed to create notification for device %s: %v", device.SerialNumber, err)
			}
		} else {
			app.InfoLog.Printf("Skipping duplicate pH notification for device %s", device.SerialNumber)
		}
	}

	return notificationsGenerated
}

// HealthCheck is a simple health check endpoint that returns 200 OK if the service is running.
// This is used by cloud providers to check if the service is healthy.
func (app *Config) HealthCheck(w http.ResponseWriter, r *http.Request) {
	app.writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "field_eyes_api",
	})
}
