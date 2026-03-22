package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"
)

type Conversation struct {
	ID        int       `json:"id"`
	User1ID   int       `json:"user1_id"`
	User2ID   int       `json:"user2_id"`
	CreatedAt time.Time `json:"created_at"`
}

func normalizePair(a, b int) (int, int) {
	if a <= b {
		return a, b
	}
	return b, a
}

// ensureConversation returns the existing conversation between two users
// or creates one (ordering user ids to enforce uniqueness).
func ensureConversation(userA, userB int) (Conversation, error) {
	var conv Conversation
	if userA == 0 || userB == 0 {
		return conv, fmt.Errorf("missing participant id")
	}

	u1, u2 := normalizePair(userA, userB)

	// Try fetch existing
	err := DB.QueryRow(
		`SELECT id, user1_id, user2_id, created_at FROM conversations WHERE user1_id=$1 AND user2_id=$2`,
		u1, u2,
	).Scan(&conv.ID, &conv.User1ID, &conv.User2ID, &conv.CreatedAt)
	if err == nil {
		return conv, nil
	}
	if err != sql.ErrNoRows {
		return conv, err
	}

	// Create if not exists
	err = DB.QueryRow(
		`INSERT INTO conversations (user1_id, user2_id) VALUES ($1, $2)
         ON CONFLICT (user1_id, user2_id) DO NOTHING
         RETURNING id, user1_id, user2_id, created_at`,
		u1, u2,
	).Scan(&conv.ID, &conv.User1ID, &conv.User2ID, &conv.CreatedAt)
	if err == sql.ErrNoRows {
		// Someone else created concurrently; fetch it.
		err = DB.QueryRow(
			`SELECT id, user1_id, user2_id, created_at FROM conversations WHERE user1_id=$1 AND user2_id=$2`,
			u1, u2,
		).Scan(&conv.ID, &conv.User1ID, &conv.User2ID, &conv.CreatedAt)
	}
	return conv, err
}

// participantIDs returns both participants for a conversation id.
func participantIDs(conversationID int) (int, int, error) {
	var u1, u2 int
	err := DB.QueryRow(`SELECT user1_id, user2_id FROM conversations WHERE id=$1`, conversationID).
		Scan(&u1, &u2)
	return u1, u2, err
}

func conversationHasUser(conversationID, userID int) (bool, error) {
	var exists bool
	err := DB.QueryRow(
		`SELECT EXISTS(
            SELECT 1 FROM conversations WHERE id=$1 AND (user1_id=$2 OR user2_id=$2)
        )`,
		conversationID, userID,
	).Scan(&exists)
	return exists, err
}

type conversationSummary struct {
	ID           int              `json:"id"`
	OtherUser    userSummary      `json:"other_user"`
	LastMessage  *lastMessageInfo `json:"last_message,omitempty"`
	LastActivity time.Time        `json:"last_message_at"`
}

type lastMessageInfo struct {
	ID        int       `json:"id"`
	SenderID  int       `json:"sender_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func conversationsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	userID, ok := GetUserID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	rows, err := DB.Query(`
SELECT
    c.id,
    CASE WHEN c.user1_id = $1 THEN c.user2_id ELSE c.user1_id END AS other_id,
    u.username,
    u.email,
    lm.id,
    lm.sender_id,
    lm.content,
    lm.created_at,
    COALESCE(lm.created_at, c.created_at) AS last_activity
FROM conversations c
JOIN users u ON u.id = CASE WHEN c.user1_id = $1 THEN c.user2_id ELSE c.user1_id END
LEFT JOIN LATERAL (
    SELECT id, sender_id, content, created_at
    FROM messages m
    WHERE m.conversation_id = c.id
    ORDER BY created_at DESC
    LIMIT 1
) lm ON TRUE
WHERE c.user1_id = $1 OR c.user2_id = $1
ORDER BY last_activity DESC;`, userID)
	if err != nil {
		logRequestError(w, "query conversations", err)
		return
	}
	defer rows.Close()

	var result []conversationSummary
	for rows.Next() {
		var summary conversationSummary
		var lm lastMessageInfo
		var lmID sql.NullInt64
		var lmSender sql.NullInt64
		var lmContent sql.NullString
		var lmCreated sql.NullTime

		err := rows.Scan(
			&summary.ID,
			&summary.OtherUser.ID,
			&summary.OtherUser.Username,
			&summary.OtherUser.Email,
			&lmID,
			&lmSender,
			&lmContent,
			&lmCreated,
			&summary.LastActivity,
		)
		if err != nil {
			logRequestError(w, "scan conversation row", err)
			return
		}

		if lmID.Valid {
			lm.ID = int(lmID.Int64)
			if lmSender.Valid {
				lm.SenderID = int(lmSender.Int64)
			}
			if lmContent.Valid {
				lm.Content = lmContent.String
			}
			if lmCreated.Valid {
				lm.CreatedAt = lmCreated.Time
			}
			summary.LastMessage = &lm
		}

		result = append(result, summary)
	}

	if err := rows.Err(); err != nil {
		logRequestError(w, "iterate conversations", err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}
