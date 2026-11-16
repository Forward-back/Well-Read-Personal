package service

import (
	"context"
	"sort"
	"strings"
	"time"

	werrors "gaze/internal/errors"
	"gaze/internal/types"
)

const reviewHeatmapUncategorizedKey = "uncategorized"

type reviewHeatmapCard struct {
	card            *types.ReviewCard
	level           string
	lastRating      string
	source          *types.ReviewCardSource
	knowledgeBaseID string
	columnKeys      []string
}

type reviewHeatmapCluster struct {
	rowID     string
	columnKey string
	cards     []*reviewHeatmapCard
}

type reviewHeatmapBuild struct {
	data     *types.ReviewHeatmap
	clusters map[string]*reviewHeatmapCluster
}

func reviewClusterKey(rowID, columnKey string) string {
	return rowID + "\x00" + columnKey
}

// Heatmap returns the server-side vulnerability aggregate for the selected
// subtree. An empty groupID aggregates every group.
func (s *reviewService) Heatmap(ctx context.Context, groupID string) (*types.ReviewHeatmap, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	build, err := s.buildReviewHeatmap(ctx, tenantID, groupID)
	if err != nil {
		return nil, err
	}
	return build.data, nil
}

func (s *reviewService) ClusterDetail(
	ctx context.Context, rowID, columnKey string,
) (*types.ReviewClusterDetail, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	rowID = strings.TrimSpace(rowID)
	columnKey = strings.TrimSpace(columnKey)
	if rowID == "" || columnKey == "" {
		return nil, werrors.NewValidationError("缺少漏洞簇参数")
	}
	build, err := s.buildReviewHeatmap(ctx, tenantID, "")
	if err != nil {
		return nil, err
	}
	var row *types.ReviewHeatmapRow
	for _, item := range build.data.Rows {
		if item.ID == rowID {
			row = item
			break
		}
	}
	var column *types.ReviewHeatmapColumn
	for _, item := range build.data.Columns {
		if item.Key == columnKey {
			column = item
			break
		}
	}
	var cell *types.ReviewHeatmapCell
	for _, item := range build.data.Cells {
		if item.RowID == rowID && item.ColumnKey == columnKey {
			cell = item
			break
		}
	}
	if row == nil || column == nil || cell == nil {
		return nil, werrors.NewNotFoundError("漏洞簇不存在")
	}
	cluster := build.clusters[reviewClusterKey(rowID, columnKey)]
	detail := &types.ReviewClusterDetail{
		Cell:   cell,
		Row:    row,
		Column: column,
		Cards:  []*types.ReviewClusterCard{},
	}
	if cluster == nil {
		return detail, nil
	}
	for _, item := range cluster.cards {
		action := ""
		switch item.level {
		case types.ReviewCardLevelSevere, types.ReviewCardLevelWarning:
			action = "relearn"
		case types.ReviewCardLevelObserve:
			action = "observe"
		}
		detail.Cards = append(detail.Cards, &types.ReviewClusterCard{
			Card:            item.card,
			Level:           item.level,
			LastRating:      item.lastRating,
			SuggestedAction: action,
			Source:          item.source,
			KnowledgeBaseID: item.knowledgeBaseID,
		})
	}
	sort.SliceStable(detail.Cards, func(i, j int) bool {
		left, right := detail.Cards[i], detail.Cards[j]
		if reviewLevelRank(left.Level) != reviewLevelRank(right.Level) {
			return reviewLevelRank(left.Level) > reviewLevelRank(right.Level)
		}
		if left.Card.Stability != right.Card.Stability {
			return left.Card.Stability < right.Card.Stability
		}
		if left.Card.Lapses != right.Card.Lapses {
			return left.Card.Lapses > right.Card.Lapses
		}
		return left.Card.Question < right.Card.Question
	})
	return detail, nil
}

func (s *reviewService) buildReviewHeatmap(
	ctx context.Context, tenantID uint64, groupID string,
) (*reviewHeatmapBuild, error) {
	groups, err := s.repo.ListGroups(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	groupsByID := make(map[string]*types.ReviewGroup, len(groups))
	hasChildren := make(map[string]bool, len(groups))
	children := make(map[string][]string, len(groups))
	for _, group := range groups {
		groupsByID[group.ID] = group
		if group.ParentID != "" {
			children[group.ParentID] = append(children[group.ParentID], group.ID)
			hasChildren[group.ParentID] = true
		}
	}
	allowed := make(map[string]bool, len(groups))
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		for _, group := range groups {
			allowed[group.ID] = true
		}
	} else {
		if groupsByID[groupID] == nil {
			return nil, werrors.NewValidationError("卡组不存在")
		}
		queue := []string{groupID}
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			allowed[id] = true
			queue = append(queue, children[id]...)
		}
	}

	allCards, err := s.repo.ListAllCards(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	activeCards := make([]*types.ReviewCard, 0, len(allCards))
	for _, card := range allCards {
		if card.Status == types.ReviewStatusActive && allowed[card.GroupID] {
			activeCards = append(activeCards, card)
		}
	}
	data := &types.ReviewHeatmap{
		Rows:       []*types.ReviewHeatmapRow{},
		Columns:    []*types.ReviewHeatmapColumn{},
		Cells:      []*types.ReviewHeatmapCell{},
		Priorities: []*types.ReviewHeatmapCell{},
	}
	build := &reviewHeatmapBuild{
		data:     data,
		clusters: make(map[string]*reviewHeatmapCluster),
	}
	if len(activeCards) == 0 {
		return build, nil
	}

	cardIDs := make([]string, 0, len(activeCards))
	cardByID := make(map[string]*types.ReviewCard, len(activeCards))
	for _, card := range activeCards {
		cardIDs = append(cardIDs, card.ID)
		cardByID[card.ID] = card
	}
	allSources, err := s.repo.ListAllSources(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	sourcesByCard := make(map[string][]*types.ReviewCardSource, len(activeCards))
	knowledgeIDs := make(map[string]bool)
	for _, source := range allSources {
		if cardByID[source.CardID] == nil {
			continue
		}
		if source.SourceType != types.ReviewSourceTypeKnowledge ||
			source.Status != types.ReviewSourceStatusActive {
			continue
		}
		sourcesByCard[source.CardID] = append(sourcesByCard[source.CardID], source)
		if source.KnowledgeID != "" {
			knowledgeIDs[source.KnowledgeID] = true
		}
	}
	for cardID := range sourcesByCard {
		sort.SliceStable(sourcesByCard[cardID], func(i, j int) bool {
			return sourcesByCard[cardID][i].IsPrimary && !sourcesByCard[cardID][j].IsPrimary
		})
	}
	knowledgeTitles, knowledgeBases := s.reviewKnowledgeMeta(ctx, tenantID, knowledgeIDs)

	logs, err := s.repo.ListLogsByCardIDs(ctx, tenantID, cardIDs)
	if err != nil {
		return nil, err
	}
	logsByCard := groupReviewLogsByCard(logs)
	now := time.Now()

	columns := make(map[string]*types.ReviewHeatmapColumn)
	rows := make(map[string]*types.ReviewHeatmapRow)
	rowIDByGroup := make(map[string]string, len(groupsByID))
	heatCards := make(map[string]*reviewHeatmapCard, len(activeCards))

	for _, card := range activeCards {
		cardLogs := logsByCard[card.ID]
		level := reviewCardLevel(card, cardLogs, now)
		card.Vulnerability = level
		var source *types.ReviewCardSource
		if list := sourcesByCard[card.ID]; len(list) > 0 {
			source = list[0]
		}
		item := &reviewHeatmapCard{
			card:       card,
			level:      level,
			lastRating: reviewLastRating(cardLogs),
			source:     source,
		}
		if source != nil {
			item.knowledgeBaseID = knowledgeBases[source.KnowledgeID]
		}

		rowID := rowIDByGroup[card.GroupID]
		if rowID == "" {
			group := groupsByID[card.GroupID]
			name := ""
			if group != nil {
				name = group.Name
			}
			rowID = card.GroupID
			virtual := hasChildren[card.GroupID]
			if virtual {
				rowID += ":virtual"
			}
			rowIDByGroup[card.GroupID] = rowID
			rows[rowID] = &types.ReviewHeatmapRow{
				ID:      rowID,
				GroupID: card.GroupID,
				Name:    name,
				Path:    reviewGroupPath(groupsByID, card.GroupID),
				Virtual: virtual,
			}
		}

		tags := reviewTagsFromJSON(card.Tags)
		if len(tags) > 0 {
			seen := make(map[string]bool, len(tags))
			for _, tag := range tags {
				tag = strings.TrimSpace(tag)
				if tag == "" {
					continue
				}
				key := types.ReviewHeatmapColumnKindTag + ":" + strings.ToLower(tag)
				if seen[key] {
					continue
				}
				seen[key] = true
				if columns[key] == nil {
					columns[key] = &types.ReviewHeatmapColumn{
						Key:   key,
						Title: tag,
						Kind:  types.ReviewHeatmapColumnKindTag,
					}
				}
				item.columnKeys = append(item.columnKeys, key)
			}
		}
		if len(item.columnKeys) == 0 && source != nil && source.KnowledgeID != "" {
			key := types.ReviewHeatmapColumnKindSource + ":" + source.KnowledgeID
			if columns[key] == nil {
				title := knowledgeTitles[source.KnowledgeID]
				if title == "" {
					title = reviewSourceTitleFallback(source)
				}
				columns[key] = &types.ReviewHeatmapColumn{
					Key:             key,
					Title:           title,
					Kind:            types.ReviewHeatmapColumnKindSource,
					SourceID:        source.KnowledgeID,
					KnowledgeBaseID: knowledgeBases[source.KnowledgeID],
				}
			}
			item.columnKeys = append(item.columnKeys, key)
		}
		if len(item.columnKeys) == 0 {
			key := reviewHeatmapUncategorizedKey
			if columns[key] == nil {
				columns[key] = &types.ReviewHeatmapColumn{
					Key:   key,
					Title: "未分类",
					Kind:  types.ReviewHeatmapColumnKindUncategorized,
				}
			}
			item.columnKeys = append(item.columnKeys, key)
		}
		rows[rowID].TotalCards++
		heatCards[card.ID] = item
	}

	cellCounts := make(map[string]*types.ReviewHeatmapCell)
	cellWeights := make(map[string]float64)
	cellReviewed := make(map[string]int)
	cellHighest := make(map[string]string)
	for _, card := range activeCards {
		item := heatCards[card.ID]
		if item == nil {
			continue
		}
		rowID := rowIDByGroup[card.GroupID]
		for _, columnKey := range item.columnKeys {
			key := reviewClusterKey(rowID, columnKey)
			cell := cellCounts[key]
			if cell == nil {
				cell = &types.ReviewHeatmapCell{RowID: rowID, ColumnKey: columnKey}
				cellCounts[key] = cell
			}
			cluster := build.clusters[key]
			if cluster == nil {
				cluster = &reviewHeatmapCluster{rowID: rowID, columnKey: columnKey}
				build.clusters[key] = cluster
			}
			cluster.cards = append(cluster.cards, item)
			cell.Total++
			switch item.level {
			case types.ReviewCardLevelSevere:
				cell.Severe++
			case types.ReviewCardLevelWarning:
				cell.Warning++
			case types.ReviewCardLevelObserve:
				cell.Observe++
			case types.ReviewCardLevelHealthy:
				cell.Healthy++
			default:
				cell.NoData++
			}
			if item.level != types.ReviewCardLevelNone {
				cellReviewed[key]++
			}
			cellWeights[key] += reviewLevelWeight(item.level)
			cellHighest[key] = maxReviewLevel(cellHighest[key], item.level)
		}
	}

	columnMaxLevel := make(map[string]string, len(columns))
	columnIntensity := make(map[string]float64, len(columns))
	for key, cell := range cellCounts {
		if cell.Total > 0 {
			cell.Intensity = cellWeights[key] / float64(cell.Total)
		}
		cell.Level = reviewLevelFromIntensity(cell.Intensity, cellReviewed[key])
		cell.Level = maxReviewLevel(cell.Level, cellHighest[key])
		cell.Clickable = cell.Level == types.ReviewCardLevelSevere ||
			cell.Level == types.ReviewCardLevelWarning ||
			cell.Level == types.ReviewCardLevelObserve
		data.Cells = append(data.Cells, cell)
		columnMaxLevel[cell.ColumnKey] = maxReviewLevel(columnMaxLevel[cell.ColumnKey], cell.Level)
		columnIntensity[cell.ColumnKey] += cell.Intensity
		if cell.Level == types.ReviewCardLevelObserve {
			data.ObserveCount++
		}
	}

	rowList := make([]*types.ReviewHeatmapRow, 0, len(rows))
	for _, row := range rows {
		rowList = append(rowList, row)
	}
	sort.Slice(rowList, func(i, j int) bool {
		if rowList[i].Path != rowList[j].Path {
			return rowList[i].Path < rowList[j].Path
		}
		return rowList[i].ID < rowList[j].ID
	})
	data.Rows = rowList

	columnList := make([]*types.ReviewHeatmapColumn, 0, len(columns))
	for _, column := range columns {
		columnList = append(columnList, column)
	}
	sort.Slice(columnList, func(i, j int) bool {
		left, right := columnList[i], columnList[j]
		if reviewLevelRank(columnMaxLevel[left.Key]) != reviewLevelRank(columnMaxLevel[right.Key]) {
			return reviewLevelRank(columnMaxLevel[left.Key]) > reviewLevelRank(columnMaxLevel[right.Key])
		}
		if columnIntensity[left.Key] != columnIntensity[right.Key] {
			return columnIntensity[left.Key] > columnIntensity[right.Key]
		}
		return left.Title < right.Title
	})
	data.Columns = columnList

	sort.Slice(data.Cells, func(i, j int) bool {
		left, right := data.Cells[i], data.Cells[j]
		if left.RowID != right.RowID {
			return left.RowID < right.RowID
		}
		return left.ColumnKey < right.ColumnKey
	})

	rowPath := make(map[string]string, len(rowList))
	for _, row := range rowList {
		rowPath[row.ID] = row.Path
	}
	columnTitle := make(map[string]string, len(columns))
	for _, column := range columnList {
		columnTitle[column.Key] = column.Title
	}
	for _, cell := range data.Cells {
		if cell.Level != types.ReviewCardLevelSevere && cell.Level != types.ReviewCardLevelWarning {
			continue
		}
		data.Priorities = append(data.Priorities, cell)
	}
	sort.SliceStable(data.Priorities, func(i, j int) bool {
		left, right := data.Priorities[i], data.Priorities[j]
		if left.Intensity != right.Intensity {
			return left.Intensity > right.Intensity
		}
		if left.Severe != right.Severe {
			return left.Severe > right.Severe
		}
		if left.Warning != right.Warning {
			return left.Warning > right.Warning
		}
		if rowPath[left.RowID] != rowPath[right.RowID] {
			return rowPath[left.RowID] < rowPath[right.RowID]
		}
		return columnTitle[left.ColumnKey] < columnTitle[right.ColumnKey]
	})
	if len(data.Priorities) > 3 {
		data.Priorities = data.Priorities[:3]
	}
	data.Insight = reviewHeatmapInsight(data.Cells, rowPath, columnTitle)
	return build, nil
}

func reviewHeatmapInsight(
	cells []*types.ReviewHeatmapCell, rowPath, columnTitle map[string]string,
) *types.ReviewHeatmapInsight {
	rowSevere := make(map[string]int)
	columnSevere := make(map[string]int)
	for _, cell := range cells {
		if cell.Level != types.ReviewCardLevelSevere {
			continue
		}
		// The insight counts severe clusters, not severe cards: a row with
		// two red cells is a multi-topic warning even if each cell holds one card.
		rowSevere[cell.RowID]++
		columnSevere[cell.ColumnKey]++
	}
	bestRowID, bestRowSevere := "", 0
	for rowID, count := range rowSevere {
		if count < 2 {
			continue
		}
		if count > bestRowSevere || (count == bestRowSevere && rowID < bestRowID) ||
			bestRowID == "" {
			bestRowID, bestRowSevere = rowID, count
		}
	}
	bestColumnKey, bestColumnSevere := "", 0
	for columnKey, count := range columnSevere {
		if count < 2 {
			continue
		}
		if count > bestColumnSevere || (count == bestColumnSevere && columnKey < bestColumnKey) ||
			bestColumnKey == "" {
			bestColumnKey, bestColumnSevere = columnKey, count
		}
	}
	if bestRowID == "" && bestColumnKey == "" {
		return nil
	}
	if bestRowID != "" && bestRowSevere >= bestColumnSevere {
		name := rowPath[bestRowID]
		if name == "" {
			name = bestRowID
		}
		return &types.ReviewHeatmapInsight{
			Kind:    "row",
			RowID:   bestRowID,
			Message: "「" + name + "」多个子主题同时告急",
		}
	}
	title := columnTitle[bestColumnKey]
	if title == "" {
		title = bestColumnKey
	}
	return &types.ReviewHeatmapInsight{
		Kind:      "column",
		ColumnKey: bestColumnKey,
		Message:   "「" + title + "」主题跨卡组普遍薄弱",
	}
}

func (s *reviewService) reviewKnowledgeMeta(
	ctx context.Context, tenantID uint64, ids map[string]bool,
) (map[string]string, map[string]string) {
	titles := make(map[string]string, len(ids))
	bases := make(map[string]string, len(ids))
	if s.knowledgeResolver == nil || len(ids) == 0 {
		return titles, bases
	}
	list := make([]string, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	items, err := s.knowledgeResolver.GetKnowledgeBatch(ctx, tenantID, list)
	if err != nil {
		return titles, bases
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		titles[item.ID] = item.Title
		bases[item.ID] = item.KnowledgeBaseID
	}
	return titles, bases
}

func reviewSourceTitleFallback(source *types.ReviewCardSource) string {
	path := strings.TrimSpace(source.Path)
	if path == "" {
		return "未命名来源"
	}
	if index := strings.LastIndexAny(path, "/\\"); index >= 0 && index+1 < len(path) {
		return path[index+1:]
	}
	return path
}

func reviewGroupPath(groupsByID map[string]*types.ReviewGroup, groupID string) string {
	names := make([]string, 0, 4)
	visited := make(map[string]bool, 4)
	for cursor := groupID; cursor != "" && !visited[cursor]; {
		visited[cursor] = true
		group := groupsByID[cursor]
		if group == nil {
			break
		}
		names = append(names, group.Name)
		cursor = group.ParentID
	}
	for left, right := 0, len(names)-1; left < right; left, right = left+1, right-1 {
		names[left], names[right] = names[right], names[left]
	}
	return strings.Join(names, " / ")
}
