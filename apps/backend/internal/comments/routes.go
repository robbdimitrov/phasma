package comments

import "net/http"

type routeRegistrar interface {
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

func RegisterPublicRoutes(mux routeRegistrar, handler Handler) {
	mux.HandleFunc("GET /posts/{publicId}/comments", handler.ListComments)
}

func RegisterProtectedRoutes(mux routeRegistrar, handler Handler) {
	mux.HandleFunc("POST /posts/{publicId}/comments", handler.CreateComment)
	mux.HandleFunc("DELETE /posts/{publicId}/comments/{commentId}", handler.DeleteComment)
}
