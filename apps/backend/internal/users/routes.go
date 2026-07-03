package users

import "net/http"

type routeRegistrar interface {
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

func RegisterPublicRoutes(mux routeRegistrar, handler Handler) {
	mux.HandleFunc("POST /users", handler.CreateUser)
	mux.HandleFunc("GET /users/{username}/followers", handler.ListFollowers)
	mux.HandleFunc("GET /users/{username}/following", handler.ListFollowing)
	mux.HandleFunc("GET /users/{username}", handler.GetUser)
}

// GET /users/me and GET /users/suggested are registered separately, directly
// on the public mux (see app/routes.go), because the public "GET
// /users/{username}" wildcard would otherwise shadow them.
func RegisterProtectedRoutes(mux routeRegistrar, handler Handler) {
	mux.HandleFunc("PUT /users/me", handler.UpdateUser)
	mux.HandleFunc("POST /users/{username}/follow", handler.FollowUser)
	mux.HandleFunc("DELETE /users/{username}/follow", handler.UnfollowUser)
}
