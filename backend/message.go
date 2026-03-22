package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ChatMessage describes the payload sent over WebSocket connections and HTTP APIs.
type ChatMessage struct {
	ID             int       `json:"id"`
	Type           string    `json:"type"`
	ConversationID int       `json:"conversation_id"`
	SenderID       int       `json:"sender_id"`
	ReceiverID     int       `json:"receiver_id"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

const (
	defaultMessageType = "message"
	maxPageSize        = 100
)

// persistMessage ensures a conversation exists, saves the message, and returns the stored record.
func persistMessage(senderID, receiverID, conversationID int, content string) (ChatMessage, error) {
	content = strings.TrimSpace(content)
	if senderID == 0 || content == "" {
		return ChatMessage{}, errors.New("sender and content are required")
	}

	// Resolve receiver from conversation if not provided.
	if receiverID == 0 && conversationID > 0 {
		u1, u2, err := participantIDs(conversationID)
		if err != nil {
			return ChatMessage{}, fmt.Errorf("conversation lookup: %w", err)
		}
		switch senderID {
		case u1:
			receiverID = u2
		case u2:
			receiverID = u1
		default:
			return ChatMessage{}, errors.New("sender not part of conversation")
		}
	}

	if receiverID == 0 {
		return ChatMessage{}, errors.New("receiver is required")
	}

	conv, err := ensureConversation(senderID, receiverID)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("ensure conversation: %w", err)
	}

	if conversationID > 0 && conversationID != conv.ID {
		return ChatMessage{}, errors.New("conversation id does not match participants")
	}

	var msg ChatMessage
	err = DB.QueryRow(
		`INSERT INTO messages (conversation_id, sender_id, receiver_id, content)
         VALUES ($1, $2, $3, $4)
         RETURNING id, created_at`,
		conv.ID, senderID, receiverID, content,
	).Scan(&msg.ID, &msg.CreatedAt)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("insert message: %w", err)
	}

	msg.ConversationID = conv.ID
	msg.SenderID = senderID
	msg.ReceiverID = receiverID
	msg.Content = content
	msg.Type = defaultMessageType
	return msg, nil
}

func fetchMessages(conversationID, limit, offset int) ([]ChatMessage, error) {
	if limit <= 0 || limit > maxPageSize {
		limit = maxPageSize
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := DB.Query(
		`SELECT id, conversation_id, sender_id, receiver_id, content, created_at
         FROM messages
         WHERE conversation_id=$1
         ORDER BY created_at ASC
         LIMIT $2 OFFSET $3`,
		conversationID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []ChatMessage
	for rows.Next() {
		var m ChatMessage
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.ReceiverID, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.Type = defaultMessageType
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func messagesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	userID, ok := GetUserID(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	q := r.URL.Query()
	convID, err := strconv.Atoi(q.Get("conversation_id"))
	if err != nil || convID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "conversation_id is required")
		return
	}

	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	inConv, err := conversationHasUser(convID, userID)
	if err != nil {
		logRequestError(w, "check conversation membership", err)
		return
	}
	if !inConv {
		writeJSONError(w, http.StatusForbidden, "not part of this conversation")
		return
	}

	msgs, err := fetchMessages(convID, limit, offset)
	if err != nil {
		logRequestError(w, "fetch messages", err)
		return
	}

	writeJSON(w, http.StatusOK, msgs)
}
