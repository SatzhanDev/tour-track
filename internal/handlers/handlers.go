// Package handlers serves the tour-track web pages.
package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tour-track/internal/store"
)

// Handler serves the site pages from an artist store.
type Handler struct {
	store  *store.Store
	tmpl   *template.Template
	static fs.FS
	now    func() time.Time // time source; tests replace it
}

// errorPage is the data passed to error.html.
type errorPage struct {
	Status  int
	Title   string
	Message string
}

// templateFuncs are helpers available inside templates.
var templateFuncs = template.FuncMap{
	"date":  func(t time.Time) string { return t.Format("02 Jan 2006") },
	"lower": strings.ToLower,
}

// New parses every *.html file in templates and returns a Handler that
// serves pages from s and static files from static.
func New(s *store.Store, templates, static fs.FS) (*Handler, error) {
	tmpl, err := template.New("").Funcs(templateFuncs).ParseFS(templates, "*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	return &Handler{store: s, tmpl: tmpl, static: static, now: time.Now}, nil
}

// Routes returns the site router wrapped in middleware.
//
// Methods are checked inside the handlers instead of in the patterns
// ("GET /..."): the catch-all "/" route matches every method, so the router
// would answer 404 instead of 405 for a wrong method.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/{$}", h.index)
	mux.HandleFunc("/artist/{id}", h.artist)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServerFS(h.static)))
	mux.HandleFunc("/", h.notFound)
	return h.recoverPanic(logRequests(mux))
}

// index lists every artist.
func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	if !h.allowGet(w, r) {
		return
	}
	if h.store.Len() == 0 {
		h.unavailable(w)
		return
	}
	h.render(w, http.StatusOK, "index.html", h.store.All(h.now().Year()))
}

// artist shows one artist by ID.
func (h *Handler) artist(w http.ResponseWriter, r *http.Request) {
	if !h.allowGet(w, r) {
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		h.renderError(w, http.StatusBadRequest, "The artist ID must be a positive number.")
		return
	}

	a, ok := h.store.ByID(id, h.now().Year())
	if !ok {
		if h.store.Len() == 0 {
			h.unavailable(w)
			return
		}
		h.renderError(w, http.StatusNotFound, "There is no artist with this ID.")
		return
	}
	h.render(w, http.StatusOK, "artist.html", a)
}

// notFound answers every path that matches no other route.
func (h *Handler) notFound(w http.ResponseWriter, r *http.Request) {
	h.renderError(w, http.StatusNotFound, "The page you are looking for does not exist.")
}

// unavailable is used while the artist data has not been loaded yet.
func (h *Handler) unavailable(w http.ResponseWriter) {
	h.renderError(w, http.StatusServiceUnavailable,
		"Artist data is not available yet. Please try again in a moment.")
}

// allowGet reports whether the request method is GET or HEAD. Otherwise it
// writes a 405 response and returns false.
func (h *Handler) allowGet(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return true
	}
	w.Header().Set("Allow", "GET, HEAD")
	h.renderError(w, http.StatusMethodNotAllowed, "This method is not allowed here.")
	return false
}

// render executes the named template into a buffer first, so a template
// error never sends a half-written page with status 200.
func (h *Handler) render(w http.ResponseWriter, status int, name string, data any) {
	var buf bytes.Buffer
	if err := h.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
		h.renderError(w, http.StatusInternalServerError, "Something went wrong on our side.")
		return
	}
	writeHTML(w, status, &buf)
}

// renderError shows error.html. If that template fails too, it falls back
// to a plain-text response.
func (h *Handler) renderError(w http.ResponseWriter, status int, message string) {
	data := errorPage{Status: status, Title: http.StatusText(status), Message: message}

	var buf bytes.Buffer
	if err := h.tmpl.ExecuteTemplate(&buf, "error.html", data); err != nil {
		log.Printf("render error.html: %v", err)
		http.Error(w, http.StatusText(status), status)
		return
	}
	writeHTML(w, status, &buf)
}

// writeHTML sends an HTML body with the given status.
func writeHTML(w http.ResponseWriter, status int, body *bytes.Buffer) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := body.WriteTo(w); err != nil {
		log.Printf("write response: %v", err)
	}
}
