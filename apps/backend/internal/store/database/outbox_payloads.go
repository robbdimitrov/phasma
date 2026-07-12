package database

import "encoding/json"

type EntityUserUpsertPayload struct {
	Table    string `json:"table"`
	Op       string `json:"op"`
	ID       int64  `json:"id"`
	UserID   string `json:"user_id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Bio      string `json:"bio"`
	Avatar   string `json:"avatar"`
}

type EntityPostUpsertPayload struct {
	Table       string   `json:"table"`
	Op          string   `json:"op"`
	ID          int64    `json:"id"`
	PostID      string   `json:"post_id"`
	AuthorID    string   `json:"author_id"`
	Description string   `json:"description"`
	Username    string   `json:"username"`
	Hashtags    []string `json:"hashtags"`
	Created     string   `json:"created"`
	IsCelebrity bool     `json:"is_celebrity"`
	Filename    string   `json:"filename"`
}

type EntityPostDeletePayload struct {
	Table            string   `json:"table"`
	Op               string   `json:"op"`
	ID               int64    `json:"id"`
	PostID           string   `json:"post_id"`
	AuthorID         string   `json:"author_id"`
	Filename         string   `json:"filename"`
	CommentPublicIDs []string `json:"comment_public_ids"`
}

type EntityHashtagUpsertPayload struct {
	Table     string `json:"table"`
	Op        string `json:"op"`
	Name      string `json:"name"`
	PostCount int    `json:"post_count"`
}

type ActivityPayload struct {
	Op          string `json:"op"`
	PostID      string `json:"post_id,omitempty"`
	CommentID   string `json:"comment_id,omitempty"`
	ActorID     string `json:"actor_id"`
	RecipientID string `json:"recipient_id,omitempty"`
}

func MarshalOutboxPayload(payload any) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
