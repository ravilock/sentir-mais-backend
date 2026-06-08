package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	analysisqueue "github.com/ravilock/sentir-mais-backend/internal/analysis/queue"
	analysisservices "github.com/ravilock/sentir-mais-backend/internal/analysis/services"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/ravilock/sentir-mais-backend/internal/id"
)

const (
	defaultConsumeTimeout = time.Second
	defaultRetryDelay     = 30 * time.Second
	defaultLockRetryDelay = time.Second
	defaultChatLockTTL    = 15 * time.Minute
	defaultRetryLimit     = int64(100)
	defaultMaxAttempts    = 10
)

type Queue interface {
	Consume(ctx context.Context, timeout time.Duration) (analysisqueue.ConsumedJob, error)
	Ack(ctx context.Context, consumed analysisqueue.ConsumedJob) error
	RetryLater(ctx context.Context, consumed analysisqueue.ConsumedJob, runAt time.Time) error
	DeadLetter(ctx context.Context, consumed analysisqueue.ConsumedJob) error
	MoveDueRetries(ctx context.Context, now time.Time, limit int64) (int64, error)
	AcquireChatLock(ctx context.Context, chatID, owner string, ttl time.Duration) (bool, error)
	ReleaseChatLock(ctx context.Context, chatID, owner string) error
}

type Processor interface {
	Process(ctx context.Context, job analysisqueue.AnalysisJob) error
}

type DeadLetterCreator interface {
	Create(ctx context.Context, deadLetter domain.AnalysisDeadLetter) error
}

type Clock interface {
	Now() time.Time
}

type Worker struct {
	queue          Queue
	processor      Processor
	deadLetters    DeadLetterCreator
	logger         *slog.Logger
	clock          Clock
	consumeTimeout time.Duration
	retryDelay     time.Duration
	lockRetryDelay time.Duration
	chatLockTTL    time.Duration
	retryLimit     int64
	maxAttempts    int
}

func NewWorker(queue Queue, processor Processor, deadLetters DeadLetterCreator, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}

	return &Worker{
		queue:          queue,
		processor:      processor,
		deadLetters:    deadLetters,
		logger:         logger,
		clock:          realClock{},
		consumeTimeout: defaultConsumeTimeout,
		retryDelay:     defaultRetryDelay,
		lockRetryDelay: defaultLockRetryDelay,
		chatLockTTL:    defaultChatLockTTL,
		retryLimit:     defaultRetryLimit,
		maxAttempts:    defaultMaxAttempts,
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

	movedRetries, err := w.queue.MoveDueRetries(ctx, w.clock.Now(), w.retryLimit)
	if err != nil {
		return false, err
	}
	if movedRetries > 0 {
		w.logger.InfoContext(ctx, "analysis delayed retries moved",
			"count", movedRetries,
		)
	}

	consumed, err := w.queue.Consume(ctx, w.consumeTimeout)
	if err != nil {
		if errors.Is(err, analysisqueue.ErrNoJob) {
			return false, nil
		}

		return false, err
	}
	w.logger.DebugContext(ctx, "analysis job consumed",
		"job_id", consumed.Job.JobID,
		"chat_id", consumed.Job.ChatID,
		"user_id", consumed.Job.UserID,
		"message_id", consumed.Job.MessageID,
		"stage", consumed.Job.Stage,
		"attempt", consumed.Job.Attempt,
	)

	lockOwner := consumed.Job.JobID
	if lockOwner == "" {
		lockOwner = consumed.Job.MessageID
	}
	locked, err := w.queue.AcquireChatLock(ctx, consumed.Job.ChatID, lockOwner, w.chatLockTTL)
	if err != nil {
		return false, w.retryOrDeadLetter(ctx, consumed, w.lockRetryDelay, err)
	}
	if !locked {
		w.logger.InfoContext(ctx, "analysis job delayed by active chat lock",
			"job_id", consumed.Job.JobID,
			"chat_id", consumed.Job.ChatID,
			"user_id", consumed.Job.UserID,
			"message_id", consumed.Job.MessageID,
			"attempt", consumed.Job.Attempt,
		)
		return false, w.retryOrDeadLetter(ctx, consumed, w.lockRetryDelay, nil)
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

		return false, w.retryOrDeadLetter(ctx, consumed, w.retryDelay, err)
	}

	if err := w.queue.Ack(ctx, consumed); err != nil {
		return false, err
	}
	w.logger.InfoContext(ctx, "analysis job processed",
		"job_id", consumed.Job.JobID,
		"chat_id", consumed.Job.ChatID,
		"user_id", consumed.Job.UserID,
		"message_id", consumed.Job.MessageID,
		"stage", consumed.Job.Stage,
		"attempt", consumed.Job.Attempt,
	)

	return true, nil
}

func (w *Worker) retryOrDeadLetter(ctx context.Context, consumed analysisqueue.ConsumedJob, delay time.Duration, cause error) error {
	if consumed.Job.Attempt >= w.maxAttempts {
		w.logger.ErrorContext(ctx, "analysis job max worker attempts exceeded",
			"job_id", consumed.Job.JobID,
			"chat_id", consumed.Job.ChatID,
			"user_id", consumed.Job.UserID,
			"message_id", consumed.Job.MessageID,
			"attempt", consumed.Job.Attempt,
			"error", cause,
		)

		if err := w.persistDeadLetter(ctx, consumed.Job, cause); err != nil {
			if cause != nil {
				return errors.Join(cause, err)
			}
			return err
		}
		if err := w.queue.DeadLetter(ctx, consumed); err != nil {
			if cause != nil {
				return errors.Join(cause, err)
			}
			return err
		}
		return cause
	}

	retryAt := w.clock.Now().Add(delay)
	w.logger.InfoContext(ctx, "analysis job scheduled for retry",
		"job_id", consumed.Job.JobID,
		"chat_id", consumed.Job.ChatID,
		"user_id", consumed.Job.UserID,
		"message_id", consumed.Job.MessageID,
		"stage", consumed.Job.Stage,
		"attempt", consumed.Job.Attempt,
		"run_at", retryAt.UTC().Format(time.RFC3339),
		"error", cause,
	)
	if err := w.queue.RetryLater(ctx, consumed, retryAt); err != nil {
		if cause != nil {
			return errors.Join(cause, err)
		}
		return err
	}

	return cause
}

func (w *Worker) persistDeadLetter(ctx context.Context, job analysisqueue.AnalysisJob, cause error) error {
	if w.deadLetters == nil {
		return nil
	}

	deadLetterID, err := id.New("adl")
	if err != nil {
		return err
	}

	errorMessage := "max worker attempts exceeded"
	if cause != nil {
		errorMessage = cause.Error()
	}

	deadLetter := domain.AnalysisDeadLetter{
		ID:        deadLetterID,
		JobID:     job.JobID,
		ChatID:    job.ChatID,
		UserID:    job.UserID,
		MessageID: job.MessageID,
		Stage:     string(job.Stage),
		Reason:    "worker_max_attempts_exceeded",
		Error:     errorMessage,
		Attempt:   job.Attempt,
		CreatedAt: w.clock.Now(),
	}
	if err := w.deadLetters.Create(ctx, deadLetter); err != nil {
		return err
	}

	w.logger.ErrorContext(ctx, "analysis job dead-lettered",
		"dead_letter_id", deadLetter.ID,
		"job_id", job.JobID,
		"chat_id", job.ChatID,
		"user_id", job.UserID,
		"message_id", job.MessageID,
		"stage", job.Stage,
		"reason", deadLetter.Reason,
		"attempt", job.Attempt,
		"error", deadLetter.Error,
	)
	return nil
}

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now().UTC()
}
