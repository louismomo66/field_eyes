package main

import (
	"bytes"
	"errors"
	"field_eyes/data"
	"fmt"
	"io"
	"net/http"

	"gorm.io/gorm"
)

func (app *Config) Signup(w http.ResponseWriter, r *http.Request) {
	// Log the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}
	app.InfoLog.Printf("Received signup request body: %s", string(body))
	// Restore the body for further reading
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	var user data.User
	if err := app.ReadJSON(w, r, &user); err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}

	// Log the parsed user data
	app.InfoLog.Printf("Parsed user data: username=%s, email=%s, password=%s", user.Username, user.Email, user.TempPassword)

	// Validate required fields
	if user.Username == "" || user.Email == "" || user.TempPassword == "" {
		app.errorJSON(w, errors.New("username, email and password are required"), http.StatusBadRequest)
		app.ErrorLog.Println("username, email and password are empty")
		return
	}

	// Check if user exists
	existingUser, err := app.Models.User.GetByEmail(user.Email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		app.errorJSON(w, errors.New("database error"), http.StatusInternalServerError)
		app.ErrorLog.Println(err)
		return
	}

	// If user exists (not nil and has email)
	if existingUser != nil && existingUser.Email != "" {
		app.errorJSON(w, errors.New("user already exists"), http.StatusBadRequest)
		app.ErrorLog.Println("user already exists")
		return
	}

	// No need to hash password here, it's done in the Insert function
	id, err := app.Models.User.Insert(&user)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		app.ErrorLog.Println(err)
		return
	}

	// Return standardized response
	app.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message": fmt.Sprintf("User created successfully with id %d", id),
		"user_id": id,
	})
	app.InfoLog.Printf("User created successfully with id %d", id)
}

func (app *Config) Login(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := app.ReadJSON(w, r, &request); err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}

	// Validate required fields
	if request.Email == "" || request.Password == "" {
		app.errorJSON(w, errors.New("email and password are required"), http.StatusBadRequest)
		app.ErrorLog.Println("email and password are empty")
		return
	}

	user, err := app.Models.User.GetByEmail(request.Email)
	if err != nil {
		app.errorJSON(w, errors.New("invalid credentials"), http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}

	isMatch, err := app.Models.User.PasswordMatches(user, request.Password)
	if err != nil {
		app.errorJSON(w, errors.New("authentication error"), http.StatusInternalServerError)
		app.ErrorLog.Println(err)
		return
	}
	if !isMatch {
		app.errorJSON(w, errors.New("invalid credentials"), http.StatusBadRequest)
		app.ErrorLog.Println("Invalid password")
		return
	}

	token, err := app.GenerateJWT(*user)
	if err != nil {
		app.errorJSON(w, errors.New("failed to generate token"), http.StatusInternalServerError)
		app.ErrorLog.Println(err)
		return
	}

	// Create a user response without password
	userResponse := map[string]interface{}{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
		"role":     user.Role,
	}

	// Respond with the token and user
	app.writeJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user":  userResponse,
	})
}

// ForgotPassword handles the forgot password request by generating an OTP and sending it via email
func (app *Config) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	// Parse request to get email
	var request struct {
		Email string `json:"email"`
	}

	if err := app.ReadJSON(w, r, &request); err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}

	// Check if email is valid
	if request.Email == "" {
		app.errorJSON(w, errors.New("email is required"), http.StatusBadRequest)
		return
	}

	// Check if user exists
	user, err := app.Models.User.GetByEmail(request.Email)
	if err != nil || user == nil {
		// Don't reveal that the user doesn't exist for security
		app.writeJSON(w, http.StatusOK, map[string]string{
			"message": "If your email is registered, you will receive an OTP shortly",
		})
		app.InfoLog.Printf("Forgot password requested for non-existent email: %s", request.Email)
		return
	}

	// Generate OTP
	otp, err := app.Models.User.GenerateAndSaveOTP(request.Email)
	if err != nil {
		app.errorJSON(w, errors.New("failed to generate OTP"), http.StatusInternalServerError)
		app.ErrorLog.Printf("Failed to generate OTP: %v", err)
		return
	}

	// Send OTP via email in a goroutine
	app.SendEmailInBackground(request.Email, otp)

	// Respond to the user
	app.writeJSON(w, http.StatusOK, map[string]string{
		"message": "If your email is registered, you will receive an OTP shortly",
	})
	app.InfoLog.Printf("Forgot password OTP generated for email: %s", request.Email)
}

// ResetPassword handles the password reset using the OTP
func (app *Config) ResetPassword(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var request struct {
		Email       string `json:"email"`
		OTP         string `json:"otp"`
		NewPassword string `json:"new_password"`
	}

	if err := app.ReadJSON(w, r, &request); err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}

	// Validate request
	if request.Email == "" || request.OTP == "" || request.NewPassword == "" {
		app.errorJSON(w, errors.New("email, OTP, and new password are required"), http.StatusBadRequest)
		return
	}

	// Validate password strength
	if len(request.NewPassword) < 8 {
		app.errorJSON(w, errors.New("password must be at least 8 characters"), http.StatusBadRequest)
		return
	}

	// Try to reset password with OTP
	err := app.Models.User.ResetPasswordWithOTP(request.Email, request.OTP, request.NewPassword)
	if err != nil {
		app.errorJSON(w, errors.New("invalid or expired OTP"), http.StatusBadRequest)
		app.ErrorLog.Printf("Password reset failed: %v", err)
		return
	}

	// Return success
	app.writeJSON(w, http.StatusOK, map[string]string{
		"message": "Password has been reset successfully",
	})
	app.InfoLog.Printf("Password reset successful for email: %s", request.Email)
}
