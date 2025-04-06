package main

import (
	"errors"
	"field_eyes/data"
	"fmt"
	"net/http"

	"gorm.io/gorm"
)

func (app *Config) Signup(w http.ResponseWriter, r *http.Request) {
	var user data.User
	if err := app.ReadJSON(w, r, user); err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}
	if user.Username == "" || user.Email == "" || user.Password == "" {
		app.errorJSON(w, errors.New("username, Email and Password are required"), http.StatusBadRequest)
		app.ErrorLog.Println(errors.New("usernam, Email and password are empty"))
		return
	}

	user1, err := app.Models.User.GetByEmail(user.Email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		app.errorJSON(w, err, http.StatusBadRequest)
		app.ErrorLog.Println(errors.New("user does not exist"))
		return
	}
	if user1.Email != "" {
		app.errorJSON(w, errors.New("user already exists"), http.StatusBadRequest)
		app.ErrorLog.Println(errors.New("user already exists"))
		return
	}

	hashedPassword, err := data.HashPassword(user.Password)
	if err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}

	user.Password = hashedPassword
	id, err := app.Models.User.Insert(&user)
	if err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}
	app.writeJSON(w, http.StatusCreated, fmt.Sprintf("User created successfully with id %d", id))
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
	user1, err := app.Models.User.GetByEmail(request.Email)
	if err != nil {
		app.errorJSON(w, errors.New("user doesn't exist"), http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}

	ismatch, err := app.Models.User.PasswordMatches(user1, request.Password)
	if err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		app.ErrorLog.Println(err)
		return
	}
	if !ismatch {
		app.errorJSON(w, errors.New("invalid password"), http.StatusBadRequest)
		app.ErrorLog.Println(errors.New("Invalid password"))
		return
	}
	token, err := app.GenerateJWT(*user1)
	if err != nil {
		app.errorJSON(w, errors.New("failed to generate token"), http.StatusInternalServerError)
		app.ErrorLog.Println(err)
		return
	}

	// Respond with the token
	app.writeJSON(w, http.StatusOK, map[string]string{
		"token": token,
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
