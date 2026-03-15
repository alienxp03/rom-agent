package bot

import (
	"testing"

	"github.com/alienxp03/rom-agent/internal/client"
	pb "github.com/alienxp03/rom-agent/internal/proto/pb"
)

func TestFormatExchangeRecordIncludesReadableValues(t *testing.T) {
	itemID := uint32(501)
	price := uint64(320000)
	count := uint32(7)
	endTime := uint32(123456)
	buyerCount := uint32(2)
	name := "Chepet Card"

	record := &client.ExchangeItemRecord{
		BaseInfo: &pb.TradeItemBaseInfo{
			Itemid:  &itemID,
			Price:   &price,
			Count:   &count,
			EndTime: &endTime,
			Name:    &name,
		},
		SellInfo: &pb.ItemSellInfoRecordTradeCmd{
			BuyerCount: &buyerCount,
		},
	}

	got := formatExchangeRecord(record, int(itemID))
	want := "501: Chepet Card: 320,000 zeny, 7 listed [SNAPPING: 2 buyer(s), ends at 1970-01-02T10:17:36Z, 0 minutes left]"
	if got != want {
		t.Fatalf("formatExchangeRecord() = %q, want %q", got, want)
	}
}

func TestFormatExchangeRecordWithoutSnapping(t *testing.T) {
	itemID := uint32(601)
	price := uint64(123123)
	count := uint32(1)
	name := "Angeling Card"

	record := &client.ExchangeItemRecord{
		BaseInfo: &pb.TradeItemBaseInfo{
			Itemid: &itemID,
			Price:  &price,
			Count:  &count,
			Name:   &name,
		},
	}

	got := formatExchangeRecord(record, int(itemID))
	want := "601: Angeling Card: 123,123 zeny, 1 listed"
	if got != want {
		t.Fatalf("formatExchangeRecord() = %q, want %q", got, want)
	}
}
