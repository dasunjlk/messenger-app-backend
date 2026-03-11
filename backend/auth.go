package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// writeJSONError sends a JSON error response
func writeJSONError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// logRequestError logs an error and sends a 500 response
func logRequestError(w http.ResponseWriter, context string, err error) {
	log.Printf("%s: %v", context, err)
	writeJSONError(w, "internal server error", http.StatusInternalServerError)
}

// jwtSecret is loaded from JWT_SECRET env var, or a default for development
func getJWTSecret() []byte {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return []byte(s)
	}
	return []byte("your-secret-key-change-in-production")
}

// CustomClaims holds the JWT payload including user_id
type CustomClaims struct {
	UserID int `json:"user_id"`
	jwt.RegisteredClaims
}

// RegisterRequest is the JSON body for POST /register
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginRequest is the JSON body for POST /login
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterHandler handles POST /register - creates a new user
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(strings.ToLower(req.Email))

	if username == "" {
		writeJSONError(w, "username is required", http.StatusBadRequest)
		return
	}
	if email == "" {
		writeJSONError(w, "email is required", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 6 {
		writeJSONError(w, "password must be at least 6 characters", http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logRequestError(w, "bcrypt error", err)
		return
	}

	var id int
	err = DB.QueryRow(
		`INSERT INTO users (username, email, password_hash) OUTPUT INSERTED.id VALUES (@p1, @p2, @p3)`,
		username, email, string(hash),
	).Scan(&id)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") || strings.Contains(err.Error(), "UNIQUE KEY") ||
			strings.Contains(err.Error(), "Violation of UNIQUE KEY") {
			writeJSONError(w, "username or email already exists", http.StatusConflict)
			return
		}
		logRequestError(w, "database error", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":       id,
		"username": username,
		"email":    email,
	})
}

// LoginHandler handles POST /login - authenticates user and returns JWT
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" || req.Password == "" {
		writeJSONError(w, "email and password are required", http.StatusBadRequest)
		return
	}

	var id int
	var username string
	var passwordHash string
	err := DB.QueryRow(
		`SELECT id, username, password_hash FROM users WHERE email = @p1`,
		email,
	).Scan(&id, &username, &passwordHash)

	if err == sql.ErrNoRows {
		writeJSONError(w, "invalid email or password", http.StatusUnauthorized)
		return
	}
	if err != nil {
		logRequestError(w, "database error", err)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		writeJSONError(w, "invalid email or password", http.StatusUnauthorized)
		return
	}

	// Generate JWT
	claims := CustomClaims{
		UserID: id,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(getJWTSecret())
	if err != nil {
		logRequestError(w, "jwt error", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":    tokenString,
		"user_id":  id,
		"username": username,
		"email":    email,
	})
}

// GenerateToken creates a JWT for a user (used by middleware for validation)
func GenerateToken(userID int) (string, error) {
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

// ParseToken validates a JWT and returns the claims
func ParseToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
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
