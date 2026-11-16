package service

import (
	"context"
	"math"
	"sort"
	"time"

	"gaze/internal/types"
)

type reviewCountBucket struct {
	easy       int64
	remembered int64
	fuzzy      int64
	forgot     int64
	total      int64
}

func (b *reviewCountBucket) add(rating string) {
	b.total++
	switch rating {
	case types.ReviewRatingEasy:
		b.easy++
	case types.ReviewRatingRemembered:
		b.remembered++
	case types.ReviewRatingFuzzy:
		b.fuzzy++
	case types.ReviewRatingForgot:
		b.forgot++
	}
}

func (b *reviewCountBucket) success() int64 {
	return b.easy + b.remembered
}

// Stats returns the review dashboard aggregate for the requested day window.
// Only 7, 30 and 90 day windows are accepted; anything else falls back to 30.
func (s *reviewService) Stats(ctx context.Context, days int) (*types.ReviewStats, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	if days != 7 && days != 30 && days != 90 {
		days = 30
	}
	cards, err := s.repo.ListAllCards(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	logs, err := s.repo.ListAllLogs(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	groups, err := s.repo.ListGroups(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	today := ReviewDayStart(now)
	cutoff := today.AddDate(0, 0, -(days - 1))
	cutoff30 := today.AddDate(0, 0, -29)

	groupNames := make(map[string]string, len(groups))
	for _, group := range groups {
		groupNames[group.ID] = group.Name
	}
	cardByID := make(map[string]*types.ReviewCard, len(cards))
	cardGroups := make(map[string]string, len(cards))
	for _, card := range cards {
		cardByID[card.ID] = card
		cardGroups[card.ID] = card.GroupID
	}

	groupBuckets := make(map[string]*reviewCountBucket)
	cardBuckets := make(map[string]*reviewCountBucket)
	trend := make([]*types.ReviewRatingTrend, 0, days)
	trendByDay := make(map[string]*types.ReviewRatingTrend, days)
	for offset := 0; offset < days; offset++ {
		date := today.AddDate(0, 0, offset-(days-1)).Format("2006-01-02")
		point := &types.ReviewRatingTrend{Date: date}
		trend = append(trend, point)
		trendByDay[date] = point
	}

	stats := &types.ReviewStats{
		Days:            days,
		TotalCards:      int64(len(cards)),
		RatingTrend:     trend,
		DueForecast:     make([]*types.ReviewDueForecast, 0, 7),
		DifficultCards:  make([]*types.ReviewCardStat, 0),
		GroupRanking:    make([]*types.ReviewGroupStat, 0),
		ForgettingCurve: make([]*types.ReviewForgettingPoint, 0),
		WeakTopics:      make([]*types.ReviewWeakTopic, 0),
	}

	for _, log := range logs {
		if !log.ReviewedAt.Before(cutoff30) {
			stats.Reviewed30d++
			if log.Rating == types.ReviewRatingRemembered {
				stats.Remembered30d++
			}
			if log.Rating == types.ReviewRatingEasy {
				stats.Easy30d++
			}
		}
		if log.ReviewedAt.Before(cutoff) {
			continue
		}
		stats.Reviewed++
		if log.Rating == types.ReviewRatingRemembered {
			stats.Remembered++
		}
		switch log.Rating {
		case types.ReviewRatingEasy:
			stats.RatingDistribution.Easy++
		case types.ReviewRatingRemembered:
			stats.RatingDistribution.Remembered++
		case types.ReviewRatingFuzzy:
			stats.RatingDistribution.Fuzzy++
		case types.ReviewRatingForgot:
			stats.RatingDistribution.Forgot++
		}
		stats.RatingDistribution.Total++

		groupID := cardGroups[log.CardID]
		if groupID != "" {
			groupBucket := groupBuckets[groupID]
			if groupBucket == nil {
				groupBucket = &reviewCountBucket{}
				groupBuckets[groupID] = groupBucket
			}
			groupBucket.add(log.Rating)
		}
		cardBucket := cardBuckets[log.CardID]
		if cardBucket == nil {
			cardBucket = &reviewCountBucket{}
			cardBuckets[log.CardID] = cardBucket
		}
		cardBucket.add(log.Rating)

		if point := trendByDay[ReviewDayStart(log.ReviewedAt).Format("2006-01-02")]; point != nil {
			point.Total++
			switch log.Rating {
			case types.ReviewRatingEasy:
				point.Easy++
			case types.ReviewRatingRemembered:
				point.Remembered++
			case types.ReviewRatingFuzzy:
				point.Fuzzy++
			case types.ReviewRatingForgot:
				point.Forgot++
			}
		}
	}

	if stats.Reviewed > 0 {
		success := stats.RatingDistribution.Easy + stats.RatingDistribution.Remembered
		stats.Retention = float64(success) / float64(stats.Reviewed)
	}
	if stats.Reviewed30d > 0 {
		success := stats.Easy30d + stats.Remembered30d
		stats.Retention30d = float64(success) / float64(stats.Reviewed30d)
	}
	for _, point := range stats.RatingTrend {
		if point.Total > 0 {
			point.Retention = float64(point.Easy+point.Remembered) / float64(point.Total)
		}
	}

	for groupID, bucket := range groupBuckets {
		retention := 0.0
		if bucket.total > 0 {
			retention = float64(bucket.success()) / float64(bucket.total)
		}
		stats.GroupRanking = append(stats.GroupRanking, &types.ReviewGroupStat{
			GroupID:   groupID,
			GroupName: groupNames[groupID],
			Reviewed:  bucket.total,
			Retention: retention,
		})
		if bucket.total >= 5 && retention < 0.6 {
			stats.WeakTopics = append(stats.WeakTopics, &types.ReviewWeakTopic{
				GroupID:   groupID,
				GroupName: groupNames[groupID],
				Reviewed:  bucket.total,
				Retention: retention,
			})
		}
	}
	sort.Slice(stats.GroupRanking, func(i, j int) bool {
		if stats.GroupRanking[i].Reviewed != stats.GroupRanking[j].Reviewed {
			return stats.GroupRanking[i].Reviewed > stats.GroupRanking[j].Reviewed
		}
		return stats.GroupRanking[i].GroupName < stats.GroupRanking[j].GroupName
	})
	sort.Slice(stats.WeakTopics, func(i, j int) bool {
		if stats.WeakTopics[i].Retention != stats.WeakTopics[j].Retention {
			return stats.WeakTopics[i].Retention < stats.WeakTopics[j].Retention
		}
		return stats.WeakTopics[i].Reviewed > stats.WeakTopics[j].Reviewed
	})

	for cardID, bucket := range cardBuckets {
		card := cardByID[cardID]
		if card == nil || bucket.total == 0 {
			continue
		}
		stats.DifficultCards = append(stats.DifficultCards, &types.ReviewCardStat{
			CardID:       card.ID,
			Question:     card.Question,
			GroupID:      card.GroupID,
			GroupName:    groupNames[card.GroupID],
			Easy:         bucket.easy,
			Remembered:   bucket.remembered,
			Fuzzy:        bucket.fuzzy,
			Forgot:       bucket.forgot,
			Total:        bucket.total,
			ForgetRate:   float64(bucket.forgot) / float64(bucket.total),
			Lapses:       card.Lapses,
			ForgetStreak: card.ForgetStreak,
		})
	}
	sort.Slice(stats.DifficultCards, func(i, j int) bool {
		if stats.DifficultCards[i].Forgot != stats.DifficultCards[j].Forgot {
			return stats.DifficultCards[i].Forgot > stats.DifficultCards[j].Forgot
		}
		if stats.DifficultCards[i].ForgetRate != stats.DifficultCards[j].ForgetRate {
			return stats.DifficultCards[i].ForgetRate > stats.DifficultCards[j].ForgetRate
		}
		return stats.DifficultCards[i].Lapses > stats.DifficultCards[j].Lapses
	})
	if len(stats.DifficultCards) > 10 {
		stats.DifficultCards = stats.DifficultCards[:10]
	}

	dueCounts := make([]int64, 7)
	for _, card := range cards {
		if card.Status != types.ReviewStatusActive {
			continue
		}
		dueDay := ReviewDayStart(card.DueAt)
		if dueDay.Before(today) {
			dueDay = today
		}
		offset := int(dueDay.Sub(today).Hours() / 24)
		if offset >= 0 && offset < len(dueCounts) {
			dueCounts[offset]++
		}
	}
	for offset, count := range dueCounts {
		stats.DueForecast = append(stats.DueForecast, &types.ReviewDueForecast{
			Date:  today.AddDate(0, 0, offset).Format("2006-01-02"),
			Count: count,
		})
	}

	stats.StreakDays = reviewStreakDays(logs, today)
	stats.ForgettingCurve = reviewForgettingCurve(logs)
	return stats, nil
}

func reviewStreakDays(logs []*types.ReviewLog, today time.Time) int {
	days := make(map[string]bool)
	for _, log := range logs {
		days[ReviewDayStart(log.ReviewedAt).Format("2006-01-02")] = true
	}
	cursor := today
	if !days[cursor.Format("2006-01-02")] {
		cursor = cursor.AddDate(0, 0, -1)
	}
	streak := 0
	for days[cursor.Format("2006-01-02")] {
		streak++
		cursor = cursor.AddDate(0, 0, -1)
	}
	return streak
}

func reviewForgettingCurve(logs []*types.ReviewLog) []*types.ReviewForgettingPoint {
	type curveBucket struct {
		sample      int64
		hits        int64
		theoretical float64
	}
	buckets := make([]*curveBucket, 31)
	lastByCard := make(map[string]time.Time, len(logs))
	for _, log := range logs {
		previous, seen := lastByCard[log.CardID]
		if seen {
			elapsed := log.ReviewedAt.Sub(previous).Hours() / 24
			if elapsed >= 0 && elapsed <= 30 {
				index := int(math.Round(elapsed))
				if index < 0 {
					index = 0
				}
				if index > 30 {
					index = 30
				}
				if buckets[index] == nil {
					buckets[index] = &curveBucket{}
				}
				bucket := buckets[index]
				bucket.sample++
				if log.Rating == types.ReviewRatingRemembered || log.Rating == types.ReviewRatingEasy {
					bucket.hits++
				}
				stability := log.StabilityBefore
				if stability <= 0 {
					stability = ReviewInitialStability
				}
				bucket.theoretical += math.Exp(-elapsed / stability)
			}
		}
		lastByCard[log.CardID] = log.ReviewedAt
	}
	points := make([]*types.ReviewForgettingPoint, 0, 31)
	for day, bucket := range buckets {
		if bucket == nil || bucket.sample == 0 {
			continue
		}
		points = append(points, &types.ReviewForgettingPoint{
			Day:         day,
			Actual:      float64(bucket.hits) / float64(bucket.sample),
			Theoretical: bucket.theoretical / float64(bucket.sample),
			Sample:      bucket.sample,
		})
	}
	return points
}
