// data/repository.go
package data

import "gorm.io/gorm"

// Models aggregates all the repository interfaces.
type Models struct {
	User       UserInterface
	Device     DeviceInterface
	DeviceData DeviceDataInterface
	// Add other repositories like Plan here if needed
}

// New creates an instance of the data package with initialized repositories.
func New(gormDB *gorm.DB) Models {
	return Models{
		User:       NewUserRepository(gormDB),
		Device:     NewDeviceRepository(gormDB),
		DeviceData: NewDeviceDataRepository(gormDB),
		// Initialize other repositories here
	}
}
