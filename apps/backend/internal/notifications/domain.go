package notifications

import "time"

type Notification struct {
	ID            int64     `json:"-"`
	PublicID      string    `json:"id"`
	ExternalID    string    `json:"-"`
	UserID        int64     `json:"-"`
	ActorID       int64     `json:"-"`
	ActorUsername string    `json:"actorUsername"`
	ActorName     string    `json:"actorName"`
	ActorAvatar   *string   `json:"actorAvatar"`
	Type          string    `json:"type"`
	EntityID      string    `json:"entityId"`
	Read          bool      `json:"read"`
	Created       time.Time `json:"created"`
}
