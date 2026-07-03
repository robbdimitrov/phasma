package search

import "net/http"

type routeRegistrar interface {
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

// GET /users/search is registered separately, directly on the public mux
// (see app/routes.go), because the public "GET /users/{username}" wildcard
// would otherwise shadow it.
func RegisterRoutes(mux routeRegistrar, handler Handler) {
	mux.HandleFunc("GET /hashtags/search", handler.SearchHashtags)
	mux.HandleFunc("GET /search", handler.Search)
}
