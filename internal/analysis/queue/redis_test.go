package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRedisQueueEnqueueConsumeAck(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	queue, err := NewRedisQueue(client, "analysis-test")
	require.NoError(t, err)

	job := testAnalysisJob()
	require.NoError(t, queue.Enqueue(ctx, job))

	consumed, err := queue.Consume(ctx, time.Millisecond)
	require.NoError(t, err)
	require.Equal(t, job, consumed.Job)
	require.NotEmpty(t, consumed.raw)
	require.Equal(t, int64(1), client.LLen(ctx, queue.processingKey).Val())

	require.NoError(t, queue.Ack(ctx, consumed))
	require.Equal(t, int64(0), client.LLen(ctx, queue.processingKey).Val())
}

func TestRedisQueueRetryLaterAndMoveDueRetries(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	queue, err := NewRedisQueue(client, "analysis-test")
	require.NoError(t, err)

	job := testAnalysisJob()
	require.NoError(t, queue.Enqueue(ctx, job))

	consumed, err := queue.Consume(ctx, time.Millisecond)
	require.NoError(t, err)

	runAt := time.Date(2026, time.June, 8, 12, 0, 5, 0, time.UTC)
	require.NoError(t, queue.RetryLater(ctx, consumed, runAt))
	require.Equal(t, int64(0), client.LLen(ctx, queue.processingKey).Val())
	require.Equal(t, int64(1), client.ZCard(ctx, queue.delayedKey).Val())

	moved, err := queue.MoveDueRetries(ctx, runAt.Add(-time.Millisecond), 100)
	require.NoError(t, err)
	require.Equal(t, int64(0), moved)
	require.Equal(t, int64(0), client.LLen(ctx, queue.readyKey).Val())

	moved, err = queue.MoveDueRetries(ctx, runAt, 100)
	require.NoError(t, err)
	require.Equal(t, int64(1), moved)
	require.Equal(t, int64(1), client.LLen(ctx, queue.readyKey).Val())

	consumedAgain, err := queue.Consume(ctx, time.Millisecond)
	require.NoError(t, err)
	require.Equal(t, job.JobID, consumedAgain.Job.JobID)
	require.Equal(t, 1, consumedAgain.Job.Attempt)
}

func TestRedisQueueConsumeNoJob(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	queue, err := NewRedisQueue(client, "analysis-test")
	require.NoError(t, err)

	_, err = queue.Consume(ctx, time.Millisecond)
	require.True(t, errors.Is(err, ErrNoJob))
}

func TestAnalysisJobSerialization(t *testing.T) {
	job := testAnalysisJob()

	payload, err := encodeJob(job)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"job_id":"anj_123",
		"chat_id":"cht_123",
		"user_id":"usr_123",
		"message_id":"msg_123",
		"message_created_at":"2026-06-08T03:00:00Z",
		"enqueued_at":"2026-06-08T03:00:01Z",
		"attempt":0,
		"stage":"extract"
	}`, payload)

	decoded, err := decodeJob(payload)
	require.NoError(t, err)
	require.Equal(t, job, decoded)
}

func testAnalysisJob() AnalysisJob {
	return AnalysisJob{
		JobID:            "anj_123",
		ChatID:           "cht_123",
		UserID:           "usr_123",
		MessageID:        "msg_123",
		MessageCreatedAt: time.Date(2026, time.June, 8, 3, 0, 0, 0, time.UTC),
		EnqueuedAt:       time.Date(2026, time.June, 8, 3, 0, 1, 0, time.UTC),
		Attempt:          0,
		Stage:            StageExtract,
	}
}
