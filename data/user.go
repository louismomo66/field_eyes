package data

import (
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// User represents the users table in the database.
type User struct {
	gorm.Model
	Username  string         `gorm:"type:varchar(100);not null" json:"username"`
	Email     string         `gorm:"type:varchar(100);uniqueIndex;not null" json:"email"`
	Password  string         `gorm:"type:varchar(255);not null" json:"-"`
	Devices   []Device       `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"devices,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Role      string         `gorm:"type:varchar(50);" json:"role,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// UserRepository implements UserInterface using GORM.
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new instance of UserRepository.
func NewUserRepository(db *gorm.DB) UserInterface {
	return &UserRepository{db: db}
}

// HashPassword creates a bcrypt hash of the password
func HashPassword(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedBytes), nil
}

// GetAll retrieves all users from the database, including their devices.
func (r *UserRepository) GetAll() ([]*User, error) {
	var users []*User
	result := r.db.Preload("Devices").Find(&users)
	return users, result.Error
}

// GetByEmail retrieves a user by their email, including their devices.
func (r *UserRepository) GetByEmail(email string) (*User, error) {
	var user User
	result := r.db.Where("email = ?", email).Preload("Devices").First(&user)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &user, result.Error
}

// GetOne retrieves a user by their ID, including their devices.
func (r *UserRepository) GetOne(id uint) (*User, error) {
	var user User
	result := r.db.Preload("Devices").First(&user, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &user, result.Error
}

// Insert creates a new user in the database after hashing the password.
func (r *UserRepository) Insert(user *User) (uint, error) { // Changed return type to match interface
	// Hash the password before saving
	hashedPassword, err := HashPassword(user.Password)
	if err != nil {
		return 0, err
	}
	user.Password = hashedPassword

	result := r.db.Create(user)
	if result.Error != nil {
		return 0, result.Error
	}
	return user.Model.ID, nil // Using the embedded Model's ID field
}

// Update modifies an existing user in the database.
func (r *UserRepository) Update(user *User) error {
	// If the password is being updated, hash it
	if user.Password != "" {
		hashedPassword, err := HashPassword(user.Password)
		if err != nil {
			return err
		}
		user.Password = hashedPassword
	} else {
		// Prevent overwriting the password with an empty string
		var existingUser User
		if err := r.db.First(&existingUser, user.Model.ID).Error; err != nil { // Using the embedded Model's ID field
			return err
		}
		user.Password = existingUser.Password
	}

	result := r.db.Save(user)
	return result.Error
}

// Delete removes a user from the database (soft delete).
func (r *UserRepository) Delete(user *User) error {
	result := r.db.Delete(user)
	return result.Error
}

// DeleteByID removes a user by their ID (soft delete).
func (r *UserRepository) DeleteByID(id uint) error {
	result := r.db.Delete(&User{}, id)
	return result.Error
}

// ResetPassword updates the password for a user.
func (r *UserRepository) ResetPassword(userID uint, newPassword string) error {
	var user User
	if err := r.db.First(&user, userID).Error; err != nil {
		return err
	}

	hashedPassword, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	user.Password = hashedPassword
	result := r.db.Save(&user)
	return result.Error
}

// PasswordMatches checks if the provided plain text password matches the stored hashed password.
func (r *UserRepository) PasswordMatches(user *User, plainText string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(plainText))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
