package main

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

var ErrNotFound = errors.New("note not found")

// Note is what gets stored. The text is only ever kept encrypted.
type Note struct {
	ID        string
	Data      []byte // nonce + encrypted text
	ExpiresAt time.Time
}

// Store is what the handlers need from a database.
type Store interface {
	// Save stores a new note.
	Save(ctx context.Context, note Note) error
	// Peek checks that a note exists and has not expired, without consuming it.
	Peek(ctx context.Context, id string) (Note, error)
	// Take returns the note AND deletes it in one step, so it can be read once.
	Take(ctx context.Context, id string) (Note, error)
}

// MemoryStore keeps notes in a map. Everything is lost on restart.
type MemoryStore struct {
	mu    sync.Mutex
	notes map[string]Note
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{notes: make(map[string]Note)}
}

func (s *MemoryStore) Save(ctx context.Context, note Note) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.notes[note.ID] = note
	return nil
}

func (s *MemoryStore) Peek(ctx context.Context, id string) (Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	note, ok := s.notes[id]
	if !ok {
		return Note{}, ErrNotFound
	}
	if time.Now().After(note.ExpiresAt) {
		delete(s.notes, id) // expired: remove it now
		return Note{}, ErrNotFound
	}
	return note, nil
}

func (s *MemoryStore) Take(ctx context.Context, id string) (Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	note, ok := s.notes[id]
	if !ok {
		return Note{}, ErrNotFound
	}

	// delete first: whatever happens next, this note can never be read again
	delete(s.notes, id)

	if time.Now().After(note.ExpiresAt) {
		return Note{}, ErrNotFound
	}
	return note, nil
}

// deleteExpired removes every expired note and returns how many it removed.
func (s *MemoryStore) deleteExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := 0
	now := time.Now()
	for id, note := range s.notes {
		if now.After(note.ExpiresAt) {
			delete(s.notes, id)
			removed++
		}
	}
	return removed
}

// CleanupLoop runs forever, removing expired notes on a timer.
// Start it in its own goroutine: go store.CleanupLoop(time.Minute)
func (s *MemoryStore) CleanupLoop(every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for range ticker.C {
		if n := s.deleteExpired(); n > 0 {
			log.Printf("removed %d expired notes", n)
		}
	}
}