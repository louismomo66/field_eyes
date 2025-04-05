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
}

// DeviceInterface defines the methods for Device operations.
type DeviceInterface interface {
	GetAll() ([]*Device, error)
	GetOne(id uint) (*Device, error)
	AssignDevice(userID uint, device *Device) error
	GetByUserID(userID uint) ([]*Device, error)
	CreateDevice(device *Device) error
	GetBySerialNumber(serialNumber string) (*Device, error)
	// Add other methods as needed
}

// DeviceDataInterface defines the methods for DeviceData operations.
type DeviceDataInterface interface {
	CreateLog(data *DeviceData) error
	GetLogsByDeviceID(deviceID uint) ([]*DeviceData, error)
	GetLogsBySerialNumber(serialNumber string) ([]*DeviceData, error)
}
