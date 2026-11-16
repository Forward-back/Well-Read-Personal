package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gaze/internal/types"
)

func TestReviewDayStartRollsBeforeFourAM(t *testing.T) {
	loc := time.FixedZone("CST", 8*60*60)
	early := time.Date(2026, 11, 14, 2, 30, 0, 0, loc)
	start := ReviewDayStart(early)
	assert.Equal(t, time.Date(2026, 11, 13, 4, 0, 0, 0, loc), start)

	late := time.Date(2026, 11, 14, 10, 30, 0, 0, loc)
	start = ReviewDayStart(late)
	assert.Equal(t, time.Date(2026, 11, 14, 4, 0, 0, 0, loc), start)
}

func TestApplyReviewScheduleRememberedGrowsStability(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	stability, level, due, interval, err := ApplyReviewSchedule(
		ReviewInitialStability, 0, types.ReviewRatingRemembered, now,
	)
	require.NoError(t, err)
	assert.Equal(t, 30.0, stability)
	assert.Equal(t, 1, level)
	assert.Equal(t, 3, interval)
	assert.Equal(t, time.Date(2026, 11, 17, 4, 0, 0, 0, now.Location()), due)
}

func TestApplyReviewScheduleFuzzyGrowsSlowly(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	stability, level, due, interval, err := ApplyReviewSchedule(
		28.48, 2, types.ReviewRatingFuzzy, now,
	)
	require.NoError(t, err)
	assert.InDelta(t, 42.72, stability, 0.001)
	assert.Equal(t, 2, level)
	assert.Equal(t, 5, interval)
	assert.Equal(t, time.Date(2026, 11, 19, 4, 0, 0, 0, now.Location()), due)
}

func TestApplyReviewScheduleForgotResetsStability(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	stability, level, due, interval, err := ApplyReviewSchedule(
		284.75, 5, types.ReviewRatingForgot, now,
	)
	require.NoError(t, err)
	assert.Equal(t, ReviewResetStability, stability)
	assert.Equal(t, 0, level)
	assert.Equal(t, 1, interval)
	assert.Equal(t, time.Date(2026, 11, 15, 4, 0, 0, 0, now.Location()), due)
}

func TestApplyReviewScheduleEasyGrowsFastest(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	stability, level, due, interval, err := ApplyReviewSchedule(
		ReviewInitialStability, 0, types.ReviewRatingEasy, now,
	)
	require.NoError(t, err)
	assert.Equal(t, 50.0, stability)
	assert.Equal(t, 2, level)
	assert.Equal(t, 5, interval)
	assert.Equal(t, time.Date(2026, 11, 19, 4, 0, 0, 0, now.Location()), due)
}

func TestApplyReviewScheduleLevelCapsAtMaximum(t *testing.T) {
	now := time.Now()
	_, level, _, _, err := ApplyReviewSchedule(100000, 5, types.ReviewRatingEasy, now)
	require.NoError(t, err)
	assert.Equal(t, len(types.ReviewScheduleIntervals)-1, level)
}

func TestReviewIntervalDaysCapsAtMaximum(t *testing.T) {
	assert.Equal(t, ReviewMaxIntervalDays, ReviewIntervalDays(100000))
	assert.Equal(t, 1, ReviewIntervalDays(0))
}

func TestApplyReviewScheduleRejectsUnknownRating(t *testing.T) {
	_, _, _, _, err := ApplyReviewSchedule(10, 0, "unknown", time.Now())
	require.Error(t, err)
}
