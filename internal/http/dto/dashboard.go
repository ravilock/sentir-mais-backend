package dto

import "time"

type FeelingScoreResponse struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
}

type TimelinePointResponse struct {
	Date            string `json:"date"`
	PrimaryFeeling  string `json:"primaryFeeling"`
	SupportingEvent string `json:"supportingEvent"`
}

type WeeklySummaryResponse struct {
	WeekStart        time.Time               `json:"weekStart"`
	DominantFeelings []FeelingScoreResponse  `json:"dominantFeelings"`
	MainEvents       []string                `json:"mainEvents"`
	TimelinePoints   []TimelinePointResponse `json:"timelinePoints"`
	GeneratedAt      time.Time               `json:"generatedAt"`
}
