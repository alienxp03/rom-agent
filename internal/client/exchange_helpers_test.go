package client

import (
	"testing"

	pb "github.com/alienxp03/rom-agent/internal/proto/pb"
)

func TestExchangeCategoryListMergesPublicAndRegularItems(t *testing.T) {
	pubLists := []uint32{1001, 1002}
	lists := []uint32{2001, 2002}
	resp := &pb.BriefPendingListRecordTradeCmd{
		PubLists: pubLists,
		Lists:    lists,
	}

	merged := mergeExchangeItemIDs(resp)

	want := []int{1001, 1002, 2001, 2002}
	if len(merged) != len(want) {
		t.Fatalf("len(merged) = %d, want %d", len(merged), len(want))
	}
	for i := range want {
		if merged[i] != want[i] {
			t.Fatalf("merged[%d] = %d, want %d", i, merged[i], want[i])
		}
	}
}
