package domain

import "time"

type Sender int

const (
	SenderUser Sender = iota
	SenderAssistant
)

type User struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Session struct {
	Token     string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type Chat struct {
	ID        string
	UserID    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ChatSummary struct {
	ID                 string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	LastMessagePreview string
	LastMessageAt      time.Time
}

type Message struct {
	ID        string
	ChatID    string
	UserID    string
	Sender    Sender
	Content   string
	CreatedAt time.Time
}

type FeelingScore struct {
	Label      string
	Confidence float64
}

type ContextGap string

const (
	ContextGapWhatHappened                 ContextGap = "what_happened"
	ContextGapFeltEmotionsDescribedByUser  ContextGap = "felt_emotions_described_by_user"
	ContextGapUserReaction                 ContextGap = "user_reaction"
	ContextGapExpectedOutcomeOrExpectation ContextGap = "expected_outcome_or_self_expectation"
)

type RiskFlags struct {
	SelfHarm         bool
	SuicidalIdeation bool
	ImmediateDanger  bool
}

type ExtractedEvent struct {
	EnoughContext                    bool
	ContextGaps                      []ContextGap
	EventSummary                     string
	WhatHappened                     string
	FeltEmotionsDescribedByUser      []string
	UserReaction                     string
	ExpectedOutcomeOrSelfExpectation string
	PeopleInvolved                   []string
	Setting                          string
	TimeReference                    string
	RiskFlags                        RiskFlags
	ConfidenceNotes                  string
}

type ClassificationResult struct {
	PrimaryFeeling    FeelingScore
	SecondaryFeelings []FeelingScore
	AllScores         []FeelingScore
	ModelName         string
}

type MessageAnalysis struct {
	ID                  string
	MessageID           string
	ChatID              string
	UserID              string
	SourceText          string
	ClassifierInputText string
	PrimaryFeeling      FeelingScore
	SecondaryFeelings   []FeelingScore
	AllScores           []FeelingScore
	EnoughContext       *bool
	ContextGaps         []ContextGap
	ExtractedEvent      *ExtractedEvent
	ClassifierProvider  string
	ClassifierModel     string
	CreatedAt           time.Time
}

type TimelinePoint struct {
	Date            string
	PrimaryFeeling  string
	SupportingEvent string
}

type DailySummary struct {
	UserID           string
	DayStart         time.Time
	DominantFeelings []FeelingScore
	MainEvents       []string
	TimelinePoints   []TimelinePoint
	GeneratedAt      time.Time
}

type WeeklySummary struct {
	UserID           string
	WeekStart        time.Time
	DominantFeelings []FeelingScore
	MainEvents       []string
	TimelinePoints   []TimelinePoint
	GeneratedAt      time.Time
}

type DashboardTimeline struct {
	From time.Time
	To   time.Time
	Days []DailySummary
}
