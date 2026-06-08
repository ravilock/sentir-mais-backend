package repositories

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

type DeadLetterRepository struct {
	collection *mongo.Collection
}

type deadLetterDocument struct {
	ID        string    `bson:"_id"`
	JobID     string    `bson:"job_id"`
	ChatID    string    `bson:"chat_id"`
	UserID    string    `bson:"user_id"`
	MessageID string    `bson:"message_id"`
	Stage     string    `bson:"stage"`
	Reason    string    `bson:"reason"`
	Error     string    `bson:"error"`
	Attempt   int       `bson:"attempt"`
	CreatedAt time.Time `bson:"created_at"`
}

func NewDeadLetterRepository(ctx context.Context, database *mongo.Database) (*DeadLetterRepository, error) {
	repository := &DeadLetterRepository{collection: database.Collection("analysis_dead_letters")}
	if err := repository.ensureIndexes(ctx); err != nil {
		return nil, err
	}

	return repository, nil
}

func (r *DeadLetterRepository) Create(ctx context.Context, deadLetter domain.AnalysisDeadLetter) error {
	_, err := r.collection.InsertOne(ctx, deadLetterDocument{
		ID:        deadLetter.ID,
		JobID:     deadLetter.JobID,
		ChatID:    deadLetter.ChatID,
		UserID:    deadLetter.UserID,
		MessageID: deadLetter.MessageID,
		Stage:     deadLetter.Stage,
		Reason:    deadLetter.Reason,
		Error:     deadLetter.Error,
		Attempt:   deadLetter.Attempt,
		CreatedAt: deadLetter.CreatedAt.UTC(),
	})
	return err
}

func (r *DeadLetterRepository) ensureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "message_id", Value: 1}, {Key: "created_at", Value: -1}},
			Options: options.Index(),
		},
		{
			Keys:    bson.D{{Key: "job_id", Value: 1}},
			Options: options.Index(),
		},
	})
	return err
}
