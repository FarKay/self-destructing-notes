package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// mongoTestURI returns the connection string for the MongoDB tests.
// Without MONGO_TEST_URI the tests are skipped, so plain "go test"
// still works on a machine with no database.
func mongoTestURI(t *testing.T) string {
	t.Helper()
	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		t.Skip("MONGO_TEST_URI is not set; skipping MongoDB tests")
	}
	return uri
}

// newTestMongoStore connects to a brand new collection in a test database
// and deletes it when the test ends.
func newTestMongoStore(t *testing.T) *MongoStore {
	t.Helper()
	uri := mongoTestURI(t)

	collName := fmt.Sprintf("test_notes_%d", time.Now().UnixNano())
	store, err := NewMongoStore(uri, "selfdestructing_notes_test", collName)
	if err != nil {
		t.Fatalf("could not connect to MongoDB: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		store.coll.Drop(ctx)
	})
	return store
}

func TestMongoStoreCreatesTTLIndex(t *testing.T) {
	store := newTestMongoStore(t)
	ctx := context.Background()

	cursor, err := store.coll.Indexes().List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var indexes []bson.M
	if err := cursor.All(ctx, &indexes); err != nil {
		t.Fatal(err)
	}

	for _, index := range indexes {
		if _, ok := index["expireAfterSeconds"]; ok {
			return // found the TTL index
		}
	}
	t.Error("no TTL index found on the notes collection")
}