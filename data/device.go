package data

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Device represents the devices table in the database.
type Device struct {
	gorm.Model
	DeviceType   string         `gorm:"type:varchar(100);not null" json:"device_type"`
	SerialNumber string         `gorm:"type:varchar(100);uniqueIndex;not null" json:"serial_number"`
	Name         string         `gorm:"type:varchar(100)" json:"name"`
	UserID       uint           `json:"user_id"`
	User         User           `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// DeviceData represents the data logs for a device.
type DeviceData struct {
	gorm.Model
	DeviceID               uint      `gorm:"index:idx_device_created,priority:1;not null" json:"device_id"` // Foreign key to the Device table
	SerialNumber           string    `gorm:"index:idx_serial_created,priority:1;not null" json:"serial_number"`
	Temperature            float64   `json:"temperature"`
	Humidity               float64   `json:"humidity"`
	Nitrogen               float64   `json:"nitrogen"`
	Phosphorous            float64   `json:"phosphorous"`
	Potassium              float64   `json:"potassium"`
	PH                     float64   `json:"ph"`
	SoilMoisture           float64   `json:"soil_moisture"`
	SoilTemperature        float64   `json:"soil_temperature"`
	SoilHumidity           float64   `json:"soil_humidity"`
	ElectricalConductivity float64   `json:"electrical_conductivity"`
	Longitude              float64   `json:"longitude"`
	Latitude               float64   `json:"latitude"`
	CreatedAt              time.Time `gorm:"index:idx_device_created,priority:2;index:idx_serial_created,priority:2" json:"created_at"`
}

// Notification represents a notification in the database
type Notification struct {
	gorm.Model
	Type       string         `gorm:"type:varchar(50);not null" json:"type"` // info, warning, alert, success
	Message    string         `gorm:"type:text;not null" json:"message"`
	DeviceID   uint           `json:"device_id"`
	DeviceName string         `json:"device_name"`
	Read       bool           `gorm:"default:false" json:"read"`
	UserID     uint           `gorm:"not null" json:"user_id"`
	User       User           `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Notification) TableName() string {
	return "notifications"
}

// DeviceRepository implements DeviceInterface using GORM.
type DeviceRepository struct {
	db *gorm.DB
}

// NewDeviceRepository creates a new instance of DeviceRepository.
func NewDeviceRepository(db *gorm.DB) DeviceInterface {
	return &DeviceRepository{db: db}
}

// DeviceDataRepository implements DeviceDataInterface using GORM.
type DeviceDataRepository struct {
	db *gorm.DB
}

// NewDeviceDataRepository creates a new instance of DeviceDataRepository.
func NewDeviceDataRepository(db *gorm.DB) DeviceDataInterface {
	return &DeviceDataRepository{db: db}
}

// NotificationRepository implements NotificationInterface using GORM
type NotificationRepository struct {
	db *gorm.DB
}

// NewNotificationRepository creates a new instance of NotificationRepository
func NewNotificationRepository(db *gorm.DB) NotificationInterface {
	return &NotificationRepository{db: db}
}

// GetAll retrieves all devices from the database.
func (r *DeviceRepository) GetAll() ([]*Device, error) {
	var devices []*Device
	result := r.db.Find(&devices)
	return devices, result.Error
}

// GetOne retrieves a device by its ID.
func (r *DeviceRepository) GetOne(id uint) (*Device, error) {
	var device Device
	result := r.db.First(&device, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &device, result.Error
}

// AssignDevice assigns a device to a user.
// It ensures that each device is uniquely assigned and handles the assignment logic.
func (r *DeviceRepository) AssignDevice(userID uint, device *Device) error {
	// Start a transaction to ensure atomicity
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Check if the user exists
		var user User
		if err := tx.First(&user, userID).Error; err != nil {
			return err
		}

		// Check if the device with the same serial number already exists
		var existingDevice Device
		if err := tx.Where("serial_number = ?", device.SerialNumber).First(&existingDevice).Error; err == nil {
			return errors.New("device with this serial number already exists")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// Assign the device to the user
		device.UserID = userID
		if err := tx.Create(device).Error; err != nil {
			return err
		}

		return nil
	})
}

func (r *DeviceRepository) GetByUserID(userID uint) ([]*Device, error) {
	var devices []*Device
	result := r.db.Where("user_id = ?", userID).Find(&devices)
	return devices, result.Error
}

func (r *DeviceRepository) CreateDevice(device *Device) error {
	return r.db.Create(device).Error
}

func (r *DeviceRepository) GetBySerialNumber(serialNumber string) (*Device, error) {
	var device Device
	result := r.db.Where("serial_number = ?", serialNumber).First(&device)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &device, result.Error
}

// CreateLog creates a new log entry for a device.
func (r *DeviceDataRepository) CreateLog(data *DeviceData) error {
	return r.db.Create(data).Error
}

// GetLogsByDeviceID retrieves all logs for a specific device using its DeviceID.
func (r *DeviceDataRepository) GetLogsByDeviceID(deviceID uint) ([]*DeviceData, error) {
	var logs []*DeviceData
	result := r.db.Where("device_id = ?", deviceID).Order("created_at DESC").Find(&logs)
	return logs, result.Error
}

// GetLogsByDeviceIDWithDateRange retrieves logs for a specific device within a date range
func (r *DeviceDataRepository) GetLogsByDeviceIDWithDateRange(deviceID uint, startDate, endDate time.Time) ([]*DeviceData, error) {
	var logs []*DeviceData
	result := r.db.Where("device_id = ? AND created_at >= ? AND created_at <= ?", deviceID, startDate, endDate).Order("created_at DESC").Find(&logs)
	return logs, result.Error
}

// GetLogsBySerialNumber retrieves all logs for a specific device using its SerialNumber.
func (r *DeviceDataRepository) GetLogsBySerialNumber(serialNumber string) ([]*DeviceData, error) {
	var logs []*DeviceData
	result := r.db.Where("serial_number = ?", serialNumber).Order("created_at DESC").Find(&logs)
	return logs, result.Error
}

// GetDeviceDataForDownload retrieves device data for download with optional date range filtering
func (r *DeviceDataRepository) GetDeviceDataForDownload(deviceID uint, startDate, endDate time.Time) ([]*DeviceData, error) {
	var logs []*DeviceData

	// If both dates are zero, get all data for the device
	if startDate.IsZero() && endDate.IsZero() {
		result := r.db.Where("device_id = ?", deviceID).Order("created_at ASC").Find(&logs)
		return logs, result.Error
	}

	// If only start date is provided, get data from start date to now
	if !startDate.IsZero() && endDate.IsZero() {
		result := r.db.Where("device_id = ? AND created_at >= ?", deviceID, startDate).Order("created_at ASC").Find(&logs)
		return logs, result.Error
	}

	// If only end date is provided, get data from beginning to end date
	if startDate.IsZero() && !endDate.IsZero() {
		result := r.db.Where("device_id = ? AND created_at <= ?", deviceID, endDate).Order("created_at ASC").Find(&logs)
		return logs, result.Error
	}

	// If both dates are provided, get data within the range
	result := r.db.Where("device_id = ? AND created_at >= ? AND created_at <= ?", deviceID, startDate, endDate).Order("created_at ASC").Find(&logs)
	return logs, result.Error
}

// Update updates an existing device in the database
func (r *DeviceRepository) Update(device *Device) error {
	result := r.db.Save(device)
	return result.Error
}

// GetUnclaimedDevices retrieves all devices that haven't been claimed by any user
func (r *DeviceRepository) GetUnclaimedDevices() ([]*Device, error) {
	var devices []*Device

	// Find the system user ID
	var systemUser User
	result := r.db.Where("email = ?", "system@fieldeyes.internal").First(&systemUser)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to find system user: %w", result.Error)
	}

	// Return devices assigned to the system user
	result = r.db.Where("user_id = ?", systemUser.ID).Find(&devices)
	return devices, result.Error
}

// CreateNotification creates a new notification
func (r *NotificationRepository) CreateNotification(notification *Notification) error {
	return r.db.Create(notification).Error
}

// GetUserNotifications retrieves all notifications for a user
func (r *NotificationRepository) GetUserNotifications(userID uint) ([]*Notification, error) {
	var notifications []*Notification
	result := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&notifications)
	return notifications, result.Error
}

// GetUnreadNotifications retrieves unread notifications for a user
func (r *NotificationRepository) GetUnreadNotifications(userID uint) ([]*Notification, error) {
	var notifications []*Notification
	result := r.db.Where("user_id = ? AND read = ?", userID, false).Order("created_at DESC").Find(&notifications)
	return notifications, result.Error
}

// MarkAsRead marks a notification as read
func (r *NotificationRepository) MarkAsRead(id uint) error {
	return r.db.Model(&Notification{}).Where("id = ?", id).Update("read", true).Error
}

// MarkAllAsRead marks all notifications for a user as read
func (r *NotificationRepository) MarkAllAsRead(userID uint) error {
	return r.db.Model(&Notification{}).Where("user_id = ?", userID).Update("read", true).Error
}

// DeleteNotification deletes a notification by its ID
func (r *NotificationRepository) DeleteNotification(id uint) error {
	return r.db.Delete(&Notification{}, id).Error
}

// DeleteAllNotifications deletes all notifications for a user
func (r *NotificationRepository) DeleteAllNotifications(userID uint) error {
	return r.db.Where("user_id = ?", userID).Delete(&Notification{}).Error
}

// DeleteByID deletes a device by its ID
func (r *DeviceRepository) DeleteByID(id uint) error {
	return r.db.Delete(&Device{}, id).Error
}

// DeleteByDeviceID deletes all device data records for a specific device
func (r *DeviceDataRepository) DeleteByDeviceID(deviceID uint) error {
	return r.db.Where("device_id = ?", deviceID).Delete(&DeviceData{}).Error
}

// GetNotificationsByDeviceID retrieves all notifications for a specific device
func (r *NotificationRepository) GetNotificationsByDeviceID(userID uint, deviceID uint) ([]*Notification, error) {
	var notifications []*Notification
	result := r.db.Where("user_id = ? AND device_id = ?", userID, deviceID).Order("created_at DESC").Find(&notifications)
	return notifications, result.Error
}

// GetNotificationsByDeviceName retrieves all notifications for a specific device by its name/serial number
func (r *NotificationRepository) GetNotificationsByDeviceName(userID uint, deviceName string) ([]*Notification, error) {
	var notifications []*Notification

	// Enable query logging for this query
	tx := r.db.Debug().Where("user_id = ? AND device_name = ?", userID, deviceName).Order("created_at DESC")

	// Execute the query
	result := tx.Find(&notifications)

	// Log the count of notifications found
	fmt.Printf("Found %d notifications for device name '%s' and user_id '%d'\n",
		len(notifications), deviceName, userID)

	return notifications, result.Error
}

// HasSimilarNotification checks if a similar notification was recently created
// to prevent duplicate notifications for the same condition
func (r *NotificationRepository) HasSimilarNotification(deviceID uint, deviceName string, userID uint, notificationType string, message string) (bool, error) {
	var count int64

	// For soil moisture notifications, use a longer time window of 6 hours
	var timeWindow time.Duration
	if strings.Contains(message, "Soil moisture is critically low") {
		timeWindow = 6 * time.Hour
	} else {
		timeWindow = 1 * time.Hour
	}

	timeAgo := time.Now().Add(-1 * timeWindow)

	// First check for exact message match
	result := r.db.Model(&Notification{}).
		Where("device_id = ? AND device_name = ? AND user_id = ? AND type = ? AND message = ? AND created_at > ?",
			deviceID, deviceName, userID, notificationType, message, timeAgo).
		Count(&count)

	if count > 0 || result.Error != nil {
		return count > 0, result.Error
	}

	// For soil moisture notifications, use a more specific pattern match
	if strings.Contains(message, "Soil moisture is critically low") {
		// Extract just the prefix to match any soil moisture notification regardless of exact percentage
		var similarCount int64
		result = r.db.Model(&Notification{}).
			Where("device_id = ? AND device_name = ? AND user_id = ? AND type = ? AND message LIKE ? AND created_at > ?",
				deviceID, deviceName, userID, notificationType, "Soil moisture is critically low%", timeAgo).
			Count(&similarCount)

		return similarCount > 0, result.Error
	}

	// For pH notifications
	if strings.Contains(message, "pH level outside optimal range") {
		var similarCount int64
		result = r.db.Model(&Notification{}).
			Where("device_id = ? AND device_name = ? AND user_id = ? AND type = ? AND message LIKE ? AND created_at > ?",
				deviceID, deviceName, userID, notificationType, "pH level outside optimal range%", timeAgo).
			Count(&similarCount)

		return similarCount > 0, result.Error
	}

	// For temperature notifications
	if strings.Contains(message, "Temperature is") {
		var similarCount int64
		result = r.db.Model(&Notification{}).
			Where("device_id = ? AND device_name = ? AND user_id = ? AND type = ? AND message LIKE ? AND created_at > ?",
				deviceID, deviceName, userID, notificationType, "Temperature is%", timeAgo).
			Count(&similarCount)

		return similarCount > 0, result.Error
	}

	// For other notification types, check if there's any notification of the same type
	// for the same device in the last hour
	var otherCount int64
	result = r.db.Model(&Notification{}).
		Where("device_id = ? AND device_name = ? AND user_id = ? AND type = ? AND created_at > ?",
			deviceID, deviceName, userID, notificationType, timeAgo).
		Count(&otherCount)

	return otherCount > 0, result.Error
}
