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

type riskFlagsDocument struct {
	SelfHarm         bool `bson:"self_harm"`
	SuicidalIdeation bool `bson:"suicidal_ideation"`
	ImmediateDanger  bool `bson:"immediate_danger"`
}

type extractedEventDocument struct {
	EnoughContext                    bool              `bson:"enough_context"`
	ContextGaps                      []string          `bson:"context_gaps"`
	EventSummary                     string            `bson:"event_summary"`
	WhatHappened                     string            `bson:"what_happened"`
	FeltEmotionsDescribedByUser      []string          `bson:"felt_emotions_described_by_user"`
	UserReaction                     string            `bson:"user_reaction"`
	ExpectedOutcomeOrSelfExpectation string            `bson:"expected_outcome_or_self_expectation"`
	PeopleInvolved                   []string          `bson:"people_involved"`
	Setting                          string            `bson:"setting"`
	TimeReference                    string            `bson:"time_reference"`
	RiskFlags                        riskFlagsDocument `bson:"risk_flags"`
	ConfidenceNotes                  string            `bson:"confidence_notes"`
}

type messageAnalysisDocument struct {
	ID                 string                  `bson:"_id"`
	MessageID          string                  `bson:"message_id"`
	ChatID             string                  `bson:"chat_id"`
	UserID             string                  `bson:"user_id"`
	SourceText         string                  `bson:"source_text"`
	PrimaryFeeling     feelingScoreDocument    `bson:"primary_feeling"`
	SecondaryFeelings  []feelingScoreDocument  `bson:"secondary_feelings"`
	AllScores          []feelingScoreDocument  `bson:"all_scores"`
	EnoughContext      *bool                   `bson:"enough_context,omitempty"`
	ContextGaps        []string                `bson:"context_gaps,omitempty"`
	ExtractedEvent     *extractedEventDocument `bson:"extracted_event,omitempty"`
	ClassifierProvider string                  `bson:"classifier_provider"`
	ClassifierModel    string                  `bson:"classifier_model"`
	CreatedAt          time.Time               `bson:"created_at"`
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
		EnoughContext:      analysis.EnoughContext,
		ContextGaps:        toContextGapStrings(analysis.ContextGaps),
		ExtractedEvent:     toExtractedEventDocument(analysis.ExtractedEvent),
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

func toContextGapStrings(gaps []domain.ContextGap) []string {
	values := make([]string, 0, len(gaps))
	for _, gap := range gaps {
		if gap == "" {
			continue
		}

		values = append(values, string(gap))
	}

	return values
}

func toExtractedEventDocument(event *domain.ExtractedEvent) *extractedEventDocument {
	if event == nil {
		return nil
	}

	return &extractedEventDocument{
		EnoughContext:                    event.EnoughContext,
		ContextGaps:                      toContextGapStrings(event.ContextGaps),
		EventSummary:                     event.EventSummary,
		WhatHappened:                     event.WhatHappened,
		FeltEmotionsDescribedByUser:      event.FeltEmotionsDescribedByUser,
		UserReaction:                     event.UserReaction,
		ExpectedOutcomeOrSelfExpectation: event.ExpectedOutcomeOrSelfExpectation,
		PeopleInvolved:                   event.PeopleInvolved,
		Setting:                          event.Setting,
		TimeReference:                    event.TimeReference,
		RiskFlags: riskFlagsDocument{
			SelfHarm:         event.RiskFlags.SelfHarm,
			SuicidalIdeation: event.RiskFlags.SuicidalIdeation,
			ImmediateDanger:  event.RiskFlags.ImmediateDanger,
		},
		ConfidenceNotes: event.ConfidenceNotes,
	}
}
