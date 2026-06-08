package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	analysisqueue "github.com/ravilock/sentir-mais-backend/internal/analysis/queue"
	analysisservices "github.com/ravilock/sentir-mais-backend/internal/analysis/services"
)

const (
	defaultConsumeTimeout = time.Second
	defaultRetryDelay     = 30 * time.Second
	defaultLockRetryDelay = time.Second
	defaultChatLockTTL    = 2 * time.Minute
	defaultRetryLimit     = int64(100)
)

type Queue interface {
	Consume(ctx context.Context, timeout time.Duration) (analysisqueue.ConsumedJob, error)
	Ack(ctx context.Context, consumed analysisqueue.ConsumedJob) error
	RetryLater(ctx context.Context, consumed analysisqueue.ConsumedJob, runAt time.Time) error
	MoveDueRetries(ctx context.Context, now time.Time, limit int64) (int64, error)
	AcquireChatLock(ctx context.Context, chatID, owner string, ttl time.Duration) (bool, error)
	ReleaseChatLock(ctx context.Context, chatID, owner string) error
}

type Processor interface {
	Process(ctx context.Context, job analysisqueue.AnalysisJob) error
}

type Clock interface {
	Now() time.Time
}

type Worker struct {
	queue          Queue
	processor      Processor
	logger         *slog.Logger
	clock          Clock
	consumeTimeout time.Duration
	retryDelay     time.Duration
	lockRetryDelay time.Duration
	chatLockTTL    time.Duration
	retryLimit     int64
}

func NewWorker(queue Queue, processor Processor, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}

	return &Worker{
		queue:          queue,
		processor:      processor,
		logger:         logger,
		clock:          realClock{},
		consumeTimeout: defaultConsumeTimeout,
		retryDelay:     defaultRetryDelay,
		lockRetryDelay: defaultLockRetryDelay,
		chatLockTTL:    defaultChatLockTTL,
		retryLimit:     defaultRetryLimit,
	}
}

func (w *Worker) Run(ctx context.Context) {
	w.logger.Info("analysis worker started")
	defer w.logger.Info("analysis worker stopped")

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		processed, err := w.ProcessOne(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			w.logger.Error("analysis worker process failed", "error", err)
		}
		if !processed && err == nil {
			continue
		}
	}
}

func (w *Worker) ProcessOne(ctx context.Context) (bool, error) {
	if w.queue == nil {
		return false, errors.New("analysis queue is required")
	}
	if w.processor == nil {
		return false, errors.New("analysis processor is required")
	}

	if _, err := w.queue.MoveDueRetries(ctx, w.clock.Now(), w.retryLimit); err != nil {
		return false, err
	}

	consumed, err := w.queue.Consume(ctx, w.consumeTimeout)
	if err != nil {
		if errors.Is(err, analysisqueue.ErrNoJob) {
			return false, nil
		}

		return false, err
	}

	lockOwner := consumed.Job.JobID
	if lockOwner == "" {
		lockOwner = consumed.Job.MessageID
	}
	locked, err := w.queue.AcquireChatLock(ctx, consumed.Job.ChatID, lockOwner, w.chatLockTTL)
	if err != nil {
		retryAt := w.clock.Now().Add(w.lockRetryDelay)
		if retryErr := w.queue.RetryLater(ctx, consumed, retryAt); retryErr != nil {
			return false, errors.Join(err, retryErr)
		}

		return false, err
	}
	if !locked {
		return false, w.queue.RetryLater(ctx, consumed, w.clock.Now().Add(w.lockRetryDelay))
	}
	defer func() {
		if err := w.queue.ReleaseChatLock(ctx, consumed.Job.ChatID, lockOwner); err != nil {
			w.logger.Error("failed to release analysis chat lock", "chat_id", consumed.Job.ChatID, "job_id", consumed.Job.JobID, "error", err)
		}
	}()

	if err := w.processor.Process(ctx, consumed.Job); err != nil {
		if errors.Is(err, analysisservices.ErrDeadLettered) {
			if ackErr := w.queue.Ack(ctx, consumed); ackErr != nil {
				return false, errors.Join(err, ackErr)
			}

			return true, nil
		}

		retryAt := w.clock.Now().Add(w.retryDelay)
		if retryErr := w.queue.RetryLater(ctx, consumed, retryAt); retryErr != nil {
			return false, errors.Join(err, retryErr)
		}

		return false, err
	}

	if err := w.queue.Ack(ctx, consumed); err != nil {
		return false, err
	}

	return true, nil
}

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now().UTC()
}
