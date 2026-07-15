package users

import "net/http"

type routeRegistrar interface {
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

func RegisterPublicRoutes(mux routeRegistrar, handler Handler) {
	mux.HandleFunc("POST /users", handler.CreateUser)
	mux.HandleFunc("GET /users/{username}", handler.GetUser)
}

// Public-prefix authenticated reads are registered separately on the public mux
// (see app/routes.go) so they outrank broader public wildcard routes.
func RegisterProtectedRoutes(mux routeRegistrar, handler Handler) {
	mux.HandleFunc("PUT /users/me", handler.UpdateUser)
	mux.HandleFunc("POST /users/{username}/follow", handler.FollowUser)
	mux.HandleFunc("DELETE /users/{username}/follow", handler.UnfollowUser)
}
