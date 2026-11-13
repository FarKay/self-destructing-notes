package main

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// compile-time check: MongoStore must satisfy the Store interface
var _ Store = (*MongoStore)(nil)

// noteDoc is how a note looks inside MongoDB. The note ID is used as
// the document's _id, which MongoDB already keeps unique.
type noteDoc struct {
	ID        string    `bson:"_id"`
	Data      []byte    `bson:"data"` // nonce + encrypted text, never the plain text
	ExpiresAt time.Time `bson:"expires_at"`
}

func (d noteDoc) toNote() Note {
	return Note{ID: d.ID, Data: d.Data, ExpiresAt: d.ExpiresAt}
}

// MongoStore keeps notes in a MongoDB collection, so they survive restarts.
type MongoStore struct {
	client *mongo.Client
	coll   *mongo.Collection
}

func NewMongoStore(uri, dbName, collName string) (*MongoStore, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	// Connect does not prove the server is reachable, Ping does.
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	coll := client.Database(dbName).Collection(collName)

	// TTL index: MongoDB deletes each note itself once its expires_at time
	// has passed (0 seconds after that time). MongoDB checks about once a
	// minute, so an expired note can linger briefly. That is why Peek and
	// Take still compare the time themselves.
	_, err = coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "expires_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(0),
	})
	if err != nil {
		return nil, err
	}

	return &MongoStore{client: client, coll: coll}, nil
}

func (s *MongoStore) Save(ctx context.Context, note Note) error {
	_, err := s.coll.InsertOne(ctx, noteDoc{
		ID:        note.ID,
		Data:      note.Data,
		ExpiresAt: note.ExpiresAt,
	})
	return err
}

func (s *MongoStore) Peek(ctx context.Context, id string) (Note, error) {
	var doc noteDoc
	err := s.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Note{}, ErrNotFound
	}
	if err != nil {
		return Note{}, err
	}

	if time.Now().After(doc.ExpiresAt) {
		return Note{}, ErrNotFound // expired; the TTL index will remove it
	}
	return doc.toNote(), nil
}

// Take finds the note AND deletes it in one operation inside the database.
// If many requests arrive at once, MongoDB hands the document to exactly
// one of them and the rest get ErrNotFound. This replaces the mutex.
func (s *MongoStore) Take(ctx context.Context, id string) (Note, error) {
	var doc noteDoc
	err := s.coll.FindOneAndDelete(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Note{}, ErrNotFound
	}
	if err != nil {
		return Note{}, err
	}

	// it is deleted either way, but an expired note must not be returned
	if time.Now().After(doc.ExpiresAt) {
		return Note{}, ErrNotFound
	}
	return doc.toNote(), nil
}