package main

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// runStoreContract checks the behaviour EVERY Store must have, no matter
// what is behind it. The same tests run against memory and against MongoDB.
func runStoreContract(t *testing.T, newStore func(t *testing.T) Store) {
	ctx := context.Background()

	t.Run("take returns the note once", func(t *testing.T) {
		s := newStore(t)
		want := Note{ID: "n1", Data: []byte("secret-bytes"), ExpiresAt: time.Now().Add(time.Hour)}
		if err := s.Save(ctx, want); err != nil {
			t.Fatal(err)
		}

		got, err := s.Take(ctx, "n1")
		if err != nil {
			t.Fatalf("first Take failed: %v", err)
		}
		if got.ID != want.ID || !bytes.Equal(got.Data, want.Data) {
			t.Errorf("got %+v, want %+v", got, want)
		}
		// databases may store time with less precision, so allow a small gap
		if gap := got.ExpiresAt.Sub(want.ExpiresAt); gap < -time.Second || gap > time.Second {
			t.Errorf("expiry changed by %v", gap)
		}

		if _, err := s.Take(ctx, "n1"); !errors.Is(err, ErrNotFound) {
			t.Errorf("second Take: got %v, want ErrNotFound", err)
		}
		if _, err := s.Peek(ctx, "n1"); !errors.Is(err, ErrNotFound) {
			t.Errorf("Peek after Take: got %v, want ErrNotFound", err)
		}
	})

	t.Run("peek does not consume the note", func(t *testing.T) {
		s := newStore(t)
		note := Note{ID: "n2", Data: []byte("x"), ExpiresAt: time.Now().Add(time.Hour)}
		if err := s.Save(ctx, note); err != nil {
			t.Fatal(err)
		}

		for i := 1; i <= 3; i++ {
			if _, err := s.Peek(ctx, "n2"); err != nil {
				t.Fatalf("Peek #%d failed: %v", i, err)
			}
		}
		if _, err := s.Take(ctx, "n2"); err != nil {
			t.Errorf("Take after Peek failed: %v", err)
		}
	})

	t.Run("expired notes are not returned", func(t *testing.T) {
		s := newStore(t)
		past := time.Now().Add(-time.Minute)
		for _, id := range []string{"old-peek", "old-take"} {
			if err := s.Save(ctx, Note{ID: id, Data: []byte("x"), ExpiresAt: past}); err != nil {
				t.Fatal(err)
			}
		}

		if _, err := s.Peek(ctx, "old-peek"); !errors.Is(err, ErrNotFound) {
			t.Errorf("Peek expired: got %v, want ErrNotFound", err)
		}
		if _, err := s.Take(ctx, "old-take"); !errors.Is(err, ErrNotFound) {
			t.Errorf("Take expired: got %v, want ErrNotFound", err)
		}
	})

	t.Run("unknown notes are not found", func(t *testing.T) {
		s := newStore(t)

		if _, err := s.Peek(ctx, "missing"); !errors.Is(err, ErrNotFound) {
			t.Errorf("Peek: got %v, want ErrNotFound", err)
		}
		if _, err := s.Take(ctx, "missing"); !errors.Is(err, ErrNotFound) {
			t.Errorf("Take: got %v, want ErrNotFound", err)
		}
	})

	t.Run("many simultaneous takes: exactly one wins", func(t *testing.T) {
		s := newStore(t)
		note := Note{ID: "race", Data: []byte("x"), ExpiresAt: time.Now().Add(time.Hour)}
		if err := s.Save(ctx, note); err != nil {
			t.Fatal(err)
		}

		const takers = 50
		var wg sync.WaitGroup
		var winners atomic.Int32

		for i := 0; i < takers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := s.Take(ctx, "race")
				switch {
				case err == nil:
					winners.Add(1)
				case !errors.Is(err, ErrNotFound):
					t.Errorf("unexpected error: %v", err)
				}
			}()
		}
		wg.Wait()

		if got := winners.Load(); got != 1 {
			t.Errorf("%d takers got the note, want exactly 1", got)
		}
	})
}

func TestMemoryStoreContract(t *testing.T) {
	runStoreContract(t, func(t *testing.T) Store {
		return NewMemoryStore()
	})
}

func TestMongoStoreContract(t *testing.T) {
	mongoTestURI(t) // skips the whole test when no database is configured

	runStoreContract(t, func(t *testing.T) Store {
		return newTestMongoStore(t)
	})
}