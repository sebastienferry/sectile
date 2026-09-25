package main

import "net/http"

// routeOf names the controller a request reaches: the mux pattern, which is a
// bounded set, rather than the path, which carries ids.
func routeOf(mux *http.ServeMux) func(*http.Request) string {
	return func(r *http.Request) string {
		_, pattern := mux.Handler(r)
		return pattern
	}
}
