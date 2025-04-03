package main

import (
	"errors"
	"field_eyes/data"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt"
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
		app.errorJSON(w, errors.New("Username, Email and Password are required"), http.StatusBadRequest)
		app.ErrorLog.Println(errors.New("Usernam, Email and password are empty"))
		return
	}

	user1, err := app.Models.User.GetByEmail(user.Email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		app.errorJSON(w, err, http.StatusBadRequest)
		app.ErrorLog.Println(errors.New("User does not exist"))
		return
	}
	if user1.Email != "" {
		app.errorJSON(w, errors.New("user already exists"), http.StatusBadRequest)
		app.ErrorLog.Println(errors.New("User already exists"))
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

func (app *Config) GenerateJWT(user data.User) (string, error) {
	mySigningKey := os.Getenv("JWT_SECRETE")
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"email":   user.Email,
		"role":    user.Role,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(mySigningKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
