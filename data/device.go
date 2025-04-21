package data

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// Device represents the devices table in the database.
type Device struct {
	gorm.Model
	DeviceType   string         `gorm:"type:varchar(100);not null" json:"device_type"`
	SerialNumber string         `gorm:"type:varchar(100);uniqueIndex;not null" json:"serial_number"`
	UserID       uint           `gorm:"not null" json:"user_id"`
	User         User           `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// DeviceData represents the data logs for a device.
type DeviceData struct {
	gorm.Model
	DeviceID        uint      `gorm:"not null" json:"device_id"` // Foreign key to the Device table
	SerialNumber    string    `gorm:"not null" json:"serial_number"`
	Temperature     float64   `json:"temperature"`
	Humidity        float64   `json:"humidity"`
	Nitrogen        float64   `json:"nitrogen"`
	Phosphorous     float64   `json:"phosphorous"`
	Potassium       float64   `json:"potassium"`
	PH              float64   `json:"ph"`
	SoilMoisture    float64   `json:"soil_moisture"`
	SoilTemperature float64   `json:"soil_temperature"`
	SoilHumidity    float64   `json:"soil_humidity"`
	Longitude       float64   `json:"longitude"`
	Latitude        float64   `json:"latitude"`
	CreatedAt       time.Time `json:"created_at"`
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

// GetLogsBySerialNumber retrieves all logs for a specific device using its SerialNumber.
func (r *DeviceDataRepository) GetLogsBySerialNumber(serialNumber string) ([]*DeviceData, error) {
	var logs []*DeviceData
	result := r.db.Where("serial_number = ?", serialNumber).Order("created_at DESC").Find(&logs)
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
	result := r.db.Where("user_id = 0").Find(&devices)
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

// DeleteByID deletes a device by its ID
func (r *DeviceRepository) DeleteByID(id uint) error {
	return r.db.Delete(&Device{}, id).Error
}

// DeleteByDeviceID deletes all device data records for a specific device
func (r *DeviceDataRepository) DeleteByDeviceID(deviceID uint) error {
	return r.db.Where("device_id = ?", deviceID).Delete(&DeviceData{}).Error
}
