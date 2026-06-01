package repositories

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/ravilock/sentir-mais-backend/internal/auth"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

type SessionRepository struct {
	collection *mongo.Collection
}

type sessionDocument struct {
	ID        string    `bson:"_id"`
	UserID    string    `bson:"user_id"`
	ExpiresAt time.Time `bson:"expires_at"`
	CreatedAt time.Time `bson:"created_at"`
}

func NewSessionRepository(ctx context.Context, database *mongo.Database) (*SessionRepository, error) {
	repository := &SessionRepository{collection: database.Collection("sessions")}
	if err := repository.ensureIndexes(ctx); err != nil {
		return nil, err
	}

	return repository, nil
}

func (r *SessionRepository) Save(ctx context.Context, session domain.Session) error {
	_, err := r.collection.ReplaceOne(
		ctx,
		bson.D{{Key: "_id", Value: session.Token}},
		sessionDocument{
			ID:        session.Token,
			UserID:    session.UserID,
			ExpiresAt: session.ExpiresAt.UTC(),
			CreatedAt: session.CreatedAt.UTC(),
		},
		options.Replace().SetUpsert(true),
	)
	return err
}

func (r *SessionRepository) FindByToken(ctx context.Context, token string) (domain.Session, error) {
	var document sessionDocument
	err := r.collection.FindOne(ctx, bson.D{{Key: "_id", Value: token}}).Decode(&document)
	if err == mongo.ErrNoDocuments {
		return domain.Session{}, auth.ErrNotFound
	}
	if err != nil {
		return domain.Session{}, err
	}

	return domain.Session{
		Token:     document.ID,
		UserID:    document.UserID,
		ExpiresAt: document.ExpiresAt.UTC(),
		CreatedAt: document.CreatedAt.UTC(),
	}, nil
}

func (r *SessionRepository) ensureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}},
			Options: options.Index(),
		},
		{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
	})
	return err
}
