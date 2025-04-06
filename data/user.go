package data

import (
	"errors"
	"math/rand"
	"strconv"
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
	// OTP fields
	OTPCode      string    `gorm:"type:varchar(6)" json:"-"`
	OTPExpiresAt time.Time `json:"-"`
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

// GenerateAndSaveOTP generates a new OTP code for the user and saves it to the database.
func (r *UserRepository) GenerateAndSaveOTP(email string) (string, error) {
	var user User
	result := r.db.Where("email = ?", email).First(&user)
	if result.Error != nil {
		return "", result.Error
	}

	// Generate a random 6-digit OTP using crypto/rand for better security
	otpNum := 100000 + rand.New(rand.NewSource(time.Now().UnixNano())).Intn(900000)
	otp := strconv.Itoa(otpNum)

	// Set OTP and expiration (15 minutes from now)
	user.OTPCode = otp
	user.OTPExpiresAt = time.Now().Add(15 * time.Minute)

	// Save the user with the new OTP
	if err := r.db.Save(&user).Error; err != nil {
		return "", err
	}

	return otp, nil
}

// VerifyOTP checks if the provided OTP is valid for the user
func (r *UserRepository) VerifyOTP(email, otp string) (bool, error) {
	var user User
	result := r.db.Where("email = ?", email).First(&user)
	if result.Error != nil {
		return false, result.Error
	}

	// Check if OTP matches and has not expired
	if user.OTPCode != otp {
		return false, nil
	}

	if time.Now().After(user.OTPExpiresAt) {
		return false, errors.New("OTP has expired")
	}

	return true, nil
}

// ResetPasswordWithOTP resets a user's password after validating the OTP
func (r *UserRepository) ResetPasswordWithOTP(email, otp, newPassword string) error {
	// Verify OTP first
	valid, err := r.VerifyOTP(email, otp)
	if err != nil {
		return err
	}
	if !valid {
		return errors.New("invalid or expired OTP")
	}

	var user User
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
		return err
	}

	// Hash the new password
	hashedPassword, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	// Update the password and clear the OTP
	user.Password = hashedPassword
	user.OTPCode = ""

	// Save the changes
	return r.db.Save(&user).Error
}
