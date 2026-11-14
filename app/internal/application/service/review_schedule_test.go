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

func TestApplyReviewRatingRememberedAdvancesOneLevel(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	level, due, interval, err := ApplyReviewRating(0, types.ReviewRatingRemembered, now)
	require.NoError(t, err)
	assert.Equal(t, 1, level)
	assert.Equal(t, 3, interval)
	assert.Equal(t, time.Date(2026, 11, 17, 4, 0, 0, 0, now.Location()), due)
}

func TestApplyReviewRatingFuzzyUsesHalfInterval(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	level, due, interval, err := ApplyReviewRating(2, types.ReviewRatingFuzzy, now)
	require.NoError(t, err)
	assert.Equal(t, 2, level)
	assert.Equal(t, 4, interval, "7-day interval rounds up to 4")
	assert.Equal(t, time.Date(2026, 11, 18, 4, 0, 0, 0, now.Location()), due)
}

func TestApplyReviewRatingForgotResetsLevel(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	level, due, interval, err := ApplyReviewRating(5, types.ReviewRatingForgot, now)
	require.NoError(t, err)
	assert.Equal(t, 0, level)
	assert.Equal(t, 1, interval)
	assert.Equal(t, time.Date(2026, 11, 15, 4, 0, 0, 0, now.Location()), due)
}

func TestApplyReviewRatingRejectsUnknownRating(t *testing.T) {
	_, _, _, err := ApplyReviewRating(0, "unknown", time.Now())
	require.Error(t, err)
}
