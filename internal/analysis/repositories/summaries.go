package repositories

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

type timelinePointDocument struct {
	Date            string `bson:"date"`
	PrimaryFeeling  string `bson:"primary_feeling"`
	SupportingEvent string `bson:"supporting_event"`
}

type dailySummaryDocument struct {
	UserID           string                  `bson:"user_id"`
	DayStart         time.Time               `bson:"day_start"`
	DominantFeelings []feelingScoreDocument  `bson:"dominant_feelings"`
	MainEvents       []string                `bson:"main_events"`
	TimelinePoints   []timelinePointDocument `bson:"timeline_points"`
	GeneratedAt      time.Time               `bson:"generated_at"`
}

type weeklySummaryDocument struct {
	UserID           string                  `bson:"user_id"`
	WeekStart        time.Time               `bson:"week_start"`
	DominantFeelings []feelingScoreDocument  `bson:"dominant_feelings"`
	MainEvents       []string                `bson:"main_events"`
	TimelinePoints   []timelinePointDocument `bson:"timeline_points"`
	GeneratedAt      time.Time               `bson:"generated_at"`
}

type DailySummaryRepository struct {
	collection *mongo.Collection
}

type WeeklySummaryRepository struct {
	collection *mongo.Collection
}

func NewDailySummaryRepository(ctx context.Context, database *mongo.Database) (*DailySummaryRepository, error) {
	repository := &DailySummaryRepository{collection: database.Collection("daily_summaries")}
	if err := repository.ensureIndexes(ctx); err != nil {
		return nil, err
	}

	return repository, nil
}

func NewWeeklySummaryRepository(ctx context.Context, database *mongo.Database) (*WeeklySummaryRepository, error) {
	repository := &WeeklySummaryRepository{collection: database.Collection("weekly_summaries")}
	if err := repository.ensureIndexes(ctx); err != nil {
		return nil, err
	}

	return repository, nil
}

func (r *DailySummaryRepository) Upsert(ctx context.Context, summary domain.DailySummary) error {
	filter := bson.M{
		"user_id":   summary.UserID,
		"day_start": summary.DayStart.UTC(),
	}
	update := bson.M{"$set": dailySummaryDocument{
		UserID:           summary.UserID,
		DayStart:         summary.DayStart.UTC(),
		DominantFeelings: toFeelingScoreDocuments(summary.DominantFeelings),
		MainEvents:       compactSummaryStrings(summary.MainEvents),
		TimelinePoints:   toTimelinePointDocuments(summary.TimelinePoints),
		GeneratedAt:      summary.GeneratedAt.UTC(),
	}}

	_, err := r.collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

func (r *DailySummaryRepository) FindByUserAndDay(ctx context.Context, userID string, dayStart time.Time) (domain.DailySummary, error) {
	var document dailySummaryDocument
	err := r.collection.FindOne(ctx, bson.M{
		"user_id":   userID,
		"day_start": dayStart.UTC(),
	}).Decode(&document)
	if err != nil {
		return domain.DailySummary{}, err
	}

	return document.toDomain(), nil
}

func (r *DailySummaryRepository) ListByUserAndDayRange(ctx context.Context, userID string, from, to time.Time) ([]domain.DailySummary, error) {
	cursor, err := r.collection.Find(ctx, bson.M{
		"user_id": userID,
		"day_start": bson.M{
			"$gte": from.UTC(),
			"$lte": to.UTC(),
		},
	}, options.Find().SetSort(bson.D{{Key: "day_start", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	summaries := make([]domain.DailySummary, 0)
	for cursor.Next(ctx) {
		var document dailySummaryDocument
		if err := cursor.Decode(&document); err != nil {
			return nil, err
		}

		summaries = append(summaries, document.toDomain())
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return summaries, nil
}

func (r *DailySummaryRepository) ensureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "day_start", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "generated_at", Value: -1}},
			Options: options.Index(),
		},
	})
	return err
}

func (r *WeeklySummaryRepository) Upsert(ctx context.Context, summary domain.WeeklySummary) error {
	filter := bson.M{
		"user_id":    summary.UserID,
		"week_start": summary.WeekStart.UTC(),
	}
	update := bson.M{"$set": weeklySummaryDocument{
		UserID:           summary.UserID,
		WeekStart:        summary.WeekStart.UTC(),
		DominantFeelings: toFeelingScoreDocuments(summary.DominantFeelings),
		MainEvents:       compactSummaryStrings(summary.MainEvents),
		TimelinePoints:   toTimelinePointDocuments(summary.TimelinePoints),
		GeneratedAt:      summary.GeneratedAt.UTC(),
	}}

	_, err := r.collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

func (r *WeeklySummaryRepository) FindByUserAndWeek(ctx context.Context, userID string, weekStart time.Time) (domain.WeeklySummary, error) {
	var document weeklySummaryDocument
	err := r.collection.FindOne(ctx, bson.M{
		"user_id":    userID,
		"week_start": weekStart.UTC(),
	}).Decode(&document)
	if err != nil {
		return domain.WeeklySummary{}, err
	}

	return document.toDomain(), nil
}

func (r *WeeklySummaryRepository) ensureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "week_start", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "generated_at", Value: -1}},
			Options: options.Index(),
		},
	})
	return err
}

func toTimelinePointDocuments(points []domain.TimelinePoint) []timelinePointDocument {
	documents := make([]timelinePointDocument, 0, len(points))
	for _, point := range points {
		documents = append(documents, timelinePointDocument{
			Date:            point.Date,
			PrimaryFeeling:  point.PrimaryFeeling,
			SupportingEvent: point.SupportingEvent,
		})
	}

	return documents
}

func toTimelinePoints(points []timelinePointDocument) []domain.TimelinePoint {
	result := make([]domain.TimelinePoint, 0, len(points))
	for _, point := range points {
		result = append(result, domain.TimelinePoint{
			Date:            point.Date,
			PrimaryFeeling:  point.PrimaryFeeling,
			SupportingEvent: point.SupportingEvent,
		})
	}

	return result
}

func (document dailySummaryDocument) toDomain() domain.DailySummary {
	return domain.DailySummary{
		UserID:           document.UserID,
		DayStart:         document.DayStart.UTC(),
		DominantFeelings: toFeelingScores(document.DominantFeelings),
		MainEvents:       compactSummaryStrings(document.MainEvents),
		TimelinePoints:   toTimelinePoints(document.TimelinePoints),
		GeneratedAt:      document.GeneratedAt.UTC(),
	}
}

func (document weeklySummaryDocument) toDomain() domain.WeeklySummary {
	return domain.WeeklySummary{
		UserID:           document.UserID,
		WeekStart:        document.WeekStart.UTC(),
		DominantFeelings: toFeelingScores(document.DominantFeelings),
		MainEvents:       compactSummaryStrings(document.MainEvents),
		TimelinePoints:   toTimelinePoints(document.TimelinePoints),
		GeneratedAt:      document.GeneratedAt.UTC(),
	}
}

func compactSummaryStrings(values []string) []string {
	compacted := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}

		compacted = append(compacted, value)
	}

	return compacted
}
