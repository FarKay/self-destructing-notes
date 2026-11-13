package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newTestApp builds an app with a fresh in-memory store and a fixed key.
func newTestApp(t *testing.T) *app {
	t.Helper()
	return &app{
		store:   NewMemoryStore(),
		key:     bytes.Repeat([]byte("k"), 32),
		baseURL: "http://test",
	}
}

// send runs one request through the handler without starting a real server.
func send(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// createTestNote creates a note through the API and returns its id.
func createTestNote(t *testing.T, h http.Handler, text string) string {
	t.Helper()

	body, err := json.Marshal(map[string]any{"text": text, "expires_in_minutes": 10})
	if err != nil {
		t.Fatal(err)
	}

	rec := send(h, "POST", "/notes", string(body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: got status %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("create: could not read response: %v", err)
	}
	if resp.ID == "" {
		t.Fatal("create: response has no id")
	}
	return resp.ID
}

func TestCreateAndReadOnce(t *testing.T) {
	h := newTestApp(t).routes()
	id := createTestNote(t, h, "my secret")

	// checking a note must not consume it, even several times
	for i := 1; i <= 2; i++ {
		rec := send(h, "GET", "/notes/"+id, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("check #%d: got %d, want 200", i, rec.Code)
		}
	}

	// the first read returns the text
	rec := send(h, "POST", "/notes/"+id+"/read", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("first read: got %d, want 200", rec.Code)
	}
	var resp struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Text != "my secret" {
		t.Errorf("first read: got text %q, want %q", resp.Text, "my secret")
	}

	// the note is now gone
	rec = send(h, "POST", "/notes/"+id+"/read", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("second read: got %d, want 404", rec.Code)
	}
	rec = send(h, "GET", "/notes/"+id, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("check after read: got %d, want 404", rec.Code)
	}
}

func TestCreateValidation(t *testing.T) {
	h := newTestApp(t).routes()

	tests := []struct {
		name string
		body string
	}{
		{"invalid JSON", `{not json`},
		{"empty text", `{"text":""}`},
		{"blank text", `{"text":"   "}`},
		{"text too long", `{"text":"` + strings.Repeat("a", maxTextBytes+1) + `"}`},
		{"expiry too long", `{"text":"hi","expires_in_minutes":99999}`},
		{"negative expiry", `{"text":"hi","expires_in_minutes":-5}`},
		{"body too large", `{"text":"` + strings.Repeat("a", maxBodyBytes) + `"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := send(h, "POST", "/notes", tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("got status %d, want 400", rec.Code)
			}
		})
	}
}

func TestUnknownNote(t *testing.T) {
	h := newTestApp(t).routes()

	if rec := send(h, "GET", "/notes/doesnotexist", ""); rec.Code != http.StatusNotFound {
		t.Errorf("check: got %d, want 404", rec.Code)
	}
	if rec := send(h, "POST", "/notes/doesnotexist/read", ""); rec.Code != http.StatusNotFound {
		t.Errorf("read: got %d, want 404", rec.Code)
	}
}

func TestExpiredNotesAreRejected(t *testing.T) {
	a := newTestApp(t)
	h := a.routes()

	data, err := encrypt(a.key, "old news")
	if err != nil {
		t.Fatal(err)
	}

	// two already-expired notes: one is checked, the other is read
	expired := time.Now().Add(-time.Minute)
	for _, id := range []string{"expired-check", "expired-read"} {
		err := a.store.Save(context.Background(), Note{ID: id, Data: data, ExpiresAt: expired})
		if err != nil {
			t.Fatal(err)
		}
	}

	if rec := send(h, "GET", "/notes/expired-check", ""); rec.Code != http.StatusNotFound {
		t.Errorf("check expired: got %d, want 404", rec.Code)
	}
	if rec := send(h, "POST", "/notes/expired-read/read", ""); rec.Code != http.StatusNotFound {
		t.Errorf("read expired: got %d, want 404", rec.Code)
	}
}

func TestNoteIsEncryptedAtRest(t *testing.T) {
	a := newTestApp(t)
	h := a.routes()

	const secret = "this sentence is easy to spot in raw data"
	id := createTestNote(t, h, secret)

	// look at what the store actually holds
	note, err := a.store.Peek(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(note.Data, []byte(secret)) {
		t.Error("the note text is stored in plain text")
	}
}

// Many people open the same link at the same moment: exactly one may win.
func TestConcurrentReadsOnlyOneSucceeds(t *testing.T) {
	h := newTestApp(t).routes()
	id := createTestNote(t, h, "read me once")

	const readers = 100
	var wg sync.WaitGroup
	var successes atomic.Int32

	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := send(h, "POST", "/notes/"+id+"/read", "")
			if rec.Code == http.StatusOK {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := successes.Load(); got != 1 {
		t.Errorf("%d readers got the note, want exactly 1", got)
	}
}