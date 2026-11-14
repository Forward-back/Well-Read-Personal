package service

import (
	"fmt"
	"math"
	"time"

	"gaze/internal/types"
)

const (
	ReviewInitialStability = 10.0
	ReviewResetStability   = 5.0
	ReviewTargetRetention  = 0.9
	ReviewMaxIntervalDays  = 180
)

// ReviewDayStart returns the start of the review day containing now. The day
// boundary is 04:00 local time: work done at 02:00 counts toward the previous
// day, matching the product rule that late-night study belongs to the prior day.
func ReviewDayStart(now time.Time) time.Time {
	loc := now.Location()
	start := time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, loc)
	if now.Before(start) {
		start = start.AddDate(0, 0, -1)
	}
	return start
}

func ReviewDayEnd(now time.Time) time.Time {
	return ReviewDayStart(now).AddDate(0, 0, 1)
}

// ReviewIntervalDays applies the Ebbinghaus-style exponential forgetting curve:
// retention R(t) = e^(-t/S). The next review is scheduled when predicted
// retention reaches the target (0.9 by default), bounded to 1–180 days.
func ReviewIntervalDays(stability float64) int {
	if stability <= 0 {
		stability = ReviewInitialStability
	}
	days := int(math.Round(-stability * math.Log(ReviewTargetRetention)))
	if days < 1 {
		days = 1
	}
	if days > ReviewMaxIntervalDays {
		days = ReviewMaxIntervalDays
	}
	return days
}

// ApplyReviewSchedule updates one card's stability and next due date. The
// three ratings intentionally stay simple: remembered grows stability, fuzzy
// grows it slowly, forgot resets it to a short interval.
func ApplyReviewSchedule(
	stability float64, level int, rating string, now time.Time,
) (float64, int, time.Time, int, error) {
	if stability <= 0 {
		stability = ReviewInitialStability
	}
	if level < 0 {
		level = 0
	}
	if level >= len(types.ReviewScheduleIntervals) {
		level = len(types.ReviewScheduleIntervals) - 1
	}
	nextLevel := level
	switch rating {
	case types.ReviewRatingRemembered:
		stability *= 3
		nextLevel = level + 1
		if nextLevel >= len(types.ReviewScheduleIntervals) {
			nextLevel = len(types.ReviewScheduleIntervals) - 1
		}
	case types.ReviewRatingFuzzy:
		stability *= 1.5
	case types.ReviewRatingForgot:
		stability = ReviewResetStability
		nextLevel = 0
	default:
		return stability, level, now, 0, fmt.Errorf("unsupported review rating %q", rating)
	}
	if stability < 1 {
		stability = 1
	}
	interval := ReviewIntervalDays(stability)
	due := ReviewDayStart(now).AddDate(0, 0, interval)
	return stability, nextLevel, due, interval, nil
}
