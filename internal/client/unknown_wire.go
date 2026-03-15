package client

import (
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protowire"
)

type unknownField struct {
	Number protowire.Number
	Kind   protowire.Type
	Value  any
}

func parseUnknownFields(payload []byte) []unknownField {
	fields, ok := consumeUnknownFields(payload, 0)
	if !ok {
		return nil
	}
	return fields
}

func consumeUnknownFields(payload []byte, depth int) ([]unknownField, bool) {
	if len(payload) == 0 {
		return nil, true
	}

	fields := make([]unknownField, 0, 4)
	for len(payload) > 0 {
		num, typ, n := protowire.ConsumeTag(payload)
		if n < 0 {
			return nil, false
		}
		payload = payload[n:]

		var value any
		switch typ {
		case protowire.VarintType:
			v, n := protowire.ConsumeVarint(payload)
			if n < 0 {
				return nil, false
			}
			value = v
			payload = payload[n:]
		case protowire.Fixed32Type:
			v, n := protowire.ConsumeFixed32(payload)
			if n < 0 {
				return nil, false
			}
			value = v
			payload = payload[n:]
		case protowire.Fixed64Type:
			v, n := protowire.ConsumeFixed64(payload)
			if n < 0 {
				return nil, false
			}
			value = v
			payload = payload[n:]
		case protowire.BytesType:
			v, n := protowire.ConsumeBytes(payload)
			if n < 0 {
				return nil, false
			}
			value = summarizeUnknownBytes(v, depth)
			payload = payload[n:]
		default:
			return nil, false
		}

		fields = append(fields, unknownField{
			Number: num,
			Kind:   typ,
			Value:  value,
		})
	}

	return fields, true
}

func summarizeUnknownBytes(payload []byte, depth int) any {
	if len(payload) == 0 {
		return ""
	}

	if depth < 2 {
		if nested, ok := consumeUnknownFields(payload, depth+1); ok && len(nested) > 0 {
			return nested
		}
	}

	if isMostlyPrintable(payload) {
		return string(payload)
	}

	prefixLen := len(payload)
	if prefixLen > 16 {
		prefixLen = 16
	}
	return "0x" + hex.EncodeToString(payload[:prefixLen])
}

func isMostlyPrintable(payload []byte) bool {
	if !utf8.Valid(payload) {
		return false
	}
	printable := 0
	for _, r := range string(payload) {
		if r == '\n' || r == '\r' || r == '\t' || (r >= 0x20 && r <= 0x7e) {
			printable++
		}
	}
	return printable > 0 && printable == len([]rune(string(payload)))
}

func summarizeUnknownFields(fields []unknownField) string {
	if len(fields) == 0 {
		return "[]"
	}

	parts := make([]string, 0, minInt(len(fields), 6))
	for i, field := range fields {
		if i == 6 {
			parts = append(parts, "...")
			break
		}
		parts = append(parts, formatUnknownField(field))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func formatUnknownField(field unknownField) string {
	switch value := field.Value.(type) {
	case []unknownField:
		return fmt.Sprintf("%d:{%s}", field.Number, summarizeUnknownFields(value))
	case string:
		if value == "" {
			return fmt.Sprintf("%d:\"\"", field.Number)
		}
		return fmt.Sprintf("%d:%q", field.Number, value)
	case uint64:
		return fmt.Sprintf("%d:%d", field.Number, value)
	case uint32:
		return fmt.Sprintf("%d:%d", field.Number, value)
	default:
		return fmt.Sprintf("%d:%v", field.Number, value)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
