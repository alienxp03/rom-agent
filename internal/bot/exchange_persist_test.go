package bot

import (
	"testing"
	"time"

	"github.com/alienxp03/rom-agent/internal/client"
	"github.com/alienxp03/rom-agent/internal/db"
	pb "github.com/alienxp03/rom-agent/internal/proto/pb"
	"github.com/alienxp03/rom-agent/internal/resources"
)

func TestBuildExchangeDBRecordExtractsVariantFields(t *testing.T) {
	itemID := uint32(1001)
	price := uint64(320000)
	count := uint32(2)
	endTime := uint32(123456)
	refine := uint32(9)
	damage := true
	buyerCount := uint32(3)
	quota := uint64(7)
	name := "Angeling Card"
	buffID := uint32(88)
	attrType1 := pb.EAttrType_EATTRTYPE_STR
	attrValue1 := uint32(5)
	attrType2 := pb.EAttrType_EATTRTYPE_DEX
	attrValue2 := uint32(8)

	record := &client.ExchangeItemRecord{
		BaseInfo: &pb.TradeItemBaseInfo{
			Itemid:  &itemID,
			Price:   &price,
			Count:   &count,
			EndTime: &endTime,
			Name:    &name,
		},
		ItemData: &pb.ItemData{
			Equip: &pb.EquipData{
				Refinelv: &refine,
				Damage:   &damage,
			},
			Enchant: &pb.EnchantData{
				Attrs: []*pb.EnchantAttr{
					{Type: &attrType1, Value: &attrValue1},
					{Type: &attrType2, Value: &attrValue2},
				},
				Extras: []*pb.EnchantExtra{
					{Buffid: &buffID},
				},
			},
		},
		SellInfo: &pb.ItemSellInfoRecordTradeCmd{
			BuyerCount: &buyerCount,
			Quota:      &quota,
		},
	}

	seenAt := time.Unix(1000, 0).UTC()
	got := buildExchangeDBRecord(record, resources.Material, "SEA2", "4201", seenAt)

	if got == nil {
		t.Fatal("buildExchangeDBRecord() returned nil")
	}
	if got.ItemID != int(itemID) || got.Price != int64(price) || got.ListingCount != int(count) {
		t.Fatalf("unexpected basic fields: %#v", got)
	}
	if got.RefineLevel != int(refine) {
		t.Fatalf("RefineLevel = %d, want %d", got.RefineLevel, refine)
	}
	if !got.IsBroken {
		t.Fatal("IsBroken = false, want true")
	}
	if got.BuffID == nil || *got.BuffID != int(buffID) {
		t.Fatalf("BuffID = %#v, want %d", got.BuffID, buffID)
	}
	if got.BuffAttr1Name == nil || *got.BuffAttr1Name != "STR" {
		t.Fatalf("BuffAttr1Name = %#v, want STR", got.BuffAttr1Name)
	}
	if got.BuffAttr1Value == nil || *got.BuffAttr1Value != int(attrValue1) {
		t.Fatalf("BuffAttr1Value = %#v, want %d", got.BuffAttr1Value, attrValue1)
	}
	if got.BuffAttr2Name == nil || *got.BuffAttr2Name != "DEX" {
		t.Fatalf("BuffAttr2Name = %#v, want DEX", got.BuffAttr2Name)
	}
	if got.BuyerCount != int(buyerCount) || got.Quota != int64(quota) {
		t.Fatalf("unexpected sell info fields: %#v", got)
	}
	if got.EndTime == nil || *got.EndTime != int64(endTime) {
		t.Fatalf("EndTime = %#v, want %d", got.EndTime, endTime)
	}
	if !got.Modified {
		t.Fatal("Modified = false, want true")
	}
}

func TestExchangeIdentityKeyChangesWhenVariantChanges(t *testing.T) {
	buffID1 := 1
	buffID2 := 2
	name := "Angeling Card"
	key1 := exchangeIdentityKey(1001, name, 1004, "SEA2", "4201", 5, false, &buffID1, nil, nil, nil, nil, nil, nil)
	key2 := exchangeIdentityKey(1001, name, 1004, "SEA2", "4201", 5, false, &buffID2, nil, nil, nil, nil, nil, nil)
	if key1 == key2 {
		t.Fatal("exchangeIdentityKey() should differ when variant buff id changes")
	}
}

func TestExchangeCategorySnappingItemIDs(t *testing.T) {
	categoryData := &pb.BriefPendingListRecordTradeCmd{
		PubLists: []uint32{101, 102, 103},
		Lists:    []uint32{201, 202},
	}

	got := exchangeCategorySnappingItemIDs(categoryData)
	want := []int{101, 102, 103}

	if len(got) != len(want) {
		t.Fatalf("len(exchangeCategorySnappingItemIDs()) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("exchangeCategorySnappingItemIDs()[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestExchangeSnappingRecords(t *testing.T) {
	endTime1 := uint32(100)
	endTime2 := uint32(200)
	regular := uint32(0)

	records := []*client.ExchangeItemRecord{
		{BaseInfo: &pb.TradeItemBaseInfo{EndTime: &endTime1}},
		{BaseInfo: &pb.TradeItemBaseInfo{EndTime: &endTime2}},
		{BaseInfo: &pb.TradeItemBaseInfo{EndTime: &regular}},
		{BaseInfo: &pb.TradeItemBaseInfo{EndTime: &endTime2}},
	}

	got := exchangeSnappingRecords(records)
	if len(got) != 2 {
		t.Fatalf("len(exchangeSnappingRecords()) = %d, want 2", len(got))
	}
	for i, want := range []uint32{endTime1, endTime2} {
		if got[i].BaseInfo.GetEndTime() != want {
			t.Fatalf("exchangeSnappingRecords()[%d] end_time = %d, want %d", i, got[i].BaseInfo.GetEndTime(), want)
		}
	}
}

func TestFilterExchangeRecordsForTarget(t *testing.T) {
	itemID := uint32(1001)
	price := uint64(320000)
	count := uint32(2)
	endTime := uint32(123456)
	refine := uint32(9)
	damage := false
	buffID := uint32(88)

	matching := &client.ExchangeItemRecord{
		BaseInfo: &pb.TradeItemBaseInfo{
			Itemid:  &itemID,
			Price:   &price,
			Count:   &count,
			EndTime: &endTime,
		},
		ItemData: &pb.ItemData{
			Equip: &pb.EquipData{
				Refinelv: &refine,
				Damage:   &damage,
			},
			Enchant: &pb.EnchantData{
				Extras: []*pb.EnchantExtra{
					{Buffid: &buffID},
				},
			},
		},
	}

	refineLow := uint32(4)
	nonMatching := &client.ExchangeItemRecord{
		BaseInfo: &pb.TradeItemBaseInfo{
			Itemid: &itemID,
			Price:  &price,
			Count:  &count,
		},
		ItemData: &pb.ItemData{
			Equip: &pb.EquipData{
				Refinelv: &refineLow,
			},
		},
	}

	refineMin := 5
	refineMax := 10
	target := &db.ScanTarget{
		ThingID:             1001,
		EquipType:           "equip",
		BrokenState:         "non_broken",
		RefineMin:           &refineMin,
		RefineMax:           &refineMax,
		EnchantIDs:          []int{88},
		ProjectionSignature: "sea_el:1001",
		SnapIDs:             []int64{11, 12},
	}

	got := filterExchangeRecordsForTarget([]*client.ExchangeItemRecord{matching, nonMatching}, target)
	if len(got) != 1 {
		t.Fatalf("len(filterExchangeRecordsForTarget()) = %d, want 1", len(got))
	}
	if got[0] != matching {
		t.Fatal("filterExchangeRecordsForTarget() did not return the matching record")
	}
}

func TestBuildScanResultRecord(t *testing.T) {
	itemID := uint32(1001)
	price := uint64(320000)
	count := uint32(2)
	endTime := uint32(123456)
	orderID := uint64(99)
	refine := uint32(7)
	buffID := uint32(88)

	record := &client.ExchangeItemRecord{
		BaseInfo: &pb.TradeItemBaseInfo{
			Itemid:  &itemID,
			Price:   &price,
			Count:   &count,
			EndTime: &endTime,
			OrderId: &orderID,
		},
		ItemData: &pb.ItemData{
			Equip: &pb.EquipData{
				Refinelv: &refine,
			},
			Enchant: &pb.EnchantData{
				Extras: []*pb.EnchantExtra{
					{Buffid: &buffID},
				},
			},
		},
	}
	target := &db.ScanTarget{
		ThingID:             1001,
		ProjectionSignature: "sea_el:1001",
		SnapIDs:             []int64{1, 2},
	}
	seenAt := time.Unix(1000, 0).UTC()

	got := buildScanResultRecord(record, target, seenAt)
	if got == nil {
		t.Fatal("buildScanResultRecord() returned nil")
	}
	if got.RecordID != "order:99" {
		t.Fatalf("RecordID = %q, want order:99", got.RecordID)
	}
	if got.ThingID != 1001 || got.Price != int64(price) || got.StockCount != int(count) {
		t.Fatalf("unexpected basic fields: %#v", got)
	}
	if got.Enchant == nil || *got.Enchant != 88 || got.RefineLevel != 7 {
		t.Fatalf("unexpected enchant/refine fields: %#v", got)
	}
	if got.SnapAt != seenAt || got.SnapEndAt == nil || got.ProjectionSignature != "sea_el:1001" {
		t.Fatalf("unexpected timing/signature fields: %#v", got)
	}
}
