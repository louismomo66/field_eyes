// data/models.go
package data

import (
	"database/sql"
)

// Models is the wrapper for database models
type Models struct {
	User         UserInterface
	Device       DeviceInterface
	DeviceData   DeviceDataInterface
	Notification NotificationInterface
	Message      MessageModel
}

// New is used to create a new Models type
func New(db *sql.DB) Models {
	return Models{
		User:         NewUserRepository(db),
		Device:       &SQLDeviceAdaptor{DB: db},
		DeviceData:   &SQLDeviceDataAdaptor{DB: db},
		Notification: &SQLNotificationAdaptor{DB: db},
		Message:      MessageModel{DB: db},
	}
}

// UserModel wraps the database connection
type UserModel struct {
	DB *sql.DB
}

// DeviceModel wraps the database connection
type DeviceModel struct {
	DB *sql.DB
}

// DeviceDataModel wraps the database connection
type DeviceDataModel struct {
	DB *sql.DB
}

// NotificationModel wraps the database connection
type NotificationModel struct {
	DB *sql.DB
}

// MessageModel wraps the database connection
type MessageModel struct {
	DB *sql.DB
}

// SQLDeviceAdaptor adapts a SQL DB connection to implement DeviceInterface
type SQLDeviceAdaptor struct {
	DB *sql.DB
}

// GetAll retrieves all devices from the database
func (r *SQLDeviceAdaptor) GetAll() ([]*Device, error) {
	return nil, nil
}

// GetOne retrieves a device by its ID
func (r *SQLDeviceAdaptor) GetOne(id uint) (*Device, error) {
	return nil, nil
}

// AssignDevice assigns a device to a user
func (r *SQLDeviceAdaptor) AssignDevice(userID uint, device *Device) error {
	return nil
}

// GetByUserID retrieves all devices owned by a user
func (r *SQLDeviceAdaptor) GetByUserID(userID uint) ([]*Device, error) {
	return nil, nil
}

// CreateDevice creates a new device
func (r *SQLDeviceAdaptor) CreateDevice(device *Device) error {
	return nil
}

// GetBySerialNumber retrieves a device by its serial number
func (r *SQLDeviceAdaptor) GetBySerialNumber(serialNumber string) (*Device, error) {
	return nil, nil
}

// Update updates a device's information
func (r *SQLDeviceAdaptor) Update(device *Device) error {
	return nil
}

// GetUnclaimedDevices retrieves all unclaimed devices
func (r *SQLDeviceAdaptor) GetUnclaimedDevices() ([]*Device, error) {
	return nil, nil
}

// SQLDeviceDataAdaptor adapts a SQL DB connection to implement DeviceDataInterface
type SQLDeviceDataAdaptor struct {
	DB *sql.DB
}

// CreateLog creates a new device data log
func (r *SQLDeviceDataAdaptor) CreateLog(data *DeviceData) error {
	return nil
}

// GetLogsByDeviceID retrieves all logs for a device by its ID
func (r *SQLDeviceDataAdaptor) GetLogsByDeviceID(deviceID uint) ([]*DeviceData, error) {
	return nil, nil
}

// GetLogsBySerialNumber retrieves all logs for a device by its serial number
func (r *SQLDeviceDataAdaptor) GetLogsBySerialNumber(serialNumber string) ([]*DeviceData, error) {
	return nil, nil
}

// SQLNotificationAdaptor adapts a SQL DB connection to implement NotificationInterface
type SQLNotificationAdaptor struct {
	DB *sql.DB
}

// CreateNotification creates a new notification
func (r *SQLNotificationAdaptor) CreateNotification(notification *Notification) error {
	return nil
}

// GetUserNotifications retrieves all notifications for a user
func (r *SQLNotificationAdaptor) GetUserNotifications(userID uint) ([]*Notification, error) {
	return nil, nil
}

// GetUnreadNotifications retrieves all unread notifications for a user
func (r *SQLNotificationAdaptor) GetUnreadNotifications(userID uint) ([]*Notification, error) {
	return nil, nil
}

// MarkAsRead marks a notification as read
func (r *SQLNotificationAdaptor) MarkAsRead(id uint) error {
	return nil
}

// MarkAllAsRead marks all notifications for a user as read
func (r *SQLNotificationAdaptor) MarkAllAsRead(userID uint) error {
	return nil
}

// DeleteNotification deletes a notification
func (r *SQLNotificationAdaptor) DeleteNotification(id uint) error {
	return nil
}
