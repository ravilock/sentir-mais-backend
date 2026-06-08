package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	analysisqueue "github.com/ravilock/sentir-mais-backend/internal/analysis/queue"
)

const (
	defaultConsumeTimeout = time.Second
	defaultRetryDelay     = 30 * time.Second
	defaultRetryLimit     = int64(100)
)

type Queue interface {
	Consume(ctx context.Context, timeout time.Duration) (analysisqueue.ConsumedJob, error)
	Ack(ctx context.Context, consumed analysisqueue.ConsumedJob) error
	RetryLater(ctx context.Context, consumed analysisqueue.ConsumedJob, runAt time.Time) error
	MoveDueRetries(ctx context.Context, now time.Time, limit int64) (int64, error)
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

	if err := w.processor.Process(ctx, consumed.Job); err != nil {
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
