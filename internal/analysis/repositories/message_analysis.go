package repositories

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

type MessageAnalysisRepository struct {
	collection *mongo.Collection
}

type feelingScoreDocument struct {
	Label      string  `bson:"label"`
	Confidence float64 `bson:"confidence"`
}

type messageAnalysisDocument struct {
	ID                 string                 `bson:"_id"`
	MessageID          string                 `bson:"message_id"`
	ChatID             string                 `bson:"chat_id"`
	UserID             string                 `bson:"user_id"`
	SourceText         string                 `bson:"source_text"`
	PrimaryFeeling     feelingScoreDocument   `bson:"primary_feeling"`
	SecondaryFeelings  []feelingScoreDocument `bson:"secondary_feelings"`
	AllScores          []feelingScoreDocument `bson:"all_scores"`
	ClassifierProvider string                 `bson:"classifier_provider"`
	ClassifierModel    string                 `bson:"classifier_model"`
	CreatedAt          time.Time              `bson:"created_at"`
}

func NewMessageAnalysisRepository(ctx context.Context, database *mongo.Database) (*MessageAnalysisRepository, error) {
	repository := &MessageAnalysisRepository{collection: database.Collection("message_analyses")}
	if err := repository.ensureIndexes(ctx); err != nil {
		return nil, err
	}

	return repository, nil
}

func (r *MessageAnalysisRepository) Create(ctx context.Context, analysis domain.MessageAnalysis) error {
	_, err := r.collection.InsertOne(ctx, messageAnalysisDocument{
		ID:                 analysis.ID,
		MessageID:          analysis.MessageID,
		ChatID:             analysis.ChatID,
		UserID:             analysis.UserID,
		SourceText:         analysis.SourceText,
		PrimaryFeeling:     toFeelingScoreDocument(analysis.PrimaryFeeling),
		SecondaryFeelings:  toFeelingScoreDocuments(analysis.SecondaryFeelings),
		AllScores:          toFeelingScoreDocuments(analysis.AllScores),
		ClassifierProvider: analysis.ClassifierProvider,
		ClassifierModel:    analysis.ClassifierModel,
		CreatedAt:          analysis.CreatedAt.UTC(),
	})
	return err
}

func (r *MessageAnalysisRepository) ensureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "message_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "chat_id", Value: 1}, {Key: "created_at", Value: -1}},
			Options: options.Index(),
		},
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}},
			Options: options.Index(),
		},
	})
	return err
}

func toFeelingScoreDocument(score domain.FeelingScore) feelingScoreDocument {
	return feelingScoreDocument{
		Label:      score.Label,
		Confidence: score.Confidence,
	}
}

func toFeelingScoreDocuments(scores []domain.FeelingScore) []feelingScoreDocument {
	documents := make([]feelingScoreDocument, 0, len(scores))
	for _, score := range scores {
		documents = append(documents, toFeelingScoreDocument(score))
	}

	return documents
}
