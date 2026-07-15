package comments

import "net/http"

type routeRegistrar interface {
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

func RegisterPublicRoutes(routeRegistrar, Handler) {}

func RegisterProtectedRoutes(mux routeRegistrar, handler Handler) {
	mux.HandleFunc("GET /posts/{publicId}/comments", handler.ListComments)
	mux.HandleFunc("POST /posts/{publicId}/comments", handler.CreateComment)
	mux.HandleFunc("DELETE /posts/{publicId}/comments/{commentId}", handler.DeleteComment)
}
