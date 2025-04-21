package data

// UserInterface defines the methods that must be implemented by a User repository
type UserInterface interface {
	GetAll() ([]*User, error)
	GetByEmail(email string) (*User, error)
	GetOne(id uint) (*User, error)
	Insert(user *User) (uint, error) // Changed return type to match implementation
	Update(user *User) error
	Delete(user *User) error
	DeleteByID(id uint) error
	ResetPassword(userID uint, newPassword string) error
	PasswordMatches(user *User, plainText string) (bool, error)
	// OTP Related methods
	GenerateAndSaveOTP(email string) (string, error)
	VerifyOTP(email, otp string) (bool, error)
	ResetPasswordWithOTP(email, otp, newPassword string) error
}

// DeviceInterface defines the methods for Device operations.
type DeviceInterface interface {
	GetAll() ([]*Device, error)
	GetOne(id uint) (*Device, error)
	AssignDevice(userID uint, device *Device) error
	GetByUserID(userID uint) ([]*Device, error)
	CreateDevice(device *Device) error
	GetBySerialNumber(serialNumber string) (*Device, error)
	Update(device *Device) error
	GetUnclaimedDevices() ([]*Device, error)
	DeleteByID(id uint) error
	// Add other methods as needed
}

// DeviceDataInterface defines the methods for DeviceData operations.
type DeviceDataInterface interface {
	CreateLog(data *DeviceData) error
	GetLogsByDeviceID(deviceID uint) ([]*DeviceData, error)
	GetLogsBySerialNumber(serialNumber string) ([]*DeviceData, error)
	DeleteByDeviceID(deviceID uint) error
}

// NotificationInterface defines the methods for Notification operations
type NotificationInterface interface {
	CreateNotification(notification *Notification) error
	GetUserNotifications(userID uint) ([]*Notification, error)
	GetUnreadNotifications(userID uint) ([]*Notification, error)
	MarkAsRead(id uint) error
	MarkAllAsRead(userID uint) error
	DeleteNotification(id uint) error
}
