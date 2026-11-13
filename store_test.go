package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTakeDeletesNote(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	note := Note{ID: "n1", ExpiresAt: time.Now().Add(time.Hour)}
	if err := s.Save(ctx, note); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Take(ctx, "n1"); err != nil {
		t.Fatalf("first Take failed: %v", err)
	}
	if _, err := s.Take(ctx, "n1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Take: got error %v, want ErrNotFound", err)
	}
}

func TestPeekDoesNotDeleteNote(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	note := Note{ID: "n1", ExpiresAt: time.Now().Add(time.Hour)}
	if err := s.Save(ctx, note); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		if _, err := s.Peek(ctx, "n1"); err != nil {
			t.Fatalf("Peek #%d failed: %v", i+1, err)
		}
	}
}

func TestDeleteExpired(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	old := Note{ID: "old", ExpiresAt: time.Now().Add(-time.Minute)}
	fresh := Note{ID: "fresh", ExpiresAt: time.Now().Add(time.Hour)}
	for _, n := range []Note{old, fresh} {
		if err := s.Save(ctx, n); err != nil {
			t.Fatal(err)
		}
	}

	if removed := s.deleteExpired(); removed != 1 {
		t.Errorf("deleteExpired removed %d notes, want 1", removed)
	}

	if _, err := s.Peek(ctx, "fresh"); err != nil {
		t.Errorf("fresh note was removed: %v", err)
	}
	if _, err := s.Peek(ctx, "old"); !errors.Is(err, ErrNotFound) {
		t.Errorf("old note: got error %v, want ErrNotFound", err)
	}
}