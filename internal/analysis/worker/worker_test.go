package worker

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	analysisqueue "github.com/ravilock/sentir-mais-backend/internal/analysis/queue"
	analysisservices "github.com/ravilock/sentir-mais-backend/internal/analysis/services"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestWorkerProcessOne(t *testing.T) {
	t.Run("should process and ack consumed job", func(t *testing.T) {
		now := time.Date(2026, time.June, 8, 10, 0, 0, 0, time.UTC)
		job := analysisqueue.AnalysisJob{JobID: "anj_123", ChatID: "cht_123", UserID: "usr_123", MessageID: "msg_123"}
		queue := &fakeQueue{consumed: analysisqueue.ConsumedJob{Job: job}, lockAcquired: true}
		processor := &fakeProcessor{}
		worker := NewWorker(queue, processor, nil, slog.New(slog.NewTextHandler(os.Stdout, nil)))
		worker.clock = fakeClock{now: now}

		processed, err := worker.ProcessOne(context.Background())

		require.NoError(t, err)
		require.True(t, processed)
		require.Equal(t, int64(100), queue.moveLimit)
		require.Equal(t, now, queue.moveNow)
		require.Equal(t, time.Second, queue.consumeTimeout)
		require.Equal(t, []analysisqueue.AnalysisJob{job}, processor.jobs)
		require.Equal(t, 1, queue.ackCount)
		require.Zero(t, queue.retryCount)
		require.Equal(t, "cht_123", queue.lockChatID)
		require.Equal(t, "anj_123", queue.lockOwner)
		require.Equal(t, 15*time.Minute, queue.lockTTL)
		require.Equal(t, 1, queue.releaseCount)
	})

	t.Run("should retry later when processing fails", func(t *testing.T) {
		now := time.Date(2026, time.June, 8, 10, 0, 0, 0, time.UTC)
		expectedErr := errors.New("extract failed")
		job := analysisqueue.AnalysisJob{JobID: "anj_123", ChatID: "cht_123", UserID: "usr_123", MessageID: "msg_123"}
		queue := &fakeQueue{consumed: analysisqueue.ConsumedJob{Job: job}, lockAcquired: true}
		processor := &fakeProcessor{err: expectedErr}
		worker := NewWorker(queue, processor, nil, slog.New(slog.NewTextHandler(os.Stdout, nil)))
		worker.clock = fakeClock{now: now}

		processed, err := worker.ProcessOne(context.Background())

		require.ErrorIs(t, err, expectedErr)
		require.False(t, processed)
		require.Zero(t, queue.ackCount)
		require.Equal(t, 1, queue.retryCount)
		require.Equal(t, now.Add(30*time.Second), queue.retryAt)
	})

	t.Run("should dead letter when processing fails after max worker attempts", func(t *testing.T) {
		expectedErr := errors.New("db unavailable")
		job := analysisqueue.AnalysisJob{JobID: "anj_123", ChatID: "cht_123", UserID: "usr_123", MessageID: "msg_123", Attempt: 10}
		queue := &fakeQueue{consumed: analysisqueue.ConsumedJob{Job: job}, lockAcquired: true}
		processor := &fakeProcessor{err: expectedErr}
		deadLetters := &fakeDeadLetterCreator{}
		now := time.Date(2026, time.June, 8, 10, 0, 0, 0, time.UTC)
		worker := NewWorker(queue, processor, deadLetters, slog.New(slog.NewTextHandler(os.Stdout, nil)))
		worker.clock = fakeClock{now: now}

		processed, err := worker.ProcessOne(context.Background())

		require.ErrorIs(t, err, expectedErr)
		require.False(t, processed)
		require.Zero(t, queue.retryCount)
		require.Equal(t, 1, queue.deadLetterCount)
		require.Equal(t, 1, queue.releaseCount)
		require.Len(t, deadLetters.created, 1)
		require.Equal(t, "anj_123", deadLetters.created[0].JobID)
		require.Equal(t, "worker_max_attempts_exceeded", deadLetters.created[0].Reason)
		require.Equal(t, expectedErr.Error(), deadLetters.created[0].Error)
		require.Equal(t, 10, deadLetters.created[0].Attempt)
		require.Equal(t, now, deadLetters.created[0].CreatedAt)
	})

	t.Run("should delay job when chat lock is already held", func(t *testing.T) {
		now := time.Date(2026, time.June, 8, 10, 0, 0, 0, time.UTC)
		job := analysisqueue.AnalysisJob{JobID: "anj_123", ChatID: "cht_123", UserID: "usr_123", MessageID: "msg_123"}
		queue := &fakeQueue{consumed: analysisqueue.ConsumedJob{Job: job}, lockAcquired: false}
		processor := &fakeProcessor{}
		worker := NewWorker(queue, processor, nil, slog.New(slog.NewTextHandler(os.Stdout, nil)))
		worker.clock = fakeClock{now: now}

		processed, err := worker.ProcessOne(context.Background())

		require.NoError(t, err)
		require.False(t, processed)
		require.Empty(t, processor.jobs)
		require.Equal(t, 1, queue.retryCount)
		require.Equal(t, now.Add(time.Second), queue.retryAt)
		require.Zero(t, queue.releaseCount)
	})

	t.Run("should dead letter when chat lock stays held after max worker attempts", func(t *testing.T) {
		job := analysisqueue.AnalysisJob{JobID: "anj_123", ChatID: "cht_123", UserID: "usr_123", MessageID: "msg_123", Attempt: 10}
		queue := &fakeQueue{consumed: analysisqueue.ConsumedJob{Job: job}, lockAcquired: false}
		processor := &fakeProcessor{}
		deadLetters := &fakeDeadLetterCreator{}
		worker := NewWorker(queue, processor, deadLetters, slog.New(slog.NewTextHandler(os.Stdout, nil)))
		worker.clock = fakeClock{now: time.Date(2026, time.June, 8, 10, 0, 0, 0, time.UTC)}

		processed, err := worker.ProcessOne(context.Background())

		require.NoError(t, err)
		require.False(t, processed)
		require.Empty(t, processor.jobs)
		require.Zero(t, queue.retryCount)
		require.Equal(t, 1, queue.deadLetterCount)
		require.Zero(t, queue.releaseCount)
		require.Len(t, deadLetters.created, 1)
		require.Equal(t, "max worker attempts exceeded", deadLetters.created[0].Error)
	})

	t.Run("should requeue consumed job when chat lock acquisition errors", func(t *testing.T) {
		now := time.Date(2026, time.June, 8, 10, 0, 0, 0, time.UTC)
		expectedErr := errors.New("redis lock failed")
		job := analysisqueue.AnalysisJob{JobID: "anj_123", ChatID: "cht_123", UserID: "usr_123", MessageID: "msg_123"}
		queue := &fakeQueue{consumed: analysisqueue.ConsumedJob{Job: job}, lockErr: expectedErr}
		processor := &fakeProcessor{}
		worker := NewWorker(queue, processor, nil, slog.New(slog.NewTextHandler(os.Stdout, nil)))
		worker.clock = fakeClock{now: now}

		processed, err := worker.ProcessOne(context.Background())

		require.ErrorIs(t, err, expectedErr)
		require.False(t, processed)
		require.Empty(t, processor.jobs)
		require.Equal(t, 1, queue.retryCount)
		require.Equal(t, now.Add(time.Second), queue.retryAt)
		require.Zero(t, queue.ackCount)
		require.Zero(t, queue.releaseCount)
	})

	t.Run("should ack dead-lettered jobs", func(t *testing.T) {
		job := analysisqueue.AnalysisJob{JobID: "anj_123", ChatID: "cht_123", UserID: "usr_123", MessageID: "msg_123"}
		queue := &fakeQueue{consumed: analysisqueue.ConsumedJob{Job: job}, lockAcquired: true}
		processor := &fakeProcessor{err: analysisservices.ErrDeadLettered}
		worker := NewWorker(queue, processor, nil, slog.New(slog.NewTextHandler(os.Stdout, nil)))
		worker.clock = fakeClock{now: time.Date(2026, time.June, 8, 10, 0, 0, 0, time.UTC)}

		processed, err := worker.ProcessOne(context.Background())

		require.NoError(t, err)
		require.True(t, processed)
		require.Equal(t, 1, queue.ackCount)
		require.Zero(t, queue.retryCount)
	})
}

type fakeQueue struct {
	consumed        analysisqueue.ConsumedJob
	consumeErr      error
	consumeTimeout  time.Duration
	moveNow         time.Time
	moveLimit       int64
	ackCount        int
	retryCount      int
	retryAt         time.Time
	deadLetterCount int
	lockAcquired    bool
	lockChatID      string
	lockOwner       string
	lockTTL         time.Duration
	lockErr         error
	releaseCount    int
}

func (q *fakeQueue) Consume(_ context.Context, timeout time.Duration) (analysisqueue.ConsumedJob, error) {
	q.consumeTimeout = timeout
	if q.consumeErr != nil {
		return analysisqueue.ConsumedJob{}, q.consumeErr
	}

	return q.consumed, nil
}

func (q *fakeQueue) Ack(_ context.Context, _ analysisqueue.ConsumedJob) error {
	q.ackCount++
	return nil
}

func (q *fakeQueue) RetryLater(_ context.Context, _ analysisqueue.ConsumedJob, runAt time.Time) error {
	q.retryCount++
	q.retryAt = runAt
	return nil
}

func (q *fakeQueue) DeadLetter(_ context.Context, _ analysisqueue.ConsumedJob) error {
	q.deadLetterCount++
	return nil
}

func (q *fakeQueue) MoveDueRetries(_ context.Context, now time.Time, limit int64) (int64, error) {
	q.moveNow = now
	q.moveLimit = limit
	return 0, nil
}

func (q *fakeQueue) AcquireChatLock(_ context.Context, chatID, owner string, ttl time.Duration) (bool, error) {
	q.lockChatID = chatID
	q.lockOwner = owner
	q.lockTTL = ttl
	if q.lockErr != nil {
		return false, q.lockErr
	}

	return q.lockAcquired, nil
}

func (q *fakeQueue) ReleaseChatLock(_ context.Context, _ string, _ string) error {
	q.releaseCount++
	return nil
}

type fakeProcessor struct {
	jobs []analysisqueue.AnalysisJob
	err  error
}

func (p *fakeProcessor) Process(_ context.Context, job analysisqueue.AnalysisJob) error {
	if p.err != nil {
		return p.err
	}

	p.jobs = append(p.jobs, job)
	return nil
}

type fakeDeadLetterCreator struct {
	created []domain.AnalysisDeadLetter
	err     error
}

func (c *fakeDeadLetterCreator) Create(_ context.Context, deadLetter domain.AnalysisDeadLetter) error {
	if c.err != nil {
		return c.err
	}

	c.created = append(c.created, deadLetter)
	return nil
}

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time {
	return c.now
}
