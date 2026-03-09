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

type loginRequest struct {
	Email    string `json:"email"`
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

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" || req.Password == "" {
		writeJSONError(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	usersMu.Lock()
	u, exists := users[email]
	usersMu.Unlock()

	if !exists {
		writeJSONError(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		writeJSONError(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
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

func serveLoginPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.ServeFile(w, r, filepath.Join(".", "frontend", "login.html"))
}

func serveFrontendFile(w http.ResponseWriter, r *http.Request, path string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.ServeFile(w, r, filepath.Join(".", "frontend", path))
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", enableCORS(pingHandler))
	mux.HandleFunc("/users", enableCORS(usersHandler))
	mux.HandleFunc("/register", enableCORS(registerHandler))
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			enableCORS(loginHandler)(w, r)
			return
		}
		serveLoginPage(w, r)
	})
	mux.HandleFunc("/style.css", func(w http.ResponseWriter, r *http.Request) { serveFrontendFile(w, r, "style.css") })
	mux.HandleFunc("/asserts/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		http.ServeFile(w, r, filepath.Join(".", "frontend", path))
	})
	mux.HandleFunc("/login/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/login")
		if path == "" || path == "/" {
			serveLoginPage(w, r)
			return
		}
		serveFrontendFile(w, r, strings.TrimPrefix(path, "/"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		serveLoginPage(w, r)
	})

	log.Println("Server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
