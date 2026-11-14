package service

import (
	"fmt"
	"time"

	"gaze/internal/types"
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

// ApplyReviewRating advances the simplified SM-2 schedule. It is pure so the
// three-rating rules can be tested without a database.
func ApplyReviewRating(level int, rating string, now time.Time) (int, time.Time, int, error) {
	if level < 0 {
		level = 0
	}
	if level >= len(types.ReviewScheduleIntervals) {
		level = len(types.ReviewScheduleIntervals) - 1
	}
	dayStart := ReviewDayStart(now)
	switch rating {
	case types.ReviewRatingRemembered:
		next := level + 1
		if next >= len(types.ReviewScheduleIntervals) {
			next = len(types.ReviewScheduleIntervals) - 1
		}
		interval := types.ReviewScheduleIntervals[next]
		return next, dayStart.AddDate(0, 0, interval), interval, nil
	case types.ReviewRatingFuzzy:
		interval := (types.ReviewScheduleIntervals[level] + 1) / 2
		if interval < 1 {
			interval = 1
		}
		return level, dayStart.AddDate(0, 0, interval), interval, nil
	case types.ReviewRatingForgot:
		return 0, dayStart.AddDate(0, 0, 1), 1, nil
	default:
		return level, now, 0, fmt.Errorf("unsupported review rating %q", rating)
	}
}
