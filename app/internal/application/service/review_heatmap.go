package service

import (
	"context"
	"sort"
	"strings"
	"time"

	werrors "gaze/internal/errors"
	"gaze/internal/types"
)

const (
	reviewHeatmapUncategorizedKey = "uncategorized"
	reviewHeatmapDefaultRowLimit  = 7
	reviewHeatmapDefaultColLimit  = 7
	reviewHeatmapExpandedRowLimit = 50
	reviewHeatmapExpandedColLimit = 30
)

type reviewHeatmapCard struct {
	card            *types.ReviewCard
	level           string
	reason          string
	lastRating      string
	source          *types.ReviewCardSource
	knowledgeBaseID string
	rowID           string
	columnKeys      []string
}

type reviewHeatmapBuild struct {
	data     *types.ReviewHeatmap
	rowCards map[string][]*reviewHeatmapCard
}

func reviewClusterKey(rowID, columnKey string) string {
	return rowID + "\x00" + columnKey
}

// Heatmap returns the server-side vulnerability aggregate for the selected
// scope. Rows are the next group layer; columns are tags with a knowledge-base
// fallback. Top-N rows/columns are folded into explicit other buckets.
func (s *reviewService) Heatmap(
	ctx context.Context, opts *types.ReviewHeatmapOptions,
) (*types.ReviewHeatmap, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	if err := s.resumeExpiredSuspensions(ctx, tenantID); err != nil {
		return nil, err
	}
	if opts == nil {
		opts = &types.ReviewHeatmapOptions{}
	}
	build, err := s.buildReviewHeatmap(ctx, tenantID, strings.TrimSpace(opts.GroupID), opts)
	if err != nil {
		return nil, err
	}
	return build.data, nil
}

// ClusterDetail returns one cluster without rebuilding every row/column. The
// row group and column key come from the standalone cluster page URL.
func (s *reviewService) ClusterDetail(
	ctx context.Context, req *types.ReviewClusterDetailRequest,
) (*types.ReviewClusterDetail, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	if err := s.resumeExpiredSuspensions(ctx, tenantID); err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.RowGroupID) == "" || strings.TrimSpace(req.ColumnKey) == "" {
		return nil, werrors.NewValidationError("缺少漏洞簇参数")
	}
	rowGroupID := strings.TrimSpace(req.RowGroupID)
	columnKey := strings.TrimSpace(req.ColumnKey)
	if rowGroupID == types.ReviewHeatmapOtherRowID || columnKey == types.ReviewHeatmapOtherColumnKey {
		return nil, werrors.NewValidationError("聚合桶不能打开详情")
	}
	groups, err := s.repo.ListGroups(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	groupsByID := make(map[string]*types.ReviewGroup, len(groups))
	children := make(map[string][]string, len(groups))
	for _, group := range groups {
		groupsByID[group.ID] = group
		if group.ParentID != "" {
			children[group.ParentID] = append(children[group.ParentID], group.ID)
		}
	}
	rowGroup := groupsByID[rowGroupID]
	if rowGroup == nil {
		return nil, werrors.NewNotFoundError("漏洞簇不存在")
	}
	if scopeID := strings.TrimSpace(req.ScopeGroupID); scopeID != "" && groupsByID[scopeID] == nil {
		return nil, werrors.NewValidationError("卡组不存在")
	}

	groupIDs := []string{rowGroupID}
	if !req.Direct {
		groupIDs = reviewSubtreeGroupIDs(rowGroupID, children)
	}
	cards, err := s.repo.ListActiveCardsByGroupIDs(ctx, tenantID, groupIDs)
	if err != nil {
		return nil, err
	}
	if len(cards) == 0 {
		return nil, werrors.NewNotFoundError("漏洞簇不存在")
	}
	cardIDs := make([]string, 0, len(cards))
	cardByID := make(map[string]*types.ReviewCard, len(cards))
	for _, card := range cards {
		cardIDs = append(cardIDs, card.ID)
		cardByID[card.ID] = card
	}
	allSources, err := s.repo.ListActiveSourcesByCardIDs(ctx, tenantID, cardIDs)
	if err != nil {
		return nil, err
	}
	sourcesByCard := reviewSourcesByCard(allSources, cardByID)
	knowledgeIDs := make(map[string]bool)
	for _, list := range sourcesByCard {
		for _, source := range list {
			if source.KnowledgeID != "" {
				knowledgeIDs[source.KnowledgeID] = true
			}
		}
	}
	titles, bases := s.reviewKnowledgeMeta(ctx, tenantID, knowledgeIDs)
	kbIDs := make(map[string]bool, len(bases))
	for _, kbID := range bases {
		if kbID != "" {
			kbIDs[kbID] = true
		}
	}
	kbNames := s.reviewKnowledgeBaseNames(ctx, tenantID, kbIDs)
	logs, err := s.repo.ListLogsByCardIDs(ctx, tenantID, cardIDs)
	if err != nil {
		return nil, err
	}
	logsByCard := groupReviewLogsByCard(logs)
	now := time.Now()

	row := &types.ReviewHeatmapRow{
		ID:      rowGroupID,
		GroupID: rowGroupID,
		Name:    rowGroup.Name,
		Path:    reviewGroupPath(groupsByID, rowGroupID),
		Kind:    types.ReviewHeatmapRowKindGroup,
	}
	if req.Direct {
		row = &types.ReviewHeatmapRow{
			ID:        rowGroupID + ":direct",
			GroupID:   rowGroupID,
			Name:      "本组卡片",
			Path:      reviewGroupPath(groupsByID, rowGroupID) + " / 本组卡片",
			Kind:      types.ReviewHeatmapRowKindDirect,
			Virtual:   true,
			Drillable: false,
		}
	}

	column := reviewColumnForDetail(columnKey, cards, sourcesByCard, titles, bases, kbNames)
	matched := make([]*reviewHeatmapCard, 0, len(cards))
	for _, card := range cards {
		item := &reviewHeatmapCard{
			card:       card,
			level:      card.Vulnerability,
			reason:     card.VulnerabilityReason,
			lastRating: reviewLastRating(logsByCard[card.ID]),
			rowID:      row.ID,
			columnKeys: []string{columnKey},
		}
		if source := reviewPrimarySource(sourcesByCard[card.ID]); source != nil {
			item.source = source
			item.knowledgeBaseID = bases[source.KnowledgeID]
		}
		if !reviewCardMatchesColumn(item, columnKey) {
			continue
		}
		level, reason := reviewCardLevelDetail(card, logsByCard[card.ID], now)
		item.level = level
		item.reason = reason
		card.Vulnerability = level
		card.VulnerabilityReason = reason
		matched = append(matched, item)
	}
	cell := reviewCellFromCards(matched, row.ID, columnKey, false)
	if cell == nil {
		cell = &types.ReviewHeatmapCell{RowID: row.ID, ColumnKey: columnKey, Level: types.ReviewCardLevelNone}
	}
	row.TotalCards = len(cards)
	reasons := make([]string, 0, 4)
	seenReasons := make(map[string]bool, 4)
	for _, item := range matched {
		if item.reason == "" || seenReasons[item.reason] {
			continue
		}
		seenReasons[item.reason] = true
		reasons = append(reasons, item.reason)
	}
	sort.SliceStable(matched, func(i, j int) bool {
		left, right := matched[i], matched[j]
		if reviewLevelRank(left.level) != reviewLevelRank(right.level) {
			return reviewLevelRank(left.level) > reviewLevelRank(right.level)
		}
		if left.card.Stability != right.card.Stability {
			return left.card.Stability < right.card.Stability
		}
		if left.card.Lapses != right.card.Lapses {
			return left.card.Lapses > right.card.Lapses
		}
		return left.card.Question < right.card.Question
	})
	detail := &types.ReviewClusterDetail{
		Cell:         cell,
		Row:          row,
		Column:       column,
		Cards:        make([]*types.ReviewClusterCard, 0, len(matched)),
		Reasons:      reasons,
		Formula:      reviewHeatmapFormula(cell.Total),
		ScopeGroupID: strings.TrimSpace(req.ScopeGroupID),
	}
	for _, item := range matched {
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
	return detail, nil
}

func (s *reviewService) buildReviewHeatmap(
	ctx context.Context, tenantID uint64, scopeID string, opts *types.ReviewHeatmapOptions,
) (*reviewHeatmapBuild, error) {
	groups, err := s.repo.ListGroups(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	groupsByID := make(map[string]*types.ReviewGroup, len(groups))
	children := make(map[string][]string, len(groups))
	hasChildren := make(map[string]bool, len(groups))
	for _, group := range groups {
		groupsByID[group.ID] = group
		if group.ParentID != "" {
			children[group.ParentID] = append(children[group.ParentID], group.ID)
			hasChildren[group.ParentID] = true
		}
	}
	allowed := make(map[string]bool, len(groups))
	scopePath := "全部卡组"
	if scopeID == "" {
		for _, group := range groups {
			allowed[group.ID] = true
		}
	} else {
		if groupsByID[scopeID] == nil {
			return nil, werrors.NewValidationError("卡组不存在")
		}
		for _, id := range reviewSubtreeGroupIDs(scopeID, children) {
			allowed[id] = true
		}
		scopePath = reviewGroupPath(groupsByID, scopeID)
	}
	build := &reviewHeatmapBuild{
		data: &types.ReviewHeatmap{
			Rows:         []*types.ReviewHeatmapRow{},
			Columns:      []*types.ReviewHeatmapColumn{},
			Cells:        []*types.ReviewHeatmapCell{},
			Priorities:   []*types.ReviewHeatmapCell{},
			ScopeGroupID: scopeID,
			ScopePath:    scopePath,
			Formula:      reviewHeatmapFormula(0),
		},
		rowCards: make(map[string][]*reviewHeatmapCard),
	}
	groupIDs := make([]string, 0, len(allowed))
	for id := range allowed {
		groupIDs = append(groupIDs, id)
	}
	if len(groupIDs) == 0 {
		return build, nil
	}
	cards, err := s.repo.ListActiveCardsByGroupIDs(ctx, tenantID, groupIDs)
	if err != nil {
		return nil, err
	}
	if len(cards) == 0 {
		return build, nil
	}
	cardIDs := make([]string, 0, len(cards))
	cardByID := make(map[string]*types.ReviewCard, len(cards))
	for _, card := range cards {
		if card.Status != types.ReviewStatusActive || !allowed[card.GroupID] {
			continue
		}
		cardIDs = append(cardIDs, card.ID)
		cardByID[card.ID] = card
	}
	allSources, err := s.repo.ListActiveSourcesByCardIDs(ctx, tenantID, cardIDs)
	if err != nil {
		return nil, err
	}
	sourcesByCard := reviewSourcesByCard(allSources, cardByID)
	knowledgeIDs := make(map[string]bool)
	kbIDs := make(map[string]bool)
	for _, list := range sourcesByCard {
		for _, source := range list {
			if source.KnowledgeID != "" {
				knowledgeIDs[source.KnowledgeID] = true
			}
		}
	}
	_, bases := s.reviewKnowledgeMeta(ctx, tenantID, knowledgeIDs)
	for _, kbID := range bases {
		if kbID != "" {
			kbIDs[kbID] = true
		}
	}
	kbNames := s.reviewKnowledgeBaseNames(ctx, tenantID, kbIDs)
	logs, err := s.repo.ListLogsByCardIDs(ctx, tenantID, cardIDs)
	if err != nil {
		return nil, err
	}
	logsByCard := groupReviewLogsByCard(logs)
	now := time.Now()

	rows := make(map[string]*types.ReviewHeatmapRow)
	columns := make(map[string]*types.ReviewHeatmapColumn)
	selectedRows := make(map[string]bool, len(opts.RowGroupIDs))
	for _, id := range opts.RowGroupIDs {
		if allowed[id] && groupsByID[id] != nil {
			selectedRows[id] = true
		}
	}
	for _, card := range cards {
		if cardByID[card.ID] == nil {
			continue
		}
		rowID, direct := reviewRowForCard(card.GroupID, scopeID, groupsByID, hasChildren)
		if len(selectedRows) > 0 {
			rowID = reviewSelectedRowForCard(card.GroupID, selectedRows, groupsByID)
			direct = false
		}
		if rowID == "" {
			continue
		}
		if rows[rowID] == nil {
			row := &types.ReviewHeatmapRow{
				ID:      rowID,
				GroupID: rowID,
				Name:    groupsByID[card.GroupID].Name,
				Path:    reviewGroupPath(groupsByID, card.GroupID),
				Kind:    types.ReviewHeatmapRowKindGroup,
			}
			if direct {
				row.GroupID = scopeID
				row.Name = "本组卡片"
				row.Path = reviewGroupPath(groupsByID, scopeID) + " / 本组卡片"
				row.Kind = types.ReviewHeatmapRowKindDirect
				row.Virtual = true
				row.Drillable = false
			} else if group := groupsByID[rowID]; group != nil {
				row.GroupID = rowID
				row.Name = group.Name
				row.Path = reviewGroupPath(groupsByID, rowID)
				row.Drillable = hasChildren[rowID]
			}
			rows[rowID] = row
		}
		rows[rowID].TotalCards++
		level, reason := reviewCardLevelDetail(card, logsByCard[card.ID], now)
		card.Vulnerability = level
		card.VulnerabilityReason = reason
		item := &reviewHeatmapCard{
			card:       card,
			level:      level,
			reason:     reason,
			lastRating: reviewLastRating(logsByCard[card.ID]),
			rowID:      rowID,
		}
		if source := reviewPrimarySource(sourcesByCard[card.ID]); source != nil {
			item.source = source
			item.knowledgeBaseID = bases[source.KnowledgeID]
		}
		item.columnKeys = reviewColumnsForCard(item, columns, kbNames)
		build.rowCards[rowID] = append(build.rowCards[rowID], item)
	}
	if len(rows) == 0 {
		return build, nil
	}

	allRowIDs := make([]string, 0, len(rows))
	for id := range rows {
		allRowIDs = append(allRowIDs, id)
	}
	allColumnKeys := make([]string, 0, len(columns))
	for key := range columns {
		allColumnKeys = append(allColumnKeys, key)
	}
	allCells := make(map[string]*types.ReviewHeatmapCell, len(allRowIDs)*len(allColumnKeys))
	for _, rowID := range allRowIDs {
		for _, columnKey := range allColumnKeys {
			cell := reviewCellFromCardsForColumn(build.rowCards[rowID], columnKey)
			if cell != nil {
				cell.RowID = rowID
				cell.ColumnKey = columnKey
				cell.Clickable = reviewCellClickable(cell.Level)
				allCells[reviewClusterKey(rowID, columnKey)] = cell
			}
		}
	}

	rowList := make([]*types.ReviewHeatmapRow, 0, len(rows))
	for _, row := range rows {
		rowList = append(rowList, row)
	}
	sortReviewRows(rowList, allCells)
	columnList := make([]*types.ReviewHeatmapColumn, 0, len(columns))
	for _, column := range columns {
		columnList = append(columnList, column)
	}
	sortReviewColumns(columnList, allCells)

	build.data.RowTotal = len(rowList)
	build.data.ColumnTotal = len(columnList)
	build.data.Rows = rowList
	build.data.Columns = columnList

	visibleRows := rowList
	hiddenRows := []*types.ReviewHeatmapRow{}
	visibleColumns := columnList
	hiddenColumns := []*types.ReviewHeatmapColumn{}
	rowLimit := reviewHeatmapRowLimit(opts)
	columnLimit := reviewHeatmapColumnLimit(opts)
	if rowLimit > 0 && len(rowList) > rowLimit {
		visibleRows = rowList[:rowLimit]
		hiddenRows = rowList[rowLimit:]
	}
	if columnLimit > 0 && len(columnList) > columnLimit {
		visibleColumns = columnList[:columnLimit]
		hiddenColumns = columnList[columnLimit:]
	}
	otherCards := make([]*reviewHeatmapCard, 0)
	for _, row := range hiddenRows {
		otherCards = append(otherCards, build.rowCards[row.ID]...)
	}
	finalRows := append([]*types.ReviewHeatmapRow{}, visibleRows...)
	if len(hiddenRows) > 0 {
		otherRow := &types.ReviewHeatmapRow{
			ID:         types.ReviewHeatmapOtherRowID,
			Name:       "其他卡组",
			Path:       "其他卡组",
			Kind:       types.ReviewHeatmapRowKindOther,
			Drillable:  false,
			TotalCards: len(otherCards),
		}
		finalRows = append(finalRows, otherRow)
	}
	finalColumns := append([]*types.ReviewHeatmapColumn{}, visibleColumns...)
	if len(hiddenColumns) > 0 {
		otherTitle := "其他主题"
		finalColumns = append(finalColumns, &types.ReviewHeatmapColumn{
			Key:   types.ReviewHeatmapOtherColumnKey,
			Title: otherTitle,
			Kind:  "other",
		})
	}
	build.data.RowTruncated = len(hiddenRows) > 0
	build.data.ColumnTruncated = len(hiddenColumns) > 0
	build.data.Rows = finalRows
	build.data.Columns = finalColumns

	for _, row := range visibleRows {
		for _, column := range visibleColumns {
			cell := reviewCellFromCardsForColumn(build.rowCards[row.ID], column.Key)
			if cell == nil {
				continue
			}
			cell.RowID = row.ID
			cell.ColumnKey = column.Key
			cell.Clickable = reviewCellClickable(cell.Level)
			build.data.Cells = append(build.data.Cells, cell)
		}
	}
	if len(hiddenRows) > 0 {
		for _, column := range visibleColumns {
			cell := reviewCellFromCardsForColumn(otherCards, column.Key)
			if cell == nil {
				continue
			}
			cell.RowID = types.ReviewHeatmapOtherRowID
			cell.ColumnKey = column.Key
			cell.Clickable = false
			build.data.Cells = append(build.data.Cells, cell)
		}
	}
	if len(hiddenColumns) > 0 {
		hiddenColumnKeys := make(map[string]bool, len(hiddenColumns))
		for _, column := range hiddenColumns {
			hiddenColumnKeys[column.Key] = true
		}
		for _, row := range visibleRows {
			cell := reviewCellFromHiddenColumns(build.rowCards[row.ID], hiddenColumnKeys)
			if cell == nil {
				continue
			}
			cell.RowID = row.ID
			cell.ColumnKey = types.ReviewHeatmapOtherColumnKey
			cell.Clickable = false
			build.data.Cells = append(build.data.Cells, cell)
		}
		cell := reviewCellFromHiddenColumns(otherCards, hiddenColumnKeys)
		if cell != nil {
			cell.RowID = types.ReviewHeatmapOtherRowID
			cell.ColumnKey = types.ReviewHeatmapOtherColumnKey
			cell.Clickable = false
			build.data.Cells = append(build.data.Cells, cell)
		}
	}
	sort.Slice(build.data.Cells, func(i, j int) bool {
		if build.data.Cells[i].RowID != build.data.Cells[j].RowID {
			return build.data.Cells[i].RowID < build.data.Cells[j].RowID
		}
		return build.data.Cells[i].ColumnKey < build.data.Cells[j].ColumnKey
	})
	rowPath := make(map[string]string, len(finalRows))
	for _, row := range finalRows {
		rowPath[row.ID] = row.Path
	}
	columnTitle := make(map[string]string, len(finalColumns))
	for _, column := range finalColumns {
		columnTitle[column.Key] = column.Title
	}
	build.data.ObserveCount = 0
	for _, cell := range build.data.Cells {
		if cell.RowID == types.ReviewHeatmapOtherRowID || cell.ColumnKey == types.ReviewHeatmapOtherColumnKey {
			continue
		}
		if cell.Level == types.ReviewCardLevelObserve {
			build.data.ObserveCount++
		}
		if cell.Level != types.ReviewCardLevelSevere && cell.Level != types.ReviewCardLevelWarning {
			continue
		}
		build.data.Priorities = append(build.data.Priorities, cell)
	}
	sortReviewPriorities(build.data.Priorities, rowPath, columnTitle)
	if len(build.data.Priorities) > 3 {
		build.data.Priorities = build.data.Priorities[:3]
	}
	build.data.Insight = reviewHeatmapInsight(build.data.Cells, rowPath, columnTitle)
	return build, nil
}

func reviewHeatmapRowLimit(opts *types.ReviewHeatmapOptions) int {
	if opts != nil && opts.RowLimit > 0 {
		if opts.RowLimit > reviewHeatmapExpandedRowLimit {
			return reviewHeatmapExpandedRowLimit
		}
		return opts.RowLimit
	}
	if opts != nil && opts.Expand {
		return reviewHeatmapExpandedRowLimit
	}
	return reviewHeatmapDefaultRowLimit
}

func reviewHeatmapColumnLimit(opts *types.ReviewHeatmapOptions) int {
	if opts != nil && opts.ColumnLimit > 0 {
		if opts.ColumnLimit > reviewHeatmapExpandedColLimit {
			return reviewHeatmapExpandedColLimit
		}
		return opts.ColumnLimit
	}
	if opts != nil && opts.Expand {
		return reviewHeatmapExpandedColLimit
	}
	return reviewHeatmapDefaultColLimit
}

func reviewRowForCard(
	groupID, scopeID string,
	groupsByID map[string]*types.ReviewGroup,
	hasChildren map[string]bool,
) (string, bool) {
	if groupID == "" || groupsByID[groupID] == nil {
		return "", false
	}
	if scopeID == "" {
		chain := make([]string, 0, 8)
		cursor := groupID
		for groupsByID[cursor] != nil {
			chain = append(chain, cursor)
			parent := groupsByID[cursor].ParentID
			if parent == "" || groupsByID[parent] == nil {
				break
			}
			cursor = parent
		}
		if len(chain) >= 2 {
			// Default rows are second-level directories (the child of a root
			// group). Deeper cards roll up to that directory.
			return chain[len(chain)-2], false
		}
		return chain[len(chain)-1], false
	}
	if groupID == scopeID {
		if hasChildren[scopeID] {
			return scopeID + ":direct", true
		}
		return scopeID, false
	}
	cursor := groupID
	for groupsByID[cursor] != nil && cursor != scopeID {
		group := groupsByID[cursor]
		if group.ParentID == scopeID {
			return cursor, false
		}
		if group.ParentID == "" {
			return "", false
		}
		cursor = group.ParentID
	}
	return "", false
}

// reviewSelectedRowForCard maps a card to the deepest user-selected row group
// that owns it. Overlapping selections (parent plus child) resolve to the
// child so a card is never counted twice.
func reviewSelectedRowForCard(
	groupID string,
	selected map[string]bool,
	groupsByID map[string]*types.ReviewGroup,
) string {
	best := ""
	bestDepth := -1
	cursor := groupID
	for groupsByID[cursor] != nil {
		if selected[cursor] {
			depth := 0
			for probe := cursor; groupsByID[probe] != nil; {
				depth++
				parent := groupsByID[probe].ParentID
				if parent == "" || groupsByID[parent] == nil {
					break
				}
				probe = parent
			}
			if depth > bestDepth {
				best = cursor
				bestDepth = depth
			}
		}
		parent := groupsByID[cursor].ParentID
		if parent == "" || groupsByID[parent] == nil {
			break
		}
		cursor = parent
	}
	return best
}

func reviewSubtreeGroupIDs(rootID string, children map[string][]string) []string {
	out := make([]string, 0)
	queue := []string{rootID}
	seen := make(map[string]bool)
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
		queue = append(queue, children[id]...)
	}
	return out
}

func reviewSourcesByCard(
	sources []*types.ReviewCardSource, cardByID map[string]*types.ReviewCard,
) map[string][]*types.ReviewCardSource {
	grouped := make(map[string][]*types.ReviewCardSource)
	for _, source := range sources {
		if cardByID[source.CardID] == nil {
			continue
		}
		if source.SourceType != types.ReviewSourceTypeKnowledge ||
			source.Status != types.ReviewSourceStatusActive {
			continue
		}
		grouped[source.CardID] = append(grouped[source.CardID], source)
	}
	for cardID := range grouped {
		sort.SliceStable(grouped[cardID], func(i, j int) bool {
			return grouped[cardID][i].IsPrimary && !grouped[cardID][j].IsPrimary
		})
	}
	return grouped
}

func reviewPrimarySource(sources []*types.ReviewCardSource) *types.ReviewCardSource {
	if len(sources) == 0 {
		return nil
	}
	return sources[0]
}

func reviewColumnsForCard(
	item *reviewHeatmapCard,
	columns map[string]*types.ReviewHeatmapColumn,
	kbNames map[string]string,
) []string {
	tags := reviewTagsFromJSON(item.card.Tags)
	if len(tags) > 0 {
		seen := make(map[string]bool, len(tags))
		keys := make([]string, 0, len(tags))
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
			keys = append(keys, key)
		}
		return keys
	}
	if item.source != nil && item.source.KnowledgeID != "" && item.knowledgeBaseID != "" {
		key := types.ReviewHeatmapColumnKindKnowledgeBase + ":" + item.knowledgeBaseID
		if columns[key] == nil {
			title := kbNames[item.knowledgeBaseID]
			if title == "" {
				title = "未知知识库"
			}
			columns[key] = &types.ReviewHeatmapColumn{
				Key:             key,
				Title:           title,
				Kind:            types.ReviewHeatmapColumnKindKnowledgeBase,
				KnowledgeBaseID: item.knowledgeBaseID,
			}
		}
		return []string{key}
	}
	key := reviewHeatmapUncategorizedKey
	if columns[key] == nil {
		columns[key] = &types.ReviewHeatmapColumn{
			Key:   key,
			Title: "未分类",
			Kind:  types.ReviewHeatmapColumnKindUncategorized,
		}
	}
	return []string{key}
}

func reviewCardMatchesColumn(item *reviewHeatmapCard, columnKey string) bool {
	switch {
	case strings.HasPrefix(columnKey, types.ReviewHeatmapColumnKindTag+":"):
		tag := strings.TrimSpace(strings.TrimPrefix(columnKey, types.ReviewHeatmapColumnKindTag+":"))
		for _, value := range reviewTagsFromJSON(item.card.Tags) {
			if strings.EqualFold(strings.TrimSpace(value), tag) {
				return true
			}
		}
		return false
	case strings.HasPrefix(columnKey, types.ReviewHeatmapColumnKindKnowledgeBase+":"):
		if len(reviewTagsFromJSON(item.card.Tags)) > 0 {
			return false
		}
		kbID := strings.TrimSpace(strings.TrimPrefix(columnKey, types.ReviewHeatmapColumnKindKnowledgeBase+":"))
		return item.knowledgeBaseID == kbID
	case columnKey == reviewHeatmapUncategorizedKey:
		return len(reviewTagsFromJSON(item.card.Tags)) == 0 && item.knowledgeBaseID == ""
	case strings.HasPrefix(columnKey, types.ReviewHeatmapColumnKindSource+":"):
		sourceID := strings.TrimSpace(strings.TrimPrefix(columnKey, types.ReviewHeatmapColumnKindSource+":"))
		return item.source != nil && item.source.KnowledgeID == sourceID
	default:
		return false
	}
}

func reviewColumnForDetail(
	columnKey string, cards []*types.ReviewCard,
	sourcesByCard map[string][]*types.ReviewCardSource,
	titles, bases, kbNames map[string]string,
) *types.ReviewHeatmapColumn {
	switch {
	case strings.HasPrefix(columnKey, types.ReviewHeatmapColumnKindTag+":"):
		tag := strings.TrimPrefix(columnKey, types.ReviewHeatmapColumnKindTag+":")
		title := tag
		for _, card := range cards {
			for _, value := range reviewTagsFromJSON(card.Tags) {
				if strings.EqualFold(strings.TrimSpace(value), tag) {
					title = strings.TrimSpace(value)
					break
				}
			}
		}
		return &types.ReviewHeatmapColumn{Key: columnKey, Title: title, Kind: types.ReviewHeatmapColumnKindTag}
	case strings.HasPrefix(columnKey, types.ReviewHeatmapColumnKindKnowledgeBase+":"):
		kbID := strings.TrimPrefix(columnKey, types.ReviewHeatmapColumnKindKnowledgeBase+":")
		title := kbNames[kbID]
		if title == "" {
			title = "未知知识库"
		}
		return &types.ReviewHeatmapColumn{
			Key:             columnKey,
			Title:           title,
			Kind:            types.ReviewHeatmapColumnKindKnowledgeBase,
			KnowledgeBaseID: kbID,
		}
	case strings.HasPrefix(columnKey, types.ReviewHeatmapColumnKindSource+":"):
		sourceID := strings.TrimPrefix(columnKey, types.ReviewHeatmapColumnKindSource+":")
		title := titles[sourceID]
		if title == "" {
			title = "来源文档"
		}
		return &types.ReviewHeatmapColumn{Key: columnKey, Title: title, Kind: types.ReviewHeatmapColumnKindSource, SourceID: sourceID}
	default:
		return &types.ReviewHeatmapColumn{Key: reviewHeatmapUncategorizedKey, Title: "未分类", Kind: types.ReviewHeatmapColumnKindUncategorized}
	}
}

func reviewCellFromCardsForColumn(cards []*reviewHeatmapCard, columnKey string) *types.ReviewHeatmapCell {
	matching := make([]*reviewHeatmapCard, 0, len(cards))
	for _, card := range cards {
		for _, key := range card.columnKeys {
			if key == columnKey {
				matching = append(matching, card)
				break
			}
		}
	}
	return reviewCellFromCards(matching, "", columnKey, false)
}

func reviewCellFromHiddenColumns(
	cards []*reviewHeatmapCard, hiddenColumnKeys map[string]bool,
) *types.ReviewHeatmapCell {
	matching := make([]*reviewHeatmapCard, 0, len(cards))
	for _, card := range cards {
		for _, key := range card.columnKeys {
			if hiddenColumnKeys[key] {
				matching = append(matching, card)
				break
			}
		}
	}
	return reviewCellFromCards(matching, "", types.ReviewHeatmapOtherColumnKey, false)
}

func reviewCellFromCards(
	cards []*reviewHeatmapCard, rowID, columnKey string, clickable bool,
) *types.ReviewHeatmapCell {
	if len(cards) == 0 {
		return nil
	}
	cell := &types.ReviewHeatmapCell{RowID: rowID, ColumnKey: columnKey}
	weights := 0.0
	highest := ""
	for _, card := range cards {
		cell.Total++
		switch card.level {
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
		weights += reviewLevelWeight(card.level)
		highest = maxReviewLevel(highest, card.level)
	}
	cell.Intensity = weights / float64(cell.Total)
	reviewed := cell.Total - cell.NoData
	cell.Level = maxReviewLevel(reviewLevelFromIntensity(cell.Intensity, reviewed), highest)
	cell.Clickable = clickable && reviewCellClickable(cell.Level)
	return cell
}

func reviewCellClickable(level string) bool {
	return level == types.ReviewCardLevelSevere ||
		level == types.ReviewCardLevelWarning ||
		level == types.ReviewCardLevelObserve
}

func sortReviewRows(rows []*types.ReviewHeatmapRow, cells map[string]*types.ReviewHeatmapCell) {
	type aggregate struct {
		rank      int
		intensity float64
		severe    int
	}
	agg := make(map[string]aggregate, len(rows))
	for key, cell := range cells {
		rowID := cell.RowID
		current := agg[rowID]
		if reviewLevelRank(cell.Level) > current.rank {
			current.rank = reviewLevelRank(cell.Level)
		}
		current.intensity += cell.Intensity
		current.severe += cell.Severe
		agg[rowID] = current
		_ = key
	}
	sort.SliceStable(rows, func(i, j int) bool {
		left, right := agg[rows[i].ID], agg[rows[j].ID]
		if left.severe != right.severe {
			return left.severe > right.severe
		}
		if left.rank != right.rank {
			return left.rank > right.rank
		}
		if left.intensity != right.intensity {
			return left.intensity > right.intensity
		}
		if rows[i].Path != rows[j].Path {
			return rows[i].Path < rows[j].Path
		}
		return rows[i].ID < rows[j].ID
	})
}

func sortReviewColumns(columns []*types.ReviewHeatmapColumn, cells map[string]*types.ReviewHeatmapCell) {
	type aggregate struct {
		rank      int
		intensity float64
	}
	agg := make(map[string]aggregate, len(columns))
	for _, cell := range cells {
		current := agg[cell.ColumnKey]
		if reviewLevelRank(cell.Level) > current.rank {
			current.rank = reviewLevelRank(cell.Level)
		}
		current.intensity += cell.Intensity
		agg[cell.ColumnKey] = current
	}
	sort.SliceStable(columns, func(i, j int) bool {
		left, right := agg[columns[i].Key], agg[columns[j].Key]
		if left.rank != right.rank {
			return left.rank > right.rank
		}
		if left.intensity != right.intensity {
			return left.intensity > right.intensity
		}
		if columns[i].Title != columns[j].Title {
			return columns[i].Title < columns[j].Title
		}
		return columns[i].Key < columns[j].Key
	})
}

func sortReviewPriorities(
	priorities []*types.ReviewHeatmapCell, rowPath, columnTitle map[string]string,
) {
	sort.SliceStable(priorities, func(i, j int) bool {
		left, right := priorities[i], priorities[j]
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
}

func reviewHeatmapFormula(denominator int) *types.ReviewHeatmapFormula {
	return &types.ReviewHeatmapFormula{
		SevereWeight:  reviewLevelWeight(types.ReviewCardLevelSevere),
		WarningWeight: reviewLevelWeight(types.ReviewCardLevelWarning),
		ObserveWeight: reviewLevelWeight(types.ReviewCardLevelObserve),
		Denominator:   denominator,
		Text:          "漏洞强度 =（严重×1 + 警告×0.5 + 观察×0.25）÷ 卡数",
	}
}

func (s *reviewService) reviewKnowledgeBaseNames(
	ctx context.Context, tenantID uint64, ids map[string]bool,
) map[string]string {
	names := make(map[string]string, len(ids))
	if s.knowledgeResolver == nil || len(ids) == 0 {
		return names
	}
	list := make([]string, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	items, err := s.knowledgeResolver.GetKnowledgeBaseByIDs(ctx, list)
	if err != nil {
		return names
	}
	for _, item := range items {
		if item == nil || item.Name == "" {
			continue
		}
		if item.TenantID != 0 && item.TenantID != tenantID {
			continue
		}
		names[item.ID] = item.Name
	}
	return names
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

func reviewHeatmapInsight(
	cells []*types.ReviewHeatmapCell, rowPath, columnTitle map[string]string,
) *types.ReviewHeatmapInsight {
	rowSevere := make(map[string]int)
	columnSevere := make(map[string]int)
	for _, cell := range cells {
		if cell.Level != types.ReviewCardLevelSevere {
			continue
		}
		if cell.RowID == types.ReviewHeatmapOtherRowID || cell.ColumnKey == types.ReviewHeatmapOtherColumnKey {
			continue
		}
		rowSevere[cell.RowID]++
		columnSevere[cell.ColumnKey]++
	}
	bestRowID, bestRowSevere := "", 0
	for rowID, count := range rowSevere {
		if count < 2 {
			continue
		}
		if count > bestRowSevere || (count == bestRowSevere && rowID < bestRowID) || bestRowID == "" {
			bestRowID, bestRowSevere = rowID, count
		}
	}
	bestColumnKey, bestColumnSevere := "", 0
	for columnKey, count := range columnSevere {
		if count < 2 {
			continue
		}
		if count > bestColumnSevere || (count == bestColumnSevere && columnKey < bestColumnKey) || bestColumnKey == "" {
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
