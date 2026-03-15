package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/alienxp03/rom-agent/internal/client"
	"github.com/alienxp03/rom-agent/internal/db"
	pb "github.com/alienxp03/rom-agent/internal/proto/pb"
	"github.com/alienxp03/rom-agent/internal/resources"
)

func (b *Bot) doExchange(ctx context.Context) error {
	if b.scanTargetDb == nil {
		return fmt.Errorf("targeted exchange requires scan target store")
	}
	if b.exchangeMarket == "" {
		return fmt.Errorf("targeted exchange requires exchange market")
	}
	startTime := time.Now()
	targets, err := b.scanTargetDb.ListActiveByMarket(b.exchangeMarket, b.clientConfig.ExchangeMarketAliases)
	if err != nil {
		return fmt.Errorf("load scan targets for market %s: %w", b.exchangeMarket, err)
	}

	slog.Info("Exchange scraping started",
		"runtime_server", b.runtimeServer,
		"exchange_market", b.exchangeMarket,
		"target_count", len(targets),
		"start_time", startTime.Format("2006-01-02 @ 3:04:05 PM"))

	totalRecordCount := 0
	for _, target := range targets {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		records, err := b.gameClient.QueryExchangeItem(ctx, int(target.ThingID), resources.ExchangeCategory{})
		if err != nil {
			slog.Error("Failed to query target item",
				"runtime_server", b.runtimeServer,
				"exchange_market", b.exchangeMarket,
				"thing_id", target.ThingID,
				"error", err)
			continue
		}

		snappingRecords := exchangeSnappingRecords(records)
		filtered := filterExchangeRecordsForTarget(snappingRecords, target)
		slog.Info("Target exchange filtering completed",
			"exchange_market", b.exchangeMarket,
			"thing_id", target.ThingID,
			"target_equip_type", target.EquipType,
			"target_broken_state", target.BrokenState,
			"target_refine_min", intValue(target.RefineMin),
			"target_refine_max", intValue(target.RefineMax),
			"target_enchant_ids", target.EnchantIDs,
			"raw_record_count", len(records),
			"snapping_record_count", len(snappingRecords),
			"filtered_record_count", len(filtered),
			"target_snap_count", target.SnapCount)
		for i, record := range snappingRecords {
			slog.Debug("Snapping record candidate",
				"exchange_market", b.exchangeMarket,
				"thing_id", target.ThingID,
				"candidate_index", i,
				"price", record.BaseInfo.GetPrice(),
				"count", record.BaseInfo.GetCount(),
				"end_time", record.BaseInfo.GetEndTime(),
				"refine_level", exchangeRefineLevel(record),
				"is_broken", exchangeBrokenState(record),
				"buff_id", intValue(exchangeBuffID(record)))
		}
		seenAt := time.Now().UTC()
		persisted := make([]*db.ExchangeItemRecord, 0, len(filtered))
		for _, record := range filtered {
			persisted = append(persisted, buildExchangeDBRecord(record, resources.ExchangeCategory{}, b.exchangeMarket, "", seenAt, b.thingSnapshotStore, b.exchangeDb))
			slog.Info(formatExchangeRecord(record, int(target.ThingID), b.thingSnapshotStore, b.exchangeDb))
		}
		totalRecordCount += len(filtered)

		if b.exchangeDb == nil {
			slog.Warn("Skipping targeted exchange persistence because exchange store is not configured",
				"exchange_market", b.exchangeMarket,
				"thing_id", target.ThingID)
			continue
		}
		if err := b.exchangeDb.ReplaceTargetedRecords(int(target.ThingID), b.exchangeMarket, persisted); err != nil {
			slog.Error("Failed to persist targeted exchange item records",
				"exchange_market", b.exchangeMarket,
				"thing_id", target.ThingID,
				"error", err)
			continue
		}

		slog.Info("Target scan completed",
			"exchange_market", b.exchangeMarket,
			"thing_id", target.ThingID,
			"matched_records", len(filtered),
			"target_snap_count", target.SnapCount)
	}

	totalDuration := time.Since(startTime)
	b.lastExchangeRecordCount = totalRecordCount
	slog.Info("Exchange scraping completed",
		"exchange_market", b.exchangeMarket,
		"record_count", totalRecordCount,
		"total_mins", int(totalDuration.Minutes()),
		"total_secs", int(totalDuration.Seconds())%60)
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
			continue
		}
		trimmed = append(trimmed, record)
	}
	return trimmed
}

func exchangeItemName(record *client.ExchangeItemRecord, thingSnapshotStore *db.ExchangeThingSnapshotStore, exchangeDb *db.ExchangeDb) string {
	if record == nil || record.BaseInfo == nil {
		return ""
	}
	itemID := int(record.BaseInfo.GetItemid())

	if thingSnapshotStore != nil {
		if name := thingSnapshotStore.GetName(itemID); name != "" {
			return name
		}
	}

	// Fall back to names captured from previous exchange rows.
	if exchangeDb != nil {
		if name := exchangeDb.GetItemName(itemID); name != "" {
			return name
		}
	}

	// Fallback to item:<id> format rather than using seller name
	return fmt.Sprintf("item:%d", itemID)
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

func formatExchangeRecord(record *client.ExchangeItemRecord, requestedItemID int, thingSnapshotStore *db.ExchangeThingSnapshotStore, exchangeDb *db.ExchangeDb) string {
	if record == nil || record.BaseInfo == nil {
		return fmt.Sprintf("%d: <missing record>", requestedItemID)
	}

	baseInfo := record.BaseInfo
	name := exchangeItemName(record, thingSnapshotStore, exchangeDb)
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
	thingSnapshotStore *db.ExchangeThingSnapshotStore,
	exchangeDb *db.ExchangeDb,
) *db.ExchangeItemRecord {
	if record == nil || record.BaseInfo == nil {
		return nil
	}

	baseInfo := record.BaseInfo
	name := exchangeItemName(record, thingSnapshotStore, exchangeDb)
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
