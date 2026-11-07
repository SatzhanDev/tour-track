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

	"tour-track/internal/activity"
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

// indexPage is the data passed to index.html.
type indexPage struct {
	Artists []store.ArtistView // artists that match the filters
	Total   int                // number of artists before filtering
	Filter  store.Filter       // to keep the form filled in
	Labels  []labelOption      // every label, for the filter menu
}

// labelOption is one entry of the label filter menu.
type labelOption struct {
	Name  string
	Count int // artists with this label, before filtering
}

// labelOptions counts the artists per label, so the menu shows what each
// filter can find (for example "Emerging (0)").
func labelOptions(all []store.ArtistView) []labelOption {
	counts := make(map[string]int)
	for _, a := range all {
		counts[a.Label]++
	}
	var labels []labelOption
	for _, name := range activity.Labels() {
		labels = append(labels, labelOption{Name: name, Count: counts[name]})
	}
	return labels
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
	mux.HandleFunc("/api/artists", h.apiArtists)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServerFS(h.static)))
	mux.HandleFunc("/", h.notFound)
	return h.recoverPanic(logRequests(secureHeaders(mux)))
}

// index lists the artists that match the q, label and rising query
// parameters.
// The same filters are used by the live search in app.js, so the page also
// works without JavaScript.
func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	if !h.allowGet(w, r) {
		return
	}
	if h.store.Len() == 0 {
		h.unavailable(w)
		return
	}
	filter, ok := parseFilter(r)
	if !ok {
		h.renderError(w, http.StatusBadRequest, "Unknown activity label.")
		return
	}

	all := h.store.All(h.now())
	h.render(w, http.StatusOK, "index.html", indexPage{
		Artists: store.Search(all, filter),
		Total:   len(all),
		Filter:  filter,
		Labels:  labelOptions(all),
	})
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

	a, ok := h.store.ByID(id, h.now())
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

// parseFilter reads the q, label and rising query parameters. The label is
// returned in its canonical spelling ("active" becomes "Active"); ok is
// false when it is not a known label. An empty label means "any label".
// rising=1 keeps only Rising Stars; any other value is ignored.
func parseFilter(r *http.Request) (f store.Filter, ok bool) {
	params := r.URL.Query()
	f.Query = strings.TrimSpace(params.Get("q"))
	f.RisingOnly = params.Get("rising") == "1"

	raw := strings.TrimSpace(params.Get("label"))
	if raw == "" {
		return f, true
	}
	for _, l := range activity.Labels() {
		if strings.EqualFold(l, raw) {
			f.Label = l
			return f, true
		}
	}
	return f, false
}

// isReadMethod reports whether the request only reads data (GET or HEAD).
func isReadMethod(r *http.Request) bool {
	return r.Method == http.MethodGet || r.Method == http.MethodHead
}

// allowGet reports whether the request method is GET or HEAD. Otherwise it
// writes a 405 response and returns false.
func (h *Handler) allowGet(w http.ResponseWriter, r *http.Request) bool {
	if isReadMethod(r) {
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
	send(w, status, "text/html; charset=utf-8", &buf)
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
	send(w, status, "text/html; charset=utf-8", &buf)
}

// send writes a ready response body with the given status and type.
func send(w http.ResponseWriter, status int, contentType string, body *bytes.Buffer) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	if _, err := body.WriteTo(w); err != nil {
		log.Printf("write response: %v", err)
	}
}
