package services

import (
	"context"
	"sync"

	analysisqueue "github.com/ravilock/sentir-mais-backend/internal/analysis/queue"
)

type capturingAnalysisJobEnqueuer struct {
	mu   sync.Mutex
	jobs []analysisqueue.AnalysisJob
	err  error
}

func (e *capturingAnalysisJobEnqueuer) Enqueue(_ context.Context, job analysisqueue.AnalysisJob) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.err != nil {
		return e.err
	}

	e.jobs = append(e.jobs, job)
	return nil
}

func (e *capturingAnalysisJobEnqueuer) jobCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()

	return len(e.jobs)
}

func (e *capturingAnalysisJobEnqueuer) lastJob() analysisqueue.AnalysisJob {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.jobs[len(e.jobs)-1]
}
