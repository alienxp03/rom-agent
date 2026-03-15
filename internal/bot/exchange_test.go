package bot

import (
	"context"
	"strings"
	"testing"

	"github.com/alienxp03/rom-agent/internal/client"
	"github.com/alienxp03/rom-agent/internal/db"
	pb "github.com/alienxp03/rom-agent/internal/proto/pb"
)

func TestFormatExchangeRecordIncludesReadableValues(t *testing.T) {
	itemID := uint32(501)
	price := uint64(320000)
	count := uint32(7)
	endTime := uint32(123456)
	buyerCount := uint32(2)

	record := &client.ExchangeItemRecord{
		BaseInfo: &pb.TradeItemBaseInfo{
			Itemid:  &itemID,
			Price:   &price,
			Count:   &count,
			EndTime: &endTime,
		},
		SellInfo: &pb.ItemSellInfoRecordTradeCmd{
			BuyerCount: &buyerCount,
		},
	}

	got := formatExchangeRecord(record, int(itemID), nil, nil)
	want := "501: item:501: 320,000 zeny, 7 listed [SNAPPING: 2 buyer(s), ends at 1970-01-02T10:17:36Z, 0 minutes left]"
	if got != want {
		t.Fatalf("formatExchangeRecord() = %q, want %q", got, want)
	}
}

func TestFormatExchangeRecordWithoutSnapping(t *testing.T) {
	itemID := uint32(601)
	price := uint64(123123)
	count := uint32(1)

	record := &client.ExchangeItemRecord{
		BaseInfo: &pb.TradeItemBaseInfo{
			Itemid: &itemID,
			Price:  &price,
			Count:  &count,
		},
	}

	got := formatExchangeRecord(record, int(itemID), nil, nil)
	want := "601: item:601: 123,123 zeny, 1 listed"
	if got != want {
		t.Fatalf("formatExchangeRecord() = %q, want %q", got, want)
	}
}

func TestDoExchangeRequiresScanTargetStore(t *testing.T) {
	bot := &Bot{exchangeMarket: "sea_shared"}

	err := bot.doExchange(context.Background())
	if err == nil {
		t.Fatal("doExchange() error = nil, want missing scan target store error")
	}
	if !strings.Contains(err.Error(), "scan target store") {
		t.Fatalf("doExchange() error = %q, want mention of scan target store", err)
	}
}

func TestDoExchangeRequiresExchangeMarket(t *testing.T) {
	bot := &Bot{scanTargetDb: &db.ScanTargetStore{}}

	err := bot.doExchange(context.Background())
	if err == nil {
		t.Fatal("doExchange() error = nil, want missing exchange market error")
	}
	if !strings.Contains(err.Error(), "exchange market") {
		t.Fatalf("doExchange() error = %q, want mention of exchange market", err)
	}
}
