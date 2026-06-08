package analysis

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	integrationtests "github.com/ravilock/sentir-mais-backend/integrationTests"
	analysisqueue "github.com/ravilock/sentir-mais-backend/internal/analysis/queue"
	analysisrepositories "github.com/ravilock/sentir-mais-backend/internal/analysis/repositories"
	analysisservices "github.com/ravilock/sentir-mais-backend/internal/analysis/services"
	apirequests "github.com/ravilock/sentir-mais-backend/internal/api/requests"
	apiresponses "github.com/ravilock/sentir-mais-backend/internal/api/responses"
	chatrepositories "github.com/ravilock/sentir-mais-backend/internal/chat/repositories"
	"github.com/ravilock/sentir-mais-backend/internal/classifier"
	dashboardservices "github.com/ravilock/sentir-mais-backend/internal/dashboard/services"
	"github.com/ravilock/sentir-mais-backend/internal/llm"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func TestChatAnalysisFlow(t *testing.T) {
	integrationtests.ClearDatabase(t)
	redisServer.FlushAll()

	token := registerTestUser(t)
	processor := newAnalysisProcessor(t)
	queue := newAnalysisQueue(t)

	createResponse := integrationtests.MustJSONRequest(t, http.MethodPost, "/api/v1/chats", apirequests.CreateChatRequest{
		InitialMessage: "Discuti com meu gerente hoje, fiquei ansioso e travei.",
	}, token)
	require.Equal(t, http.StatusCreated, createResponse.StatusCode)
	createPayload := integrationtests.DecodeResponse[apiresponses.CreateChatResponse](t, createResponse)
	require.NotEmpty(t, createPayload.ChatID)
	require.NotEmpty(t, createPayload.Response.ID)
	require.Equal(t, "Entendi. Vamos organizar isso com calma: o que voce esperava que acontecesse?", createPayload.Response.Content)

	initialJob := consumeAndProcessAnalysisJob(t, queue, processor)
	require.Equal(t, createPayload.ChatID, initialJob.ChatID)

	initialAnalysis := integrationtests.EventuallyFindDocument(t, "message_analyses", bson.M{
		"chat_id":     createPayload.ChatID,
		"message_id":  initialJob.MessageID,
		"source_text": "Discuti com meu gerente hoje, fiquei ansioso e travei.",
	})
	require.Equal(t, "anxious", nestedString(t, initialAnalysis, "primary_feeling", "label"))
	require.Equal(t, "sentir-mais-classifier", initialAnalysis["classifier_provider"])
	require.Equal(t, "test-classifier-model", initialAnalysis["classifier_model"])
	require.Equal(t, "Discussao com o gerente no trabalho.", nestedString(t, initialAnalysis, "extracted_event", "event_summary"))
	require.Equal(t, true, initialAnalysis["enough_context"])

	sendResponse := integrationtests.MustJSONRequest(t, http.MethodPost, "/api/v1/chats/"+createPayload.ChatID+"/messages", apirequests.SendMessageRequest{
		Message: "Eu esperava respeito e acabei ficando quieto, mas queria ter respondido com calma.",
	}, token)
	require.Equal(t, http.StatusOK, sendResponse.StatusCode)
	sendPayload := integrationtests.DecodeResponse[apiresponses.MessageResponse](t, sendResponse)
	require.NotEmpty(t, sendPayload.ID)
	require.Equal(t, "Entendi. Vamos organizar isso com calma: o que voce esperava que acontecesse?", sendPayload.Content)

	secondJob := consumeAndProcessAnalysisJob(t, queue, processor)
	require.Equal(t, createPayload.ChatID, secondJob.ChatID)

	secondAnalysis := integrationtests.EventuallyFindDocument(t, "message_analyses", bson.M{
		"chat_id":    createPayload.ChatID,
		"message_id": secondJob.MessageID,
	})
	require.Equal(t, "Eu esperava respeito e acabei ficando quieto, mas queria ter respondido com calma.", secondAnalysis["source_text"])
	require.Equal(t, "anxious", nestedString(t, secondAnalysis, "primary_feeling", "label"))

	messagesResponse := integrationtests.MustJSONRequest(t, http.MethodGet, "/api/v1/chats/"+createPayload.ChatID+"/messages", nil, token)
	require.Equal(t, http.StatusOK, messagesResponse.StatusCode)
	messagesPayload := integrationtests.DecodeResponse[apiresponses.ListMessagesResponse](t, messagesResponse)
	require.Len(t, messagesPayload.Messages, 4)

	weekResponse := integrationtests.MustJSONRequest(t, http.MethodGet, "/api/v1/dashboard/week", nil, token)
	require.Equal(t, http.StatusOK, weekResponse.StatusCode)
	weekPayload := integrationtests.DecodeResponse[apiresponses.WeeklySummaryResponse](t, weekResponse)
	require.NotEmpty(t, weekPayload.DominantFeelings)
	require.Equal(t, "anxious", weekPayload.DominantFeelings[0].Label)
	require.Contains(t, weekPayload.MainEvents, "Discussao com o gerente no trabalho.")
	require.NotEmpty(t, weekPayload.TimelinePoints)
	require.Equal(t, "anxious", weekPayload.TimelinePoints[0].PrimaryFeeling)

	today := time.Now().UTC().Format(time.DateOnly)
	timelineResponse := integrationtests.MustJSONRequest(t, http.MethodGet, "/api/v1/dashboard/timeline?from="+today+"&to="+today, nil, token)
	require.Equal(t, http.StatusOK, timelineResponse.StatusCode)
	timelinePayload := integrationtests.DecodeResponse[apiresponses.DashboardTimelineResponse](t, timelineResponse)
	require.Equal(t, today, timelinePayload.From)
	require.Equal(t, today, timelinePayload.To)
	require.Len(t, timelinePayload.Days, 1)
	require.Equal(t, "anxious", timelinePayload.Days[0].DominantFeelings[0].Label)
	require.Contains(t, timelinePayload.Days[0].MainEvents, "Discussao com o gerente no trabalho.")
}

func registerTestUser(t *testing.T) string {
	t.Helper()

	response := integrationtests.MustJSONRequest(t, http.MethodPost, "/api/v1/auth/register", apirequests.RegisterRequest{
		Email:    "analysis-flow@test.com",
		Password: "super-safe-password",
	}, "")
	require.Equal(t, http.StatusCreated, response.StatusCode)

	payload := integrationtests.DecodeResponse[apiresponses.AuthResponse](t, response)
	require.NotEmpty(t, payload.AccessToken)
	return payload.AccessToken
}

func newAnalysisQueue(t *testing.T) *analysisqueue.RedisQueue {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})

	queue, err := analysisqueue.NewRedisQueue(client, "analysis-integration-tests")
	require.NoError(t, err)
	return queue
}

func newAnalysisProcessor(t *testing.T) *analysisservices.Processor {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	database := integrationtests.Database()
	messageRepository, err := chatrepositories.NewMessageRepository(ctx, database)
	require.NoError(t, err)
	messageAnalysisRepository, err := analysisrepositories.NewMessageAnalysisRepository(ctx, database)
	require.NoError(t, err)
	dailySummaryRepository, err := analysisrepositories.NewDailySummaryRepository(ctx, database)
	require.NoError(t, err)
	weeklySummaryRepository, err := analysisrepositories.NewWeeklySummaryRepository(ctx, database)
	require.NoError(t, err)
	deadLetterRepository, err := analysisrepositories.NewDeadLetterRepository(ctx, database)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	prompterClient := llm.NewPrompterClient(prompterServer.URL, "", time.Second, logger)
	classifierClient := classifier.NewClient(classifierServer.URL, "", time.Second, logger)
	summaryWriter := dashboardservices.NewSummaryWriter(messageAnalysisRepository, dailySummaryRepository, weeklySummaryRepository)

	return analysisservices.NewProcessorWithDeadLetters(
		messageRepository,
		prompterClient,
		classifierClient,
		messageAnalysisRepository,
		summaryWriter,
		deadLetterRepository,
		nil,
		logger,
	)
}

func consumeAndProcessAnalysisJob(t *testing.T, queue *analysisqueue.RedisQueue, processor *analysisservices.Processor) analysisqueue.AnalysisJob {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	consumed, err := queue.Consume(ctx, time.Second)
	require.NoError(t, err)
	require.Equal(t, analysisqueue.StageExtract, consumed.Job.Stage)

	require.NoError(t, processor.Process(ctx, consumed.Job))
	require.NoError(t, queue.Ack(ctx, consumed))
	return consumed.Job
}

func nestedString(t *testing.T, document bson.M, parentKey, childKey string) string {
	t.Helper()

	parent, ok := document[parentKey].(bson.M)
	require.Truef(t, ok, "expected %s to be a document", parentKey)
	value, ok := parent[childKey].(string)
	require.Truef(t, ok, "expected %s.%s to be a string", parentKey, childKey)
	return value
}
