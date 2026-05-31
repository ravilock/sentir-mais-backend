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

type TimelinePoint struct {
	Date            string
	PrimaryFeeling  string
	SupportingEvent string
}

type WeeklySummary struct {
	UserID           string
	WeekStart        time.Time
	DominantFeelings []FeelingScore
	MainEvents       []string
	TimelinePoints   []TimelinePoint
	GeneratedAt      time.Time
}
