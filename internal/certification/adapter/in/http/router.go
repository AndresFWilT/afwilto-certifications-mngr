package http

import "net/http"

// RouterDeps carries everything the HTTP adapter needs, injected from the
// composition root. Use-case ports are added slice by slice.
type RouterDeps struct {
	Pinger       Pinger
	AllowOrigins []string
}

func NewRouter(deps RouterDeps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", handleHealthz())
	mux.HandleFunc("GET /readyz", handleReadyz(deps.Pinger))

	var handler http.Handler = mux
	handler = corsMiddleware(deps.AllowOrigins, handler)
	handler = recoverMiddleware(handler)
	handler = requestLogMiddleware(handler)
	return handler
}
