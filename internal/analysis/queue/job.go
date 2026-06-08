package queue

import "time"

type Stage string

const (
	StageExtract   Stage = "extract"
	StageClassify  Stage = "classify"
	StageSummaries Stage = "summaries"
)

type AnalysisJob struct {
	JobID            string    `json:"job_id"`
	ChatID           string    `json:"chat_id"`
	UserID           string    `json:"user_id"`
	MessageID        string    `json:"message_id"`
	MessageCreatedAt time.Time `json:"message_created_at"`
	EnqueuedAt       time.Time `json:"enqueued_at"`
	Attempt          int       `json:"attempt"`
	Stage            Stage     `json:"stage"`
}

type ConsumedJob struct {
	Job      AnalysisJob
	raw      string
	streamID string
}
