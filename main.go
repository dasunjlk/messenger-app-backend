package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

// CORS middleware
func enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		next(w, r)
	}
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("pong"))
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
	users := []User{
		{ID: 1, Username: "alice"},
		{ID: 2, Username: "bob"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func main() {
	http.HandleFunc("/ping", enableCORS(pingHandler))
	http.HandleFunc("/users", enableCORS(usersHandler))

	log.Println("Server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
