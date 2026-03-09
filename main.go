package main

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	Email        string `json:"email,omitempty"`
	PasswordHash string `json:"-"`
}

type registerRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

var (
	usersMu   sync.Mutex
	users     = make(map[string]*User)
	nextID    = 1
	usersByID = make(map[int]*User)
)

// CORS middleware
func enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("pong"))
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
	usersMu.Lock()
	list := make([]User, 0, len(users))
	for _, u := range users {
		list = append(list, User{ID: u.ID, Username: u.Username, Email: u.Email})
	}
	usersMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	username := strings.TrimSpace(req.Username)

	if email == "" {
		writeJSONError(w, "Email is required", http.StatusBadRequest)
		return
	}
	if username == "" {
		writeJSONError(w, "Username is required", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 6 {
		writeJSONError(w, "Password must be at least 6 characters", http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("bcrypt error: %v", err)
		writeJSONError(w, "Registration failed", http.StatusInternalServerError)
		return
	}

	usersMu.Lock()
	defer usersMu.Unlock()

	if _, exists := users[email]; exists {
		writeJSONError(w, "Email already registered", http.StatusConflict)
		return
	}
	for _, u := range users {
		if strings.EqualFold(u.Username, username) {
			writeJSONError(w, "Username already taken", http.StatusConflict)
			return
		}
	}

	u := &User{
		ID:           nextID,
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
	}
	nextID++
	users[email] = u
	usersByID[u.ID] = u

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":       u.ID,
		"username": u.Username,
		"email":    u.Email,
	})
}

func writeJSONError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func serveLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.Path != "/login" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(".", "frontend", "login.html"))
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", enableCORS(pingHandler))
	mux.HandleFunc("/users", enableCORS(usersHandler))
	mux.HandleFunc("/register", enableCORS(registerHandler))
	mux.HandleFunc("/login", serveLogin)
	mux.HandleFunc("/login/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/login")
		if path == "" || path == "/" {
			http.ServeFile(w, r, filepath.Join(".", "frontend", "login.html"))
			return
		}
		if strings.HasPrefix(path, "/") {
			http.ServeFile(w, r, filepath.Join(".", "frontend", strings.TrimPrefix(path, "/")))
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(".", "index.html"))
	})

	log.Println("Server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
