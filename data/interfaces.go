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
	// Add other methods as needed
}
