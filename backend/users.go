package main

import (
	"log"
	"net/http"
)

type userSummary struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

// listUsersHandler returns all registered users.
// NOTE: This currently allows any authenticated user. Restrict further if needed.
func listUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	rows, err := DB.Query(`SELECT id, username, email, created_at FROM users ORDER BY id ASC`)
	if err != nil {
		logRequestError(w, "query users", err)
		return
	}
	defer rows.Close()

	var users []userSummary
	for rows.Next() {
		var u userSummary
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.CreatedAt); err != nil {
			log.Printf("scan user row: %v", err)
			continue
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		logRequestError(w, "iterate users", err)
		return
	}

	writeJSON(w, http.StatusOK, users)
}
