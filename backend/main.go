package main

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"strings"
)

func main() {
	// Connect to database
	if err := InitDB(); err != nil {
		log.Fatalf("database init failed: %v", err)
	}

	mux := http.NewServeMux()

	// Public routes
	mux.HandleFunc("/ping", enableCORS(pingHandler))
	mux.HandleFunc("/register", enableCORS(registerRoute))
	mux.HandleFunc("/login", enableCORS(loginRoute)) // GET=page, POST=API

	// Protected routes (require JWT)
	mux.HandleFunc("/profile", enableCORS(RequireAuth(profileHandler)))

	// Serve frontend static files (run from project root: go run ./backend)
	mux.HandleFunc("/", serveFrontend)
	mux.HandleFunc("/login/", func(w http.ResponseWriter, r *http.Request) {
		serveFile(w, r, strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/login"), "/"), "login.html")
	})
	mux.HandleFunc("/style.css", func(w http.ResponseWriter, r *http.Request) { serveFile(w, r, "style.css", "") })
	mux.HandleFunc("/register.html", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
		http.ServeFile(w, r, filepath.Join("frontend", "register.html"))
	})
	mux.HandleFunc("/asserts/", func(w http.ResponseWriter, r *http.Request) {
		serveFile(w, r, strings.TrimPrefix(r.URL.Path, "/"), "")
	})

	log.Println("Server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

// enableCORS adds CORS headers for API requests
func enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
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

// profileHandler returns the authenticated user's info (GET /profile, requires JWT)
func profileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := GetUserID(r.Context())
	if !ok {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var id int
	var username, email string
	err := DB.QueryRow(
		`SELECT id, username, email FROM users WHERE id = @p1`,
		userID,
	).Scan(&id, &username, &email)

	if err != nil {
		writeJSONError(w, "user not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":       id,
		"username": username,
		"email":    email,
	})
}

// serveFrontend handles / and serves index.html
func serveFrontend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join("frontend", "index.html"))
}

// loginRoute: GET serves login page, POST calls LoginHandler
func loginRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		http.ServeFile(w, r, filepath.Join("frontend", "login.html"))
		return
	}
	LoginHandler(w, r)
}

// registerRoute: GET serves register page, POST calls RegisterHandler
func registerRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		http.ServeFile(w, r, filepath.Join("frontend", "register.html"))
		return
	}
	RegisterHandler(w, r)
}

// serveFile serves a file from frontend/ directory
func serveFile(w http.ResponseWriter, r *http.Request, path, defaultFile string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if path == "" {
		path = defaultFile
	}
	if path == "" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join("frontend", path))
}
