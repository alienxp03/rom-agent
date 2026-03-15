package bot

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/alienxp03/rom-agent/internal/client"
	"github.com/alienxp03/rom-agent/internal/db"
	pb "github.com/alienxp03/rom-agent/internal/proto/pb"
	"github.com/alienxp03/rom-agent/internal/resources"
)

// doExchange implements the exchange scraping logic
func (b *Bot) doExchange(ctx context.Context) error {
	if b.scanTargetDb != nil && b.scanResultDb != nil && b.activeServerID != "" {
		return b.doTargetedExchange(ctx)
	}

	startTime := time.Now()
	slog.Info("Exchange scraping started",
		"start_time", startTime.Format("2006-01-02 @ 3:04:05 PM"))

	for ; b.categoryIndex < len(b.exchangeCategories); b.categoryIndex++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		categoryStartTime := time.Now()
		category := b.exchangeCategories[b.categoryIndex]
		categorySeenAt := time.Now().UTC()
		categoryTs := categorySeenAt.UnixMilli()

		slog.Info("Processing category",
			"category", category.Name,
			"index", b.categoryIndex+1,
			"total", len(b.exchangeCategories))

		// Query all items in this category
		categoryData, err := b.gameClient.QueryExchangeCategory(ctx, category)
		if err != nil {
			slog.Error("Failed to query category",
				"category", category.Name,
				"error", err)
			continue
		}

		itemList := exchangeCategorySnappingItemIDs(categoryData)
		snappingCount := len(categoryData.GetPubLists())

		slog.Info("Category items",
			"category", category.Name,
			"category_id", category.ID,
			"item_count", len(categoryData.GetPubLists())+len(categoryData.GetLists()),
			"snapping_item_count", snappingCount,
			"regular_item_count", len(categoryData.GetLists()),
			"queried_item_count", len(itemList))

		// Process each item
		for _, itemId := range itemList {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			slog.Debug("Exchange item fetch started",
				"category", category.Name,
				"category_id", category.ID,
				"item_id", itemId)

			// Query all listings for this item
			records, err := b.gameClient.QueryExchangeItem(ctx, itemId, category)
			if err != nil {
				slog.Error("Failed to query item",
					"item_id", itemId,
					"category", category.Name,
					"error", err)
				continue
			}
			records = exchangeSnappingRecords(records)

			isSnapping := false
			for _, record := range records {
				if record != nil && record.BaseInfo != nil && record.BaseInfo.GetEndTime() > 0 {
					isSnapping = true
					break
				}
			}

			if b.exchangeDb != nil {
				persistedRecords := make([]*db.ExchangeItemRecord, 0, len(records))
				for _, record := range records {
					persistedRecords = append(persistedRecords, buildExchangeDBRecord(record, category, b.server, b.zone, categorySeenAt))
				}
				if err := b.exchangeDb.UpsertLatestRecords(persistedRecords); err != nil {
					slog.Error("Failed to persist exchange item records",
						"category", category.Name,
						"category_id", category.ID,
						"item_id", itemId,
						"server", b.server,
						"zone", b.zone,
						"error", err)
				}
			}

			for _, record := range records {
				slog.Info(formatExchangeRecord(record, itemId))
			}

			slog.Debug("Exchange item fetch completed",
				"category", category.Name,
				"category_id", category.ID,
				"item_id", itemId,
				"record_count", len(records),
				"is_snapping", isSnapping)
		}

		if b.exchangeDb != nil {
			soldOutCount, err := b.exchangeDb.MarkSoldOut(category, b.server, b.zone, categorySeenAt)
			if err != nil {
				slog.Warn("Failed to mark exchange items as out of stock",
					"category", category.Name,
					"category_id", category.ID,
					"server", b.server,
					"zone", b.zone,
					"error", err)
			} else {
				slog.Info("Marked exchange items as out of stock",
					"category", category.Name,
					"category_id", category.ID,
					"server", b.server,
					"zone", b.zone,
					"count", soldOutCount)
			}
		}
		_ = categoryTs

		categoryDuration := time.Since(categoryStartTime)
		slog.Info("Category completed",
			"category", category.Name,
			"duration_mins", int(categoryDuration.Minutes()),
			"duration_secs", int(categoryDuration.Seconds())%60,
			"end_time", time.Now().Format("2006-01-02 @ 3:04:05 PM"))

		// Force garbage collection (like Java version)
		runtime.GC()

		// Save state after each category
		if err := b.SaveState(); err != nil {
			slog.Warn("Failed to save state", "error", err)
		}
	}

	totalDuration := time.Since(startTime)
	slog.Info("Exchange scraping completed",
		"end_time", time.Now().Format("2006-01-02 @ 3:04:05 PM"),
		"total_mins", int(totalDuration.Minutes()),
		"total_secs", int(totalDuration.Seconds())%60)

	// Reset category index for next cycle
	b.categoryIndex = 0
	return nil
}

func (b *Bot) doTargetedExchange(ctx context.Context) error {
	startTime := time.Now()
	targets, err := b.scanTargetDb.ListActiveByServer(b.activeServerID)
	if err != nil {
		return fmt.Errorf("load scan targets for %s: %w", b.activeServerID, err)
	}

	slog.Info("Exchange scraping started (targeted mode)",
		"active_server", b.activeServerID,
		"target_count", len(targets),
		"start_time", startTime.Format("2006-01-02 @ 3:04:05 PM"))

	for _, target := range targets {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		records, err := b.gameClient.QueryExchangeItem(ctx, int(target.ThingID), resources.ExchangeCategory{})
		if err != nil {
			slog.Error("Failed to query target item",
				"active_server", b.activeServerID,
				"thing_id", target.ThingID,
				"projection_signature", target.ProjectionSignature,
				"error", err)
			continue
		}

		filtered := filterExchangeRecordsForTarget(records, target)
		persisted := make([]*db.ScanResultRecord, 0, len(filtered))
		seenAt := time.Now().UTC()
		for _, record := range filtered {
			persisted = append(persisted, buildScanResultRecord(record, target, seenAt))
			slog.Info(formatExchangeRecord(record, int(target.ThingID)))
		}

		if err := b.scanResultDb.ReplaceForTarget(target, persisted); err != nil {
			slog.Error("Failed to persist target scan results",
				"active_server", b.activeServerID,
				"thing_id", target.ThingID,
				"projection_signature", target.ProjectionSignature,
				"error", err)
			continue
		}

		slog.Info("Target scan completed",
			"active_server", b.activeServerID,
			"thing_id", target.ThingID,
			"projection_signature", target.ProjectionSignature,
			"matched_records", len(filtered),
			"target_snap_count", target.SnapCount)
	}

	totalDuration := time.Since(startTime)
	slog.Info("Exchange scraping completed (targeted mode)",
		"active_server", b.activeServerID,
		"total_mins", int(totalDuration.Minutes()),
		"total_secs", int(totalDuration.Seconds())%60)
	return nil
}

// doExchange2 implements optimized exchange scraping for selected items only
func (b *Bot) doExchange2(ctx context.Context, targetItemIds []int) error {
	startTime := time.Now()
	slog.Info("Exchange scraping started (selective mode)",
		"start_time", startTime.Format("2006-01-02 @ 3:04:05 PM"),
		"target_items", len(targetItemIds))

	targetItemSet := make(map[int]bool, len(targetItemIds))
	for _, itemId := range targetItemIds {
		targetItemSet[itemId] = true
	}

	for ; b.categoryIndex < len(b.exchangeCategories); b.categoryIndex++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		categoryStartTime := time.Now()
		category := b.exchangeCategories[b.categoryIndex]
		categorySeenAt := time.Now().UTC()
		categoryTs := categorySeenAt.UnixMilli()

		// Query category
		categoryData, err := b.gameClient.QueryExchangeCategory(ctx, category)
		if err != nil {
			slog.Error("Failed to query category",
				"category", category.Name,
				"error", err)
			continue
		}

		itemList := exchangeCategorySnappingItemIDs(categoryData)

		// Filter items against target list
		var queryItemIds []int
		for _, itemId := range itemList {
			if targetItemSet[itemId] {
				queryItemIds = append(queryItemIds, itemId)
			}
		}

		if len(queryItemIds) == 0 {
			slog.Info("No target items in category",
				"category", category.Name)
			continue
		}

		slog.Info("Processing category (selective)",
			"category", category.Name,
			"category_id", category.ID,
			"matched_items", len(queryItemIds),
			"total_items", len(itemList))

		// Process matched items (same as doExchange)
		for _, itemId := range queryItemIds {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			records, err := b.gameClient.QueryExchangeItem(ctx, itemId, category)
			if err != nil {
				slog.Error("Failed to query item",
					"item_id", itemId,
					"error", err)
				continue
			}
			records = exchangeSnappingRecords(records)

			if b.exchangeDb != nil {
				persistedRecords := make([]*db.ExchangeItemRecord, 0, len(records))
				for _, record := range records {
					persistedRecords = append(persistedRecords, buildExchangeDBRecord(record, category, b.server, b.zone, categorySeenAt))
				}
				if err := b.exchangeDb.UpsertLatestRecords(persistedRecords); err != nil {
					slog.Error("Failed to persist selective exchange item records",
						"category", category.Name,
						"category_id", category.ID,
						"item_id", itemId,
						"server", b.server,
						"zone", b.zone,
						"error", err)
				}
			}

			for _, record := range records {
				slog.Info(formatExchangeRecord(record, itemId))
			}
		}

		// Selective mode is not a full category snapshot, so it must not mark
		// unseen variants as out of stock.
		_ = categoryTs

		categoryDuration := time.Since(categoryStartTime)
		slog.Info("Category completed",
			"category", category.Name,
			"duration_mins", int(categoryDuration.Minutes()),
			"duration_secs", int(categoryDuration.Seconds())%60)

		runtime.GC()

		if err := b.SaveState(); err != nil {
			slog.Warn("Failed to save state", "error", err)
		}
	}

	totalDuration := time.Since(startTime)
	slog.Info("Exchange scraping completed (selective)",
		"total_mins", int(totalDuration.Minutes()),
		"total_secs", int(totalDuration.Seconds())%60)

	b.categoryIndex = 0
	return nil
}

func exchangeCategoryItemIDs(categoryData *pb.BriefPendingListRecordTradeCmd) []int {
	if categoryData == nil {
		return nil
	}

	itemList := make([]int, 0, len(categoryData.GetPubLists())+len(categoryData.GetLists()))
	for _, itemID := range categoryData.GetPubLists() {
		itemList = append(itemList, int(itemID))
	}
	for _, itemID := range categoryData.GetLists() {
		itemList = append(itemList, int(itemID))
	}
	return itemList
}

func exchangeCategorySnappingItemIDs(categoryData *pb.BriefPendingListRecordTradeCmd) []int {
	if categoryData == nil {
		return nil
	}

	itemList := make([]int, 0, len(categoryData.GetPubLists()))
	for _, itemID := range categoryData.GetPubLists() {
		itemList = append(itemList, int(itemID))
	}
	return itemList
}

func exchangeSnappingRecords(records []*client.ExchangeItemRecord) []*client.ExchangeItemRecord {
	if len(records) == 0 {
		return nil
	}

	trimmed := make([]*client.ExchangeItemRecord, 0, len(records))
	for _, record := range records {
		if record == nil || record.BaseInfo == nil || record.BaseInfo.GetEndTime() == 0 {
			break
		}
		trimmed = append(trimmed, record)
	}
	return trimmed
}

func exchangeItemName(record *client.ExchangeItemRecord) string {
	if record == nil || record.BaseInfo == nil {
		return ""
	}
	if name := resources.LookupItemName(int(record.BaseInfo.GetItemid())); name != "" {
		return name
	}
	if name := record.BaseInfo.GetName(); name != "" {
		return name
	}
	return ""
}

func exchangeStartTimeISO(record *client.ExchangeItemRecord) string {
	if record == nil || record.ItemData == nil || record.ItemData.GetBase() == nil {
		return ""
	}
	createdAt := record.ItemData.GetBase().GetCreatetime()
	if createdAt == 0 {
		return ""
	}
	return time.Unix(int64(createdAt), 0).UTC().Format(time.RFC3339)
}

func exchangeEndTimeISO(endTime uint32) string {
	if endTime == 0 {
		return ""
	}
	return time.Unix(int64(endTime), 0).UTC().Format(time.RFC3339)
}

func exchangeMinutesLeft(endTime uint32) int64 {
	if endTime == 0 {
		return 0
	}
	minutes := int64(time.Until(time.Unix(int64(endTime), 0)).Minutes())
	if minutes < 0 {
		return 0
	}
	return minutes
}

func formatNumber(value uint64) string {
	s := strconv.FormatUint(value, 10)
	if len(s) <= 3 {
		return s
	}

	out := make([]byte, 0, len(s)+(len(s)-1)/3)
	prefixLen := len(s) % 3
	if prefixLen == 0 {
		prefixLen = 3
	}
	out = append(out, s[:prefixLen]...)
	for i := prefixLen; i < len(s); i += 3 {
		out = append(out, ',')
		out = append(out, s[i:i+3]...)
	}
	return string(out)
}

func formatExchangeRecord(record *client.ExchangeItemRecord, requestedItemID int) string {
	if record == nil || record.BaseInfo == nil {
		return fmt.Sprintf("%d: <missing record>", requestedItemID)
	}

	baseInfo := record.BaseInfo
	name := exchangeItemName(record)
	if name == "" {
		name = "<unknown item>"
	}

	line := fmt.Sprintf(
		"%d: %s: %s zeny, %d listed",
		baseInfo.GetItemid(),
		name,
		formatNumber(baseInfo.GetPrice()),
		baseInfo.GetCount(),
	)

	if baseInfo.GetEndTime() == 0 {
		return line
	}

	buyerCount := uint32(0)
	if record.SellInfo != nil {
		buyerCount = record.SellInfo.GetBuyerCount()
	}

	return fmt.Sprintf(
		"%s [SNAPPING: %d buyer(s), ends at %s, %d minutes left]",
		line,
		buyerCount,
		exchangeEndTimeISO(baseInfo.GetEndTime()),
		exchangeMinutesLeft(baseInfo.GetEndTime()),
	)
}

func buildExchangeDBRecord(
	record *client.ExchangeItemRecord,
	category resources.ExchangeCategory,
	server string,
	zone string,
	seenAt time.Time,
) *db.ExchangeItemRecord {
	if record == nil || record.BaseInfo == nil {
		return nil
	}

	baseInfo := record.BaseInfo
	name := exchangeItemName(record)
	if name == "" {
		name = fmt.Sprintf("item:%d", baseInfo.GetItemid())
	}

	refineLevel := exchangeRefineLevel(record)
	isBroken := exchangeBrokenState(record)
	buffID := exchangeBuffID(record)
	attr1Name, attr1Value := exchangeBuffAttr(record, 0)
	attr2Name, attr2Value := exchangeBuffAttr(record, 1)
	attr3Name, attr3Value := exchangeBuffAttr(record, 2)

	identityKey := exchangeIdentityKey(
		int(baseInfo.GetItemid()),
		name,
		category.ID,
		server,
		zone,
		refineLevel,
		isBroken,
		buffID,
		attr1Name, attr1Value,
		attr2Name, attr2Value,
		attr3Name, attr3Value,
	)

	var endTime *int64
	if value := baseInfo.GetEndTime(); value > 0 {
		endValue := int64(value)
		endTime = &endValue
	}

	buyerCount := 0
	quota := int64(0)
	if record.SellInfo != nil {
		buyerCount = int(record.SellInfo.GetBuyerCount())
		quota = int64(record.SellInfo.GetQuota())
	}

	return &db.ExchangeItemRecord{
		IdentityKey:    identityKey,
		ItemID:         int(baseInfo.GetItemid()),
		Name:           name,
		CategoryID:     category.ID,
		Server:         server,
		Zone:           zone,
		Price:          int64(baseInfo.GetPrice()),
		ListingCount:   int(baseInfo.GetCount()),
		BuyerCount:     buyerCount,
		Quota:          quota,
		EndTime:        endTime,
		InStock:        true,
		LastSeenAt:     seenAt,
		Modified:       exchangeIsModified(record),
		RefineLevel:    refineLevel,
		IsBroken:       isBroken,
		BuffID:         buffID,
		BuffAttr1Name:  attr1Name,
		BuffAttr1Value: attr1Value,
		BuffAttr2Name:  attr2Name,
		BuffAttr2Value: attr2Value,
		BuffAttr3Name:  attr3Name,
		BuffAttr3Value: attr3Value,
	}
}

func exchangeRefineLevel(record *client.ExchangeItemRecord) int {
	if record == nil || record.ItemData == nil || record.ItemData.GetEquip() == nil {
		return 0
	}
	return int(record.ItemData.GetEquip().GetRefinelv())
}

func exchangeBrokenState(record *client.ExchangeItemRecord) bool {
	if record == nil || record.ItemData == nil || record.ItemData.GetEquip() == nil {
		return false
	}
	return record.ItemData.GetEquip().GetDamage()
}

func exchangeBuffID(record *client.ExchangeItemRecord) *int {
	if record == nil || record.ItemData == nil || record.ItemData.GetEnchant() == nil {
		return nil
	}
	enchant := record.ItemData.GetEnchant()
	if len(enchant.GetExtras()) == 0 {
		return nil
	}
	buffID := int(enchant.GetExtras()[0].GetBuffid())
	if buffID == 0 {
		return nil
	}
	return &buffID
}

func exchangeBuffAttr(record *client.ExchangeItemRecord, index int) (*string, *int) {
	if record == nil || record.ItemData == nil || record.ItemData.GetEnchant() == nil {
		return nil, nil
	}
	enchant := record.ItemData.GetEnchant()
	if len(enchant.GetExtras()) == 0 || len(enchant.GetAttrs()) <= index {
		return nil, nil
	}
	attr := enchant.GetAttrs()[index]
	name := strings.TrimPrefix(attr.GetType().String(), "EATTRTYPE_")
	if name == "" || name == "MIN" {
		return nil, nil
	}
	value := int(attr.GetValue())
	return &name, &value
}

func exchangeIsModified(record *client.ExchangeItemRecord) bool {
	if exchangeRefineLevel(record) > 0 || exchangeBrokenState(record) {
		return true
	}
	return exchangeBuffID(record) != nil
}

func exchangeIdentityKey(
	itemID int,
	name string,
	categoryID int,
	server string,
	zone string,
	refineLevel int,
	isBroken bool,
	buffID *int,
	attr1Name *string,
	attr1Value *int,
	attr2Name *string,
	attr2Value *int,
	attr3Name *string,
	attr3Value *int,
) string {
	return fmt.Sprintf(
		"%d|%s|%d|%s|%s|%d|%t|%d|%s|%d|%s|%d|%s|%d",
		itemID,
		name,
		categoryID,
		server,
		zone,
		refineLevel,
		isBroken,
		intValue(buffID),
		stringValue(attr1Name),
		intValue(attr1Value),
		stringValue(attr2Name),
		intValue(attr2Value),
		stringValue(attr3Name),
		intValue(attr3Value),
	)
}

func intValue(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func filterExchangeRecordsForTarget(records []*client.ExchangeItemRecord, target *db.ScanTarget) []*client.ExchangeItemRecord {
	filtered := make([]*client.ExchangeItemRecord, 0, len(records))
	for _, record := range records {
		if exchangeRecordMatchesTarget(record, target) {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func exchangeRecordMatchesTarget(record *client.ExchangeItemRecord, target *db.ScanTarget) bool {
	if record == nil || target == nil {
		return false
	}

	refineLevel := exchangeRefineLevel(record)
	if target.RefineMin != nil && refineLevel < *target.RefineMin {
		return false
	}
	if target.RefineMax != nil && refineLevel > *target.RefineMax {
		return false
	}

	isBroken := exchangeBrokenState(record)
	switch target.BrokenState {
	case "broken":
		if !isBroken {
			return false
		}
	case "non_broken":
		if isBroken {
			return false
		}
	}

	if len(target.EnchantIDs) > 0 {
		buffID := exchangeBuffID(record)
		if buffID == nil {
			return false
		}
		matched := false
		for _, targetEnchantID := range target.EnchantIDs {
			if *buffID == targetEnchantID {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if target.EquipType == "non_equip" && exchangeIsModified(record) {
		return false
	}

	return true
}

func buildScanResultRecord(record *client.ExchangeItemRecord, target *db.ScanTarget, seenAt time.Time) *db.ScanResultRecord {
	if record == nil || record.BaseInfo == nil || target == nil {
		return nil
	}

	baseInfo := record.BaseInfo
	buffID := exchangeBuffID(record)
	enchantLevel := 0

	var snapEndAt *time.Time
	if endTime := baseInfo.GetEndTime(); endTime > 0 {
		t := time.Unix(int64(endTime), 0).UTC()
		snapEndAt = &t
	}

	return &db.ScanResultRecord{
		RecordID:            scanResultRecordID(record),
		ThingID:             int64(baseInfo.GetItemid()),
		Price:               int64(baseInfo.GetPrice()),
		StockCount:          int(baseInfo.GetCount()),
		RefineLevel:         exchangeRefineLevel(record),
		Enchant:             buffID,
		EnchantLevel:        enchantLevel,
		Broken:              exchangeBrokenState(record),
		SnapAt:              seenAt,
		SnapEndAt:           snapEndAt,
		ProjectionSignature: target.ProjectionSignature,
		SnapIDs:             target.SnapIDs,
	}
}

func scanResultRecordID(record *client.ExchangeItemRecord) string {
	if record == nil || record.BaseInfo == nil {
		return ""
	}

	baseInfo := record.BaseInfo
	switch {
	case baseInfo.GetGuid() != "":
		return baseInfo.GetGuid()
	case baseInfo.GetKey() != "":
		return baseInfo.GetKey()
	case baseInfo.GetOrderId() > 0:
		return fmt.Sprintf("order:%d", baseInfo.GetOrderId())
	case baseInfo.GetPublicityId() > 0:
		return fmt.Sprintf("publicity:%d", baseInfo.GetPublicityId())
	default:
		return fmt.Sprintf("item:%d:end:%d:price:%d:count:%d", baseInfo.GetItemid(), baseInfo.GetEndTime(), baseInfo.GetPrice(), baseInfo.GetCount())
	}
}
