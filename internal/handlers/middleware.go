package handlers

import (
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

// recoverPanic turns a panic in any handler into a 500 error page, so one
// broken request never takes the server down or leaves the client without
// an answer.
func (h *Handler) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %v\n%s", err, debug.Stack())
				w.Header().Set("Connection", "close")
				h.renderError(w, http.StatusInternalServerError, "Something went wrong on our side.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// statusRecorder remembers the status code written by a handler.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(status int) {
	rec.status = status
	rec.ResponseWriter.WriteHeader(status)
}

// logRequests logs the method, path, status and duration of every request.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}
