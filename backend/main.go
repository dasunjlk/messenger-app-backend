package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const frontendDir = "frontend"

func main() {
	if err := InitDB(); err != nil {
		log.Fatalf("database init failed: %v", err)
	}

	hub := NewHub()

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/ping", withCORS(pingHandler))
	mux.HandleFunc("/healthz", withCORS(healthHandler))
	mux.HandleFunc("/login", withCORS(loginRoute))
	mux.HandleFunc("/login/", withCORS(loginRoute))
	mux.HandleFunc("/register", withCORS(registerRoute))
	mux.HandleFunc("/register/", withCORS(registerRoute))
	mux.HandleFunc("/profile", withCORS(RequireAuth(profileHandler)))
	mux.HandleFunc("/chat", withCORS(chatRoute))
	mux.HandleFunc("/ws", withCORS(websocketHandler(hub)))

	// Static assets
	mux.Handle("/asserts/", http.StripPrefix("/asserts/", http.FileServer(http.Dir(filepath.Join(frontendDir, "asserts")))))
	mux.Handle("/style.css", http.FileServer(http.Dir(frontendDir)))

	mux.HandleFunc("/", serveIndex)

	addr := ":" + envOrDefault("PORT", "8080")
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("Server running at http://localhost%v", addr)
	log.Fatal(server.ListenAndServe())
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("pong"))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	http.ServeFile(w, r, filepath.Join(frontendDir, "index.html"))
}

func chatRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	http.ServeFile(w, r, filepath.Join(frontendDir, "chat.html"))
}

func loginRoute(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		http.ServeFile(w, r, filepath.Join(frontendDir, "login.html"))
	case http.MethodPost:
		loginHandler(w, r)
	default:
		methodNotAllowed(w)
	}
}

func registerRoute(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		http.ServeFile(w, r, filepath.Join(frontendDir, "register.html"))
	case http.MethodPost:
		registerHandler(w, r)
	default:
		methodNotAllowed(w)
	}
}

func withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next(w, r)
	}
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
