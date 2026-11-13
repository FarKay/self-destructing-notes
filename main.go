package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	maxBodyBytes         = 16 * 1024 // reject request bodies bigger than this
	maxTextBytes         = 10_000    // longest allowed note
	defaultExpiryMinutes = 60
	maxExpiryMinutes     = 7 * 24 * 60 // one week
)

// The web folder is built into the program, so deploying needs only one file.
//
//go:embed web
var webFiles embed.FS

type createRequest struct {
	Text             string `json:"text"`
	ExpiresInMinutes int    `json:"expires_in_minutes"` // optional
}

// app holds what the handlers share.
type app struct {
	store   Store
	key     []byte // encryption key, never stored with the notes
	baseURL string
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store") // responses can contain secrets
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func serverError(w http.ResponseWriter, err error) {
	log.Println("error:", err) // details stay in the server log
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "something went wrong"})
}

// storeError maps "not found" to 404 and anything else to 500.
// Missing, expired and already-read notes all look the same on purpose.
func storeError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "note not found, expired, or already read"})
		return
	}
	serverError(w, err)
}

// POST /notes  {"text": "...", "expires_in_minutes": 60}
func (a *app) createNote(w http.ResponseWriter, r *http.Request) {
	// stop clients from sending huge bodies
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON, or body too large"})
		return
	}

	if strings.TrimSpace(req.Text) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "text is required"})
		return
	}
	if len(req.Text) > maxTextBytes {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "text is too long"})
		return
	}

	minutes := req.ExpiresInMinutes
	if minutes == 0 {
		minutes = defaultExpiryMinutes
	}
	if minutes < 1 || minutes > maxExpiryMinutes {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expires_in_minutes must be between 1 and 10080"})
		return
	}

	data, err := encrypt(a.key, req.Text)
	if err != nil {
		serverError(w, err)
		return
	}
	id, err := newID()
	if err != nil {
		serverError(w, err)
		return
	}

	note := Note{
		ID:        id,
		Data:      data,
		ExpiresAt: time.Now().Add(time.Duration(minutes) * time.Minute),
	}
	if err := a.store.Save(r.Context(), note); err != nil {
		serverError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         id,
		"url":        a.baseURL + "/n/" + id, // the page people open in a browser
		"expires_at": note.ExpiresAt,
	})
}

// GET /notes/{id}  checks that a note exists. It does NOT consume it.
// Chat apps and link previews fetch links automatically, so a GET
// that destroyed the note would burn it before the real person saw it.
func (a *app) checkNote(w http.ResponseWriter, r *http.Request) {
	note, err := a.store.Peek(r.Context(), r.PathValue("id"))
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"exists":     true,
		"expires_at": note.ExpiresAt,
	})
}

// POST /notes/{id}/read  returns the text and destroys the note.
func (a *app) readNote(w http.ResponseWriter, r *http.Request) {
	note, err := a.store.Take(r.Context(), r.PathValue("id"))
	if err != nil {
		storeError(w, err)
		return
	}

	text, err := decrypt(a.key, note.Data)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"text": text})
}

// setPageHeaders adds security headers to the HTML pages.
func setPageHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	// only scripts and styles from our own files may run: no inline code,
	// no other sites, and the page cannot be put inside a frame
	h.Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer") // the note link must not leak to other sites
	h.Set("Cache-Control", "no-store")
}

// pageHandler serves one embedded HTML file.
func pageHandler(name string) http.HandlerFunc {
	content, err := webFiles.ReadFile("web/" + name)
	if err != nil {
		panic(err) // the file is embedded at build time, so this should never happen
	}
	return func(w http.ResponseWriter, r *http.Request) {
		setPageHeaders(w)
		w.Write(content)
	}
}

// routes builds the router. Keeping it in its own function lets the
// tests use exactly the same routing as the real server.
func (a *app) routes() http.Handler {
	// Go 1.22+ routing: method and path variables in the pattern
	mux := http.NewServeMux()
 
	// One React app serves both pages. It reads the address in the browser
	// and shows either the create form or the note page. Opening /n/{id}
	// only returns this page: it never touches the note, so link previews
	// cannot destroy it.
	mux.HandleFunc("GET /{$}", pageHandler("index.html"))
	mux.HandleFunc("GET /n/{id}", pageHandler("index.html"))
 
	// the built JavaScript and CSS (created by "npm run build")
	assets, err := fs.Sub(webFiles, "web/assets")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServerFS(assets)))
 
	// the API
	mux.HandleFunc("POST /notes", a.createNote)
	mux.HandleFunc("GET /notes/{id}", a.checkNote)
	mux.HandleFunc("POST /notes/{id}/read", a.readNote)
	return mux
}

func main() {
	key, err := parseKey(os.Getenv("NOTES_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	// MongoDB if MONGO_URI is set, otherwise memory
	var store Store
	if uri := os.Getenv("MONGO_URI"); uri != "" {
		mongoStore, err := NewMongoStore(uri, "selfdestructing_notes", "notes")
		if err != nil {
			log.Fatal("could not connect to MongoDB: ", err)
		}
		store = mongoStore
		fmt.Println("Using MongoDB")
	} else {
		memoryStore := NewMemoryStore()
		go memoryStore.CleanupLoop(time.Minute) // background goroutine removes expired notes
		store = memoryStore
		fmt.Println("MONGO_URI not set, using the in-memory store (notes are lost on restart)")
	}

	// store := NewMemoryStore()
	// go store.CleanupLoop(time.Minute) // background goroutine removes expired notes

	port := os.Getenv("PORT")
	if port == "" {
		port = "4000"
	}
	baseURL := strings.TrimRight(os.Getenv("BASE_URL"), "/")
	if baseURL == "" {
		baseURL = "http://localhost:" + port
	}

	a := &app{store: store, key: key, baseURL: baseURL}

	fmt.Println("Self-destructing notes running on port", port)
	log.Fatal(http.ListenAndServe(":"+port, a.routes()))
}