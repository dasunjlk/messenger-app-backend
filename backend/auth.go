package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// CustomClaims represents the JWT payload we issue to clients.
type CustomClaims struct {
	UserID int `json:"user_id"`
	jwt.RegisteredClaims
}

type registerRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// registerHandler creates a new user and returns minimal profile data.
func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	username := strings.TrimSpace(req.Username)
	password := req.Password

	if email == "" || !emailRegex.MatchString(email) {
		writeJSONError(w, http.StatusBadRequest, "valid email is required")
		return
	}
	if username == "" {
		writeJSONError(w, http.StatusBadRequest, "username is required")
		return
	}
	if len(password) < 6 {
		writeJSONError(w, http.StatusBadRequest, "password must be at least 6 characters")
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		logRequestError(w, "bcrypt", err)
		return
	}

	res, err := DB.Exec(
		`INSERT INTO users (username, email, password_hash) VALUES (?, ?, ?)`,
		username, email, string(hashed),
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			writeJSONError(w, http.StatusConflict, "username or email already exists")
			return
		}
		logRequestError(w, "insert user", err)
		return
	}

	id, _ := res.LastInsertId()
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":       int(id),
		"username": username,
		"email":    email,
	})
}

// loginHandler authenticates a user and issues a JWT.
func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	password := req.Password
	if email == "" || password == "" {
		writeJSONError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	var id int
	var username, hash string
	err := DB.QueryRow(`SELECT id, username, password_hash FROM users WHERE email = ?`, email).
		Scan(&id, &username, &hash)
	if err == sql.ErrNoRows {
		writeJSONError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if err != nil {
		logRequestError(w, "query user", err)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		writeJSONError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	token, err := issueToken(id)
	if err != nil {
		logRequestError(w, "sign token", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":    token,
		"user_id":  id,
		"username": username,
		"email":    email,
	})
}

// profileHandler returns the authenticated user's profile.
func profileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	userID, ok := GetUserID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var user struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
	}

	err := DB.QueryRow(`SELECT id, username, email FROM users WHERE id = ?`, userID).
		Scan(&user.ID, &user.Username, &user.Email)
	if err == sql.ErrNoRows {
		writeJSONError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		logRequestError(w, "query profile", err)
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func issueToken(userID int) (string, error) {
	claims := CustomClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(getJWTSecret())
}

func parseToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(t *jwt.Token) (interface{}, error) {
		return getJWTSecret(), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrTokenInvalidClaims
}

func getJWTSecret() []byte {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return []byte(s)
	}
	return []byte("change-me-in-production")
}

func isDuplicateKeyError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique")
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("write json: %v", err)
	}
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func logRequestError(w http.ResponseWriter, context string, err error) {
	log.Printf("%s: %v", context, err)
	writeJSONError(w, http.StatusInternalServerError, "internal server error")
}
