package search

type UserResult struct {
	Username string  `json:"username"`
	Name     string  `json:"name"`
	Avatar   *string `json:"avatar"`
}

type HashtagResult struct {
	Name      string `json:"name"`
	PostCount int    `json:"postCount"`
}

// RecentSearchItem mirrors BlendedItem's {"type", "item"} wire shape (see
// blend.go) so the frontend reuses the same discriminated-union pattern.
type RecentSearchItem struct {
	ID   string `json:"id"`
	Type string `json:"type"` // "users" | "hashtags" | "posts"
	Item any    `json:"item"` // UserResult | HashtagResult | string
}

type PostResult struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Description string `json:"description"`
	Filename    string `json:"filename"`
}
