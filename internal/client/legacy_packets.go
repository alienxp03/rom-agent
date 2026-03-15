package client

import (
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protowire"
)

func decodeLegacyPacket(frame GameFrame) interface{} {
	fields := parseUnknownFields(frame.Payload)
	switch key := (messageKey{id1: frame.ID1, id2: frame.ID2}); key {
	case messageKey{id1: 8, id2: 43}:
		return &LegacyQuestPacket43{}
	case messageKey{id1: 11, id2: 132}:
		return &LegacyFubenPacket132{Entries: decodeLegacyTriples(fields)}
	case messageKey{id1: 11, id2: 155}:
		return &LegacyFubenPacket155{Value: firstVarint(fields, 3)}
	case messageKey{id1: 11, id2: 165}:
		return &LegacyFubenPacket165{Groups: decodeLegacyFubenGroups(fields)}
	case messageKey{id1: 12, id2: 31}:
		return &LegacyMapPacket31{Value: decodeLegacyMapValue(fields)}
	case messageKey{id1: 12, id2: 36}:
		return &LegacyMapPacket36{Value: decodeLegacyMapValue(fields)}
	case messageKey{id1: 25, id2: 72}:
		return &LegacyUserEventPacket72{IDs: decodeLegacyVarints(fields)}
	case messageKey{id1: 25, id2: 76}:
		return &LegacyUserEventPacket76{Groups: decodeLegacyUserEventGroups(fields)}
	case messageKey{id1: 25, id2: 83}:
		return &LegacyUserEventPacket83{Value: firstVarint(fields, 3)}
	case messageKey{id1: 30, id2: 27}:
		return &LegacyPhotoPacket27{}
	case messageKey{id1: 50, id2: 95}:
		return &LegacyGuildPacket95{Records: decodeLegacyGuildRecords(fields)}
	case messageKey{id1: 60, id2: 28}:
		return decodeLegacyActivity28(fields)
	case messageKey{id1: 60, id2: 53}:
		return decodeLegacyActivity53(fields)
	case messageKey{id1: 60, id2: 57}:
		return &LegacyActivityPacket57{}
	case messageKey{id1: 60, id2: 58}:
		return &LegacyActivityPacket58{
			ActivityID: firstVarint(fields, 3),
			Field6:     firstVarint(fields, 6),
			Field7:     firstVarint(fields, 7),
		}
	case messageKey{id1: 60, id2: 61}:
		return decodeLegacyActivity61(fields)
	case messageKey{id1: 60, id2: 64}:
		return decodeLegacyActivity64(fields)
	case messageKey{id1: 61, id2: 68}:
		return &LegacyMatchPacket68{
			Field5: firstVarint(fields, 5),
			Field6: firstVarint(fields, 6),
			Field7: firstVarint(fields, 7),
			Field8: firstVarint(fields, 8),
		}
	case messageKey{id1: 82, id2: 6}:
		return &LegacyCommand82TogglePacket{Value: firstVarint(fields, 3)}
	case messageKey{id1: 82, id2: 9}:
		return &LegacyBossBoardLabelsPacket{Entries: decodeBossBoardLabelEntries(fields)}
	case messageKey{id1: 82, id2: 49}:
		return &LegacyCommand82EmptyPacket49{}
	case messageKey{id1: 83, id2: 10}:
		return &LegacyCommand83StatusPacket{
			Field3: firstVarint(fields, 3),
			Field4: firstVarint(fields, 4),
			Field5: firstVarint(fields, 5),
		}
	case messageKey{id1: 83, id2: 21}:
		return &LegacyCommand83EmptyPacket21{}
	default:
		return nil
	}
}

func decodeLegacyActivity28(fields []unknownField) *LegacyActivityPacket28 {
	nested, ok := fieldNested(fields, 3)
	if !ok {
		return &LegacyActivityPacket28{}
	}
	return &LegacyActivityPacket28{
		ActivityID: firstVarint(nested, 1),
		Open:       firstVarint(nested, 2) != 0,
	}
}

func decodeLegacyActivity53(fields []unknownField) *LegacyActivityPacket53 {
	nested, ok := fieldNested(fields, 3)
	if !ok {
		return &LegacyActivityPacket53{}
	}
	packet := &LegacyActivityPacket53{ActivityID: firstVarint(nested, 1)}
	for _, field := range nested {
		child, ok := field.Value.([]unknownField)
		if field.Number != 2 || !ok {
			continue
		}
		packet.Rewards = append(packet.Rewards, LegacyActivityReward{
			ID:     firstVarint(child, 1),
			State1: firstVarint(child, 2),
			State2: firstVarint(child, 3),
		})
	}
	return packet
}

func decodeLegacyActivity61(fields []unknownField) *LegacyActivityPacket61 {
	packet := &LegacyActivityPacket61{Marker: firstVarint(fields, 4)}
	for _, field := range fields {
		child, ok := field.Value.([]unknownField)
		if field.Number != 3 || !ok {
			continue
		}
		packet.Windows = append(packet.Windows, LegacyActivityWindow{
			ID:        firstVarint(child, 1),
			StartTime: firstVarint(child, 2),
			EndTime:   firstVarint(child, 3),
			DateCode:  firstVarint(child, 4),
		})
	}
	return packet
}

func decodeLegacyActivity64(fields []unknownField) *LegacyActivityPacket64 {
	packet := &LegacyActivityPacket64{ActivityID: firstVarint(fields, 3)}
	for _, field := range fields {
		child, ok := field.Value.([]unknownField)
		if field.Number != 4 || !ok {
			continue
		}
		packet.Items = append(packet.Items, LegacyActivityItem{
			ID:     firstVarint(child, 1),
			State1: firstVarint(child, 2),
			State2: firstVarint(child, 3),
		})
	}
	return packet
}

func decodeLegacyMapValue(fields []unknownField) uint64 {
	if nested, ok := fieldNested(fields, 3); ok {
		if value := firstVarint(nested, 1); value != 0 {
			return value
		}
	}
	return firstVarint(fields, 3)
}

func decodeLegacyTriples(fields []unknownField) []LegacyFubenEntry {
	out := make([]LegacyFubenEntry, 0, len(fields))
	for _, field := range fields {
		child, ok := field.Value.([]unknownField)
		if field.Number != 3 || !ok {
			continue
		}
		out = append(out, LegacyFubenEntry{
			ID:     firstVarint(child, 1),
			Value1: firstVarint(child, 2),
			Value2: firstVarint(child, 3),
		})
	}
	return out
}

func decodeLegacyFubenGroups(fields []unknownField) []LegacyFubenGroup {
	out := make([]LegacyFubenGroup, 0, len(fields))
	for _, field := range fields {
		child, ok := field.Value.([]unknownField)
		if field.Number != 3 || !ok {
			continue
		}
		group := LegacyFubenGroup{GroupID: firstVarint(child, 1)}
		for _, member := range child {
			if member.Number == 2 {
				group.Members = append(group.Members, varintValue(member))
			}
		}
		out = append(out, group)
	}
	return out
}

func decodeLegacyVarints(fields []unknownField) []uint64 {
	out := make([]uint64, 0, len(fields))
	for _, field := range fields {
		if v := varintValue(field); v != 0 {
			out = append(out, v)
		}
	}
	return out
}

func decodeLegacyUserEventGroups(fields []unknownField) []LegacyUserEventGroup {
	out := make([]LegacyUserEventGroup, 0, len(fields))
	for _, field := range fields {
		child, ok := field.Value.([]unknownField)
		if field.Number != 3 || !ok {
			continue
		}
		out = append(out, LegacyUserEventGroup{
			GroupID:   firstVarint(child, 1),
			ItemCount: countRepeated(child, 2),
		})
	}
	return out
}

func decodeLegacyGuildRecords(fields []unknownField) []LegacyGuildRecord {
	out := make([]LegacyGuildRecord, 0, len(fields))
	for _, field := range fields {
		child, ok := field.Value.([]unknownField)
		if field.Number != 3 || !ok {
			continue
		}
		out = append(out, LegacyGuildRecord{
			ID:      firstVarint(child, 1),
			Value1:  firstVarint(child, 2),
			Value2:  firstVarint(child, 3),
			UnixSec: firstVarint(child, 4),
		})
	}
	return out
}

func decodeBossBoardLabelEntries(fields []unknownField) []BossBoardLabelEntry {
	out := make([]BossBoardLabelEntry, 0, len(fields))
	for _, field := range fields {
		child, ok := field.Value.([]unknownField)
		if field.Number != 3 || !ok {
			continue
		}
		key := nestedString(child, 1)
		value := nestedString(child, 2)
		if decoded := decodeHexText(value); decoded != "" {
			value = decoded
		}
		if key == "" && value == "" {
			continue
		}
		out = append(out, BossBoardLabelEntry{Key: key, Value: value})
	}
	return out
}

func fieldNested(fields []unknownField, number protowire.Number) ([]unknownField, bool) {
	for _, field := range fields {
		if field.Number != number {
			continue
		}
		nested, ok := field.Value.([]unknownField)
		if ok {
			return nested, true
		}
	}
	return nil, false
}

func summarizeLegacyPacket(key messageKey, fields []unknownField) string {
	switch key {
	case messageKey{id1: 60, id2: 28}:
		return summarizeActivity28(fields)
	case messageKey{id1: 60, id2: 53}:
		return summarizeActivity53(fields)
	case messageKey{id1: 60, id2: 57}:
		return "empty"
	case messageKey{id1: 60, id2: 58}:
		return summarizeActivity58(fields)
	case messageKey{id1: 60, id2: 61}:
		return summarizeActivity61(fields)
	case messageKey{id1: 60, id2: 64}:
		return summarizeActivity64(fields)
	case messageKey{id1: 11, id2: 132}:
		return summarizeTriples("entries", fields)
	case messageKey{id1: 11, id2: 155}:
		return summarizeScalar("value", fields)
	case messageKey{id1: 11, id2: 165}:
		return summarizeGroupedLists(fields)
	case messageKey{id1: 25, id2: 72}:
		return summarizeVarintList("ids", fields)
	case messageKey{id1: 25, id2: 76}:
		return summarizeNestedGroups("groups", fields)
	case messageKey{id1: 25, id2: 83}:
		return summarizeScalar("value", fields)
	case messageKey{id1: 50, id2: 95}:
		return summarizeTriples("records", fields)
	case messageKey{id1: 61, id2: 68}:
		return summarizeFlatFields(fields)
	case messageKey{id1: 82, id2: 6}:
		return summarizeScalar("value", fields)
	case messageKey{id1: 82, id2: 9}:
		return summarizeCommand82Packet9(fields)
	case messageKey{id1: 82, id2: 49}:
		return "empty"
	case messageKey{id1: 83, id2: 10}:
		return summarizeFlatFields(fields)
	case messageKey{id1: 83, id2: 21}:
		return "empty"
	default:
		return ""
	}
}

func summarizeActivity28(fields []unknownField) string {
	if len(fields) != 1 {
		return ""
	}
	nested, ok := fields[0].Value.([]unknownField)
	if !ok {
		return ""
	}
	return fmt.Sprintf("activity=%d open=%t", firstVarint(nested, 1), firstVarint(nested, 2) != 0)
}

func summarizeActivity53(fields []unknownField) string {
	if len(fields) < 1 {
		return ""
	}
	nested, ok := fields[0].Value.([]unknownField)
	if !ok {
		return ""
	}
	var subIDs []string
	for _, field := range nested {
		child, ok := field.Value.([]unknownField)
		if field.Number != 2 || !ok {
			continue
		}
		subIDs = append(subIDs, fmt.Sprintf("%d:%d/%d", firstVarint(child, 1), firstVarint(child, 2), firstVarint(child, 3)))
		if len(subIDs) == 5 {
			break
		}
	}
	return fmt.Sprintf("activity=%d rewards=[%s]", firstVarint(nested, 1), strings.Join(subIDs, ","))
}

func summarizeActivity58(fields []unknownField) string {
	if len(fields) == 0 {
		return ""
	}
	return fmt.Sprintf("activity=%d flags=%s", firstVarint(fields, 3), summarizeFlatFields(fields))
}

func summarizeActivity61(fields []unknownField) string {
	var windows []string
	var marker uint64
	for _, field := range fields {
		if field.Number == 3 {
			nested, ok := field.Value.([]unknownField)
			if !ok {
				continue
			}
			windows = append(windows, fmt.Sprintf("%d:%d-%d", firstVarint(nested, 1), firstVarint(nested, 2), firstVarint(nested, 3)))
		}
		if field.Number == 4 {
			marker = varintValue(field)
		}
	}
	if len(windows) > 4 {
		windows = append(windows[:4], "...")
	}
	return fmt.Sprintf("windows=[%s] marker=%d", strings.Join(windows, ","), marker)
}

func summarizeActivity64(fields []unknownField) string {
	var items []string
	for _, field := range fields {
		if field.Number != 4 {
			continue
		}
		nested, ok := field.Value.([]unknownField)
		if !ok {
			continue
		}
		items = append(items, fmt.Sprintf("%d:%d/%d", firstVarint(nested, 1), firstVarint(nested, 2), firstVarint(nested, 3)))
	}
	return fmt.Sprintf("activity=%d items=[%s]", firstVarint(fields, 3), strings.Join(items, ","))
}

func summarizeTriples(label string, fields []unknownField) string {
	var out []string
	for _, field := range fields {
		nested, ok := field.Value.([]unknownField)
		if field.Number != 3 || !ok {
			continue
		}
		out = append(out, fmt.Sprintf("%d:%d/%d", firstVarint(nested, 1), firstVarint(nested, 2), firstVarint(nested, 3)))
		if len(out) == 6 {
			break
		}
	}
	return fmt.Sprintf("%s=[%s]", label, strings.Join(out, ","))
}

func summarizeGroupedLists(fields []unknownField) string {
	var out []string
	for _, field := range fields {
		nested, ok := field.Value.([]unknownField)
		if field.Number != 3 || !ok {
			continue
		}
		var members []string
		for _, child := range nested {
			if child.Number == 2 {
				members = append(members, fmt.Sprintf("%d", varintValue(child)))
				if len(members) == 5 {
					break
				}
			}
		}
		out = append(out, fmt.Sprintf("%d=[%s]", firstVarint(nested, 1), strings.Join(members, ",")))
	}
	return fmt.Sprintf("groups={%s}", strings.Join(out, " "))
}

func summarizeVarintList(label string, fields []unknownField) string {
	var out []string
	for _, field := range fields {
		if v := varintValue(field); v != 0 {
			out = append(out, fmt.Sprintf("%d", v))
		}
		if len(out) == 8 {
			break
		}
	}
	return fmt.Sprintf("%s=[%s]", label, strings.Join(out, ","))
}

func summarizeNestedGroups(label string, fields []unknownField) string {
	var out []string
	for _, field := range fields {
		nested, ok := field.Value.([]unknownField)
		if field.Number != 3 || !ok {
			continue
		}
		out = append(out, fmt.Sprintf("%d(items=%d)", firstVarint(nested, 1), countRepeated(nested, 2)))
		if len(out) == 6 {
			break
		}
	}
	return fmt.Sprintf("%s=[%s]", label, strings.Join(out, ","))
}

func summarizeScalar(label string, fields []unknownField) string {
	if len(fields) == 0 {
		return "empty"
	}
	return fmt.Sprintf("%s=%d", label, firstVarint(fields, fields[0].Number))
}

func summarizeFlatFields(fields []unknownField) string {
	var out []string
	for _, field := range fields {
		out = append(out, fmt.Sprintf("%d=%d", field.Number, varintValue(field)))
	}
	return strings.Join(out, " ")
}

func summarizeCommand82Packet9(fields []unknownField) string {
	var entries []string
	for _, field := range fields {
		nested, ok := field.Value.([]unknownField)
		if field.Number != 3 || !ok {
			continue
		}
		name := nestedString(nested, 1)
		value := nestedString(nested, 2)
		if decoded := decodeHexText(value); decoded != "" {
			value = decoded
		}
		if name == "" && value == "" {
			continue
		}
		if name == "" {
			entries = append(entries, value)
		} else {
			entries = append(entries, fmt.Sprintf("%s=%s", name, value))
		}
		if len(entries) == 6 {
			break
		}
	}
	if len(entries) == 0 {
		return ""
	}
	return fmt.Sprintf("entries=[%s]", strings.Join(entries, ", "))
}

func firstVarint(fields []unknownField, number protowire.Number) uint64 {
	for _, field := range fields {
		if field.Number == number {
			return varintValue(field)
		}
	}
	return 0
}

func countRepeated(fields []unknownField, number protowire.Number) int {
	count := 0
	for _, field := range fields {
		if field.Number == number {
			count++
		}
	}
	return count
}

func varintValue(field unknownField) uint64 {
	if v, ok := field.Value.(uint64); ok {
		return v
	}
	return 0
}

func nestedString(fields []unknownField, number protowire.Number) string {
	for _, field := range fields {
		if field.Number != number {
			continue
		}
		if s, ok := field.Value.(string); ok {
			return s
		}
	}
	return ""
}

func decodeHexText(value string) string {
	if !strings.HasPrefix(value, "0x") || len(value)%2 != 0 {
		return ""
	}
	raw, err := hex.DecodeString(value[2:])
	if err != nil {
		return ""
	}
	if utf8.Valid(raw) {
		return string(raw)
	}
	decoded := strings.TrimSpace(string(raw))
	decoded = strings.Map(func(r rune) rune {
		if r == utf8.RuneError {
			return -1
		}
		return r
	}, decoded)
	if decoded == "" {
		return ""
	}
	return decoded
}
