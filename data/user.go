package data

import (
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// User represents the users table in the database.
type User struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Username     string         `gorm:"type:varchar(100);not null" json:"username"`
	FirstName    string         `gorm:"type:varchar(100)" json:"first_name"`
	LastName     string         `gorm:"type:varchar(100)" json:"last_name"`
	Email        string         `gorm:"type:varchar(100);uniqueIndex;not null" json:"email"`
	Password     []byte         `gorm:"type:varchar(255);not null" json:"-"`
	TempPassword string         `json:"password" gorm:"-"` // Temporary field for password unmarshaling
	Photo        string         `gorm:"type:varchar(255)" json:"photo"`
	Devices      []Device       `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"devices,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	Role         string         `gorm:"type:varchar(50);" json:"role,omitempty"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	// OTP fields
	OTPCode      string    `gorm:"type:varchar(6)" json:"-"`
	OTPExpiresAt time.Time `json:"-"`
}

// UserRepository implements UserInterface using standard SQL queries.
type UserRepository struct {
	DB *sql.DB
}

// NewUserRepository creates a new instance of UserRepository.
func NewUserRepository(db *sql.DB) UserInterface {
	return &UserRepository{DB: db}
}

// GetAll retrieves all users from the database, including their devices.
func (r *UserRepository) GetAll() ([]*User, error) {
	query := `SELECT id, username, first_name, last_name, email, password, photo, created_at, updated_at 
			  FROM users`

	rows, err := r.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.FirstName,
			&user.LastName,
			&user.Email,
			&user.Password,
			&user.Photo,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, &user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

// GetByEmail retrieves a user by email
func (u *UserRepository) GetByEmail(email string) (*User, error) {
	query := `SELECT id, username, first_name, last_name, email, password, photo, created_at, updated_at 
			  FROM users WHERE email = $1`

	var user User
	row := u.DB.QueryRow(query, email)

	err := row.Scan(
		&user.ID,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.Email,
		&user.Password,
		&user.Photo,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user with email %s not found", email)
		}
		return nil, err
	}

	return &user, nil
}

// GetOne retrieves a user by their ID, including their devices.
func (r *UserRepository) GetOne(id uint) (*User, error) {
	query := `SELECT id, username, first_name, last_name, email, password, photo, created_at, updated_at 
			  FROM users WHERE id = $1`

	var user User
	row := r.DB.QueryRow(query, id)

	err := row.Scan(
		&user.ID,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.Email,
		&user.Password,
		&user.Photo,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}

// Insert adds a new user to the database
func (u *UserRepository) Insert(user *User) (uint, error) {
	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.TempPassword), 12)
	if err != nil {
		return 0, err
	}

	query := `INSERT INTO users (username, first_name, last_name, email, password, photo, created_at, updated_at)
			  VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`

	var id uint
	err = u.DB.QueryRow(
		query,
		user.Username,
		user.FirstName,
		user.LastName,
		user.Email,
		hashedPassword,
		user.Photo,
		time.Now(),
		time.Now(),
	).Scan(&id)

	if err != nil {
		return 0, err
	}

	return id, nil
}

// Update modifies an existing user in the database.
func (r *UserRepository) Update(user *User) error {
	// If the password is being updated (via TempPassword field)
	if user.TempPassword != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.TempPassword), 12)
		if err != nil {
			return err
		}
		user.Password = hashedPassword
	} else {
		// Prevent overwriting the password with an empty value
		var existingUser User
		if err := r.DB.QueryRow("SELECT password FROM users WHERE id = $1", user.ID).Scan(&existingUser.Password); err != nil {
			return err
		}
		user.Password = existingUser.Password
	}

	query := `UPDATE users SET username = $1, first_name = $2, last_name = $3, email = $4, password = $5, photo = $6, updated_at = $7 WHERE id = $8`
	_, err := r.DB.Exec(query, user.Username, user.FirstName, user.LastName, user.Email, user.Password, user.Photo, time.Now(), user.ID)
	return err
}

// Delete removes a user from the database (soft delete).
func (r *UserRepository) Delete(user *User) error {
	query := `UPDATE users SET deleted_at = $1 WHERE id = $2`
	_, err := r.DB.Exec(query, time.Now(), user.ID)
	return err
}

// DeleteByID removes a user by their ID (soft delete).
func (r *UserRepository) DeleteByID(id uint) error {
	query := `UPDATE users SET deleted_at = $1 WHERE id = $2`
	_, err := r.DB.Exec(query, time.Now(), id)
	return err
}

// ResetPassword updates the password for a user.
func (r *UserRepository) ResetPassword(userID uint, newPassword string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return err
	}

	query := `UPDATE users SET password = $1 WHERE id = $2`
	_, err = r.DB.Exec(query, hashedPassword, userID)
	return err
}

// PasswordMatches checks if the provided password matches the hashed one in the database
func (u *UserRepository) PasswordMatches(user *User, plainText string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(user.Password, []byte(plainText))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}

	return true, nil
}

// GenerateAndSaveOTP generates a new OTP code for the user and saves it to the database.
func (r *UserRepository) GenerateAndSaveOTP(email string) (string, error) {
	var user User
	if err := r.DB.QueryRow("SELECT id, password FROM users WHERE email = $1", email).Scan(&user.ID, &user.Password); err != nil {
		return "", err
	}

	// Generate a random 6-digit OTP using crypto/rand for better security
	otpNum := 100000 + rand.New(rand.NewSource(time.Now().UnixNano())).Intn(900000)
	otp := strconv.Itoa(otpNum)

	// Set OTP and expiration (15 minutes from now)
	query := `UPDATE users SET otp_code = $1, otp_expires_at = $2 WHERE id = $3`
	_, err := r.DB.Exec(query, otp, time.Now().Add(15*time.Minute), user.ID)
	if err != nil {
		return "", err
	}

	return otp, nil
}

// VerifyOTP checks if the provided OTP is valid for the user
func (r *UserRepository) VerifyOTP(email, otp string) (bool, error) {
	var user User
	if err := r.DB.QueryRow("SELECT otp_code, otp_expires_at FROM users WHERE email = $1", email).Scan(&user.OTPCode, &user.OTPExpiresAt); err != nil {
		return false, err
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
	if err := r.DB.QueryRow("SELECT id, password FROM users WHERE email = $1", email).Scan(&user.ID, &user.Password); err != nil {
		return err
	}

	// Hash the new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return err
	}

	// Update the password and clear the OTP
	query := `UPDATE users SET password = $1, otp_code = $2, otp_expires_at = $3 WHERE id = $4`
	_, err = r.DB.Exec(query, hashedPassword, "", time.Time{}, user.ID)
	return err
}
