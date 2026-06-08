package repositories

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

type MessageRepository struct {
	collection *mongo.Collection
}

type messageDocument struct {
	ID        string    `bson:"_id"`
	ChatID    string    `bson:"chat_id"`
	UserID    string    `bson:"user_id"`
	Sender    int       `bson:"sender"`
	Content   string    `bson:"content"`
	CreatedAt time.Time `bson:"created_at"`
}

func NewMessageRepository(ctx context.Context, database *mongo.Database) (*MessageRepository, error) {
	repository := &MessageRepository{collection: database.Collection("messages")}
	if err := repository.ensureIndexes(ctx); err != nil {
		return nil, err
	}

	return repository, nil
}

func (r *MessageRepository) Create(ctx context.Context, message domain.Message) error {
	_, err := r.collection.InsertOne(ctx, messageDocument{
		ID:        message.ID,
		ChatID:    message.ChatID,
		UserID:    message.UserID,
		Sender:    int(message.Sender),
		Content:   message.Content,
		CreatedAt: message.CreatedAt.UTC(),
	})
	return err
}

func (r *MessageRepository) ListByChatID(ctx context.Context, chatID string) ([]domain.Message, error) {
	cursor, err := r.collection.Find(
		ctx,
		bson.D{{Key: "chat_id", Value: chatID}},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	documents := make([]messageDocument, 0)
	if err := cursor.All(ctx, &documents); err != nil {
		return nil, err
	}

	messages := make([]domain.Message, 0, len(documents))
	for _, document := range documents {
		messages = append(messages, domain.Message{
			ID:        document.ID,
			ChatID:    document.ChatID,
			UserID:    document.UserID,
			Sender:    domain.Sender(document.Sender),
			Content:   document.Content,
			CreatedAt: document.CreatedAt.UTC(),
		})
	}

	return messages, nil
}

func (r *MessageRepository) FindLatestByChatID(ctx context.Context, chatID string) (domain.Message, error) {
	var document messageDocument
	err := r.collection.FindOne(
		ctx,
		bson.D{{Key: "chat_id", Value: chatID}},
		options.FindOne().SetSort(bson.D{{Key: "created_at", Value: -1}}),
	).Decode(&document)
	if err == mongo.ErrNoDocuments {
		return domain.Message{}, nil
	}
	if err != nil {
		return domain.Message{}, err
	}

	return domain.Message{
		ID:        document.ID,
		ChatID:    document.ChatID,
		UserID:    document.UserID,
		Sender:    domain.Sender(document.Sender),
		Content:   document.Content,
		CreatedAt: document.CreatedAt.UTC(),
	}, nil
}

func (r *MessageRepository) ensureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "chat_id", Value: 1}, {Key: "created_at", Value: 1}},
			Options: options.Index(),
		},
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}},
			Options: options.Index(),
		},
	})
	return err
}
