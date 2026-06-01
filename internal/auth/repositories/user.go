package repositories

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/ravilock/sentir-mais-backend/internal/auth"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

type UserRepository struct {
	collection *mongo.Collection
}

type userDocument struct {
	ID           string    `bson:"_id"`
	Email        string    `bson:"email"`
	PasswordHash string    `bson:"password_hash"`
	CreatedAt    time.Time `bson:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at"`
}

func NewUserRepository(ctx context.Context, database *mongo.Database) (*UserRepository, error) {
	repository := &UserRepository{collection: database.Collection("users")}
	if err := repository.ensureIndexes(ctx); err != nil {
		return nil, err
	}

	return repository, nil
}

func (r *UserRepository) Create(ctx context.Context, user domain.User) error {
	_, err := r.collection.InsertOne(ctx, userDocument{
		ID:           user.ID,
		Email:        strings.ToLower(user.Email),
		PasswordHash: user.PasswordHash,
		CreatedAt:    user.CreatedAt.UTC(),
		UpdatedAt:    user.UpdatedAt.UTC(),
	})
	if mongo.IsDuplicateKeyError(err) {
		return auth.ErrEmailAlreadyExists
	}

	return err
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	var document userDocument
	err := r.collection.FindOne(ctx, bson.D{{Key: "email", Value: strings.ToLower(email)}}).Decode(&document)
	if err == mongo.ErrNoDocuments {
		return domain.User{}, auth.ErrNotFound
	}
	if err != nil {
		return domain.User{}, err
	}

	return domain.User{
		ID:           document.ID,
		Email:        document.Email,
		PasswordHash: document.PasswordHash,
		CreatedAt:    document.CreatedAt.UTC(),
		UpdatedAt:    document.UpdatedAt.UTC(),
	}, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (domain.User, error) {
	var document userDocument
	err := r.collection.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&document)
	if err == mongo.ErrNoDocuments {
		return domain.User{}, auth.ErrNotFound
	}
	if err != nil {
		return domain.User{}, err
	}

	return domain.User{
		ID:           document.ID,
		Email:        document.Email,
		PasswordHash: document.PasswordHash,
		CreatedAt:    document.CreatedAt.UTC(),
		UpdatedAt:    document.UpdatedAt.UTC(),
	}, nil
}

func (r *UserRepository) ensureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}
