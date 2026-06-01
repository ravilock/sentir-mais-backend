package repositories

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/ravilock/sentir-mais-backend/internal/chat"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

type ChatRepository struct {
	collection *mongo.Collection
}

type chatDocument struct {
	ID        string    `bson:"_id"`
	UserID    string    `bson:"user_id"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

func NewChatRepository(ctx context.Context, database *mongo.Database) (*ChatRepository, error) {
	repository := &ChatRepository{collection: database.Collection("chats")}
	if err := repository.ensureIndexes(ctx); err != nil {
		return nil, err
	}

	return repository, nil
}

func (r *ChatRepository) Create(ctx context.Context, chatRecord domain.Chat) error {
	_, err := r.collection.InsertOne(ctx, chatDocument{
		ID:        chatRecord.ID,
		UserID:    chatRecord.UserID,
		CreatedAt: chatRecord.CreatedAt.UTC(),
		UpdatedAt: chatRecord.UpdatedAt.UTC(),
	})
	return err
}

func (r *ChatRepository) FindByID(ctx context.Context, id string) (domain.Chat, error) {
	var document chatDocument
	err := r.collection.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&document)
	if err == mongo.ErrNoDocuments {
		return domain.Chat{}, chat.ErrChatNotFound
	}
	if err != nil {
		return domain.Chat{}, err
	}

	return domain.Chat{
		ID:        document.ID,
		UserID:    document.UserID,
		CreatedAt: document.CreatedAt.UTC(),
		UpdatedAt: document.UpdatedAt.UTC(),
	}, nil
}

func (r *ChatRepository) Update(ctx context.Context, chatRecord domain.Chat) error {
	result, err := r.collection.ReplaceOne(
		ctx,
		bson.D{{Key: "_id", Value: chatRecord.ID}},
		chatDocument{
			ID:        chatRecord.ID,
			UserID:    chatRecord.UserID,
			CreatedAt: chatRecord.CreatedAt.UTC(),
			UpdatedAt: chatRecord.UpdatedAt.UTC(),
		},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return chat.ErrChatNotFound
	}

	return nil
}

func (r *ChatRepository) ensureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}},
			Options: options.Index(),
		},
		{
			Keys:    bson.D{{Key: "updated_at", Value: -1}},
			Options: options.Index(),
		},
	})
	return err
}
