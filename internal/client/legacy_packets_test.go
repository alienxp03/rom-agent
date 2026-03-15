package client

import (
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func TestDecodeLegacyMapPacket31(t *testing.T) {
	payload := appendNestedVarintField(3, 1, 48)

	msg := decodeLegacyPacket(GameFrame{
		ID1:     12,
		ID2:     31,
		Payload: payload,
	})

	packet, ok := msg.(*LegacyMapPacket31)
	if !ok {
		t.Fatalf("decodeLegacyPacket returned %T, want *LegacyMapPacket31", msg)
	}
	if packet.Value != 48 {
		t.Fatalf("LegacyMapPacket31.Value = %d, want 48", packet.Value)
	}
}

func TestDecodeLegacyMapPacket36NestedValue(t *testing.T) {
	payload := appendNestedVarintField(3, 1, 77)

	msg := decodeLegacyPacket(GameFrame{
		ID1:     12,
		ID2:     36,
		Payload: payload,
	})

	packet, ok := msg.(*LegacyMapPacket36)
	if !ok {
		t.Fatalf("decodeLegacyPacket returned %T, want *LegacyMapPacket36", msg)
	}
	if packet.Value != 77 {
		t.Fatalf("LegacyMapPacket36.Value = %d, want 77", packet.Value)
	}
}

func TestDecodeLegacyCommand82TogglePacket(t *testing.T) {
	payload := appendVarintField(nil, 3, 1)

	msg := decodeLegacyPacket(GameFrame{
		ID1:     82,
		ID2:     6,
		Payload: payload,
	})

	packet, ok := msg.(*LegacyCommand82TogglePacket)
	if !ok {
		t.Fatalf("decodeLegacyPacket returned %T, want *LegacyCommand82TogglePacket", msg)
	}
	if packet.Value != 1 {
		t.Fatalf("LegacyCommand82TogglePacket.Value = %d, want 1", packet.Value)
	}
}

func TestDecodeLegacyCommand83StatusPacket(t *testing.T) {
	payload := appendVarintField(nil, 3, 7)
	payload = appendVarintField(payload, 4, 8)
	payload = appendVarintField(payload, 5, 9)

	msg := decodeLegacyPacket(GameFrame{
		ID1:     83,
		ID2:     10,
		Payload: payload,
	})

	packet, ok := msg.(*LegacyCommand83StatusPacket)
	if !ok {
		t.Fatalf("decodeLegacyPacket returned %T, want *LegacyCommand83StatusPacket", msg)
	}
	if packet.Field3 != 7 || packet.Field4 != 8 || packet.Field5 != 9 {
		t.Fatalf("LegacyCommand83StatusPacket = %#v, want fields 7/8/9", packet)
	}
}

func appendNestedVarintField(parentNum, childNum protowire.Number, value uint64) []byte {
	child := appendVarintField(nil, childNum, value)
	buf := protowire.AppendTag(nil, parentNum, protowire.BytesType)
	return protowire.AppendBytes(buf, child)
}

func appendVarintField(buf []byte, num protowire.Number, value uint64) []byte {
	buf = protowire.AppendTag(buf, num, protowire.VarintType)
	return protowire.AppendVarint(buf, value)
}
