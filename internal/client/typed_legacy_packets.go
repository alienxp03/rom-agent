package client

import (
	"fmt"
	"strings"
)

type LegacyQuestPacket43 struct{}

func (m *LegacyQuestPacket43) String() string {
	return "LegacyQuestPacket43 cmd=SCENE_USER_QUEST_PROTOCMD(8) param=QUESTPARAM_43(43) empty"
}

type LegacyMapPacket36 struct {
	Value uint64
}

type LegacyMapPacket31 struct {
	Value uint64
}

func (m *LegacyMapPacket31) String() string {
	return fmt.Sprintf("LegacyMapPacket31 cmd=SCENE_USER_MAP_PROTOCMD(12) param=31(31) value=%d", m.Value)
}

func (m *LegacyMapPacket36) String() string {
	return fmt.Sprintf("LegacyMapPacket36 cmd=SCENE_USER_MAP_PROTOCMD(12) param=MAPPARAM_36(36) value=%d", m.Value)
}

type LegacyPhotoPacket27 struct{}

func (m *LegacyPhotoPacket27) String() string {
	return "LegacyPhotoPacket27 cmd=SCENE_USER_PHOTO_PROTOCMD(30) param=PHOTOPARAM_27(27) empty"
}

type LegacyActivityPacket28 struct {
	ActivityID uint64
	Open       bool
}

func (m *LegacyActivityPacket28) String() string {
	return fmt.Sprintf("LegacyActivityPacket28 cmd=ACTIVITY_PROTOCMD(60) param=28(28) activity=%d open=%t", m.ActivityID, m.Open)
}

type LegacyActivityReward struct {
	ID     uint64
	State1 uint64
	State2 uint64
}

type LegacyActivityPacket53 struct {
	ActivityID uint64
	Rewards    []LegacyActivityReward
}

func (m *LegacyActivityPacket53) String() string {
	parts := make([]string, 0, minInt(len(m.Rewards), 5))
	for i, reward := range m.Rewards {
		if i == 5 {
			break
		}
		parts = append(parts, fmt.Sprintf("%d:%d/%d", reward.ID, reward.State1, reward.State2))
	}
	return fmt.Sprintf("LegacyActivityPacket53 cmd=ACTIVITY_PROTOCMD(60) param=53(53) activity=%d rewards=[%s]",
		m.ActivityID, strings.Join(parts, ","))
}

type LegacyActivityPacket57 struct{}

func (m *LegacyActivityPacket57) String() string {
	return "LegacyActivityPacket57 cmd=ACTIVITY_PROTOCMD(60) param=57(57) empty"
}

type LegacyActivityPacket58 struct {
	ActivityID uint64
	Field6     uint64
	Field7     uint64
}

func (m *LegacyActivityPacket58) String() string {
	return fmt.Sprintf("LegacyActivityPacket58 cmd=ACTIVITY_PROTOCMD(60) param=58(58) activity=%d flags=3=%d 6=%d 7=%d",
		m.ActivityID, m.ActivityID, m.Field6, m.Field7)
}

type LegacyActivityWindow struct {
	ID        uint64
	StartTime uint64
	EndTime   uint64
	DateCode  uint64
}

type LegacyActivityPacket61 struct {
	Windows []LegacyActivityWindow
	Marker  uint64
}

func (m *LegacyActivityPacket61) String() string {
	parts := make([]string, 0, minInt(len(m.Windows), 4))
	for i, window := range m.Windows {
		if i == 4 {
			break
		}
		parts = append(parts, fmt.Sprintf("%d:%d-%d", window.ID, window.StartTime, window.EndTime))
	}
	return fmt.Sprintf("LegacyActivityPacket61 cmd=ACTIVITY_PROTOCMD(60) param=61(61) windows=[%s] marker=%d",
		strings.Join(parts, ","), m.Marker)
}

type LegacyActivityItem struct {
	ID     uint64
	State1 uint64
	State2 uint64
}

type LegacyActivityPacket64 struct {
	ActivityID uint64
	Items      []LegacyActivityItem
}

func (m *LegacyActivityPacket64) String() string {
	parts := make([]string, 0, len(m.Items))
	for _, item := range m.Items {
		parts = append(parts, fmt.Sprintf("%d:%d/%d", item.ID, item.State1, item.State2))
	}
	return fmt.Sprintf("LegacyActivityPacket64 cmd=ACTIVITY_PROTOCMD(60) param=64(64) activity=%d items=[%s]",
		m.ActivityID, strings.Join(parts, ","))
}

type LegacyUserEventPacket72 struct {
	IDs []uint64
}

func (m *LegacyUserEventPacket72) String() string {
	parts := make([]string, 0, minInt(len(m.IDs), 8))
	for i, id := range m.IDs {
		if i == 8 {
			break
		}
		parts = append(parts, fmt.Sprintf("%d", id))
	}
	return fmt.Sprintf("LegacyUserEventPacket72 cmd=USER_EVENT_PROTOCMD(25) param=72(72) ids=[%s]", strings.Join(parts, ","))
}

type LegacyUserEventGroup struct {
	GroupID   uint64
	ItemCount int
}

type LegacyUserEventPacket76 struct {
	Groups []LegacyUserEventGroup
}

func (m *LegacyUserEventPacket76) String() string {
	parts := make([]string, 0, minInt(len(m.Groups), 6))
	for i, group := range m.Groups {
		if i == 6 {
			break
		}
		parts = append(parts, fmt.Sprintf("%d(items=%d)", group.GroupID, group.ItemCount))
	}
	return fmt.Sprintf("LegacyUserEventPacket76 cmd=USER_EVENT_PROTOCMD(25) param=76(76) groups=[%s]", strings.Join(parts, ","))
}

type LegacyUserEventPacket83 struct {
	Value uint64
}

func (m *LegacyUserEventPacket83) String() string {
	return fmt.Sprintf("LegacyUserEventPacket83 cmd=USER_EVENT_PROTOCMD(25) param=83(83) value=%d", m.Value)
}

type LegacyFubenEntry struct {
	ID     uint64
	Value1 uint64
	Value2 uint64
}

type LegacyFubenPacket132 struct {
	Entries []LegacyFubenEntry
}

func (m *LegacyFubenPacket132) String() string {
	parts := make([]string, 0, minInt(len(m.Entries), 6))
	for i, entry := range m.Entries {
		if i == 6 {
			break
		}
		parts = append(parts, fmt.Sprintf("%d:%d/%d", entry.ID, entry.Value1, entry.Value2))
	}
	return fmt.Sprintf("LegacyFubenPacket132 cmd=FUBEN_PROTOCMD(11) param=132(132) entries=[%s]", strings.Join(parts, ","))
}

type LegacyFubenPacket155 struct {
	Value uint64
}

func (m *LegacyFubenPacket155) String() string {
	return fmt.Sprintf("LegacyFubenPacket155 cmd=FUBEN_PROTOCMD(11) param=155(155) value=%d", m.Value)
}

type LegacyFubenGroup struct {
	GroupID uint64
	Members []uint64
}

type LegacyFubenPacket165 struct {
	Groups []LegacyFubenGroup
}

func (m *LegacyFubenPacket165) String() string {
	parts := make([]string, 0, len(m.Groups))
	for _, group := range m.Groups {
		memberParts := make([]string, 0, minInt(len(group.Members), 5))
		for i, member := range group.Members {
			if i == 5 {
				break
			}
			memberParts = append(memberParts, fmt.Sprintf("%d", member))
		}
		parts = append(parts, fmt.Sprintf("%d=[%s]", group.GroupID, strings.Join(memberParts, ",")))
	}
	return fmt.Sprintf("LegacyFubenPacket165 cmd=FUBEN_PROTOCMD(11) param=165(165) groups={%s}", strings.Join(parts, " "))
}

type LegacyGuildRecord struct {
	ID      uint64
	Value1  uint64
	Value2  uint64
	UnixSec uint64
}

type LegacyGuildPacket95 struct {
	Records []LegacyGuildRecord
}

func (m *LegacyGuildPacket95) String() string {
	parts := make([]string, 0, minInt(len(m.Records), 6))
	for i, record := range m.Records {
		if i == 6 {
			break
		}
		parts = append(parts, fmt.Sprintf("%d:%d/%d", record.ID, record.Value1, record.Value2))
	}
	return fmt.Sprintf("LegacyGuildPacket95 cmd=SESSION_USER_GUILD_PROTOCMD(50) param=95(95) records=[%s]", strings.Join(parts, ","))
}

type LegacyMatchPacket68 struct {
	Field5 uint64
	Field6 uint64
	Field7 uint64
	Field8 uint64
}

func (m *LegacyMatchPacket68) String() string {
	return fmt.Sprintf("LegacyMatchPacket68 cmd=MATCHC_PROTOCMD(61) param=68(68) 5=%d 6=%d 7=%d 8=%d",
		m.Field5, m.Field6, m.Field7, m.Field8)
}

type LegacyCommand82TogglePacket struct {
	Value uint64
}

func (m *LegacyCommand82TogglePacket) String() string {
	return fmt.Sprintf("LegacyCommand82TogglePacket cmd=%s(82) param=%s(6) value=%d",
		commandName(82), paramName(82, 6), m.Value)
}

type BossBoardLabelEntry struct {
	Key   string
	Value string
}

type LegacyBossBoardLabelsPacket struct {
	Entries []BossBoardLabelEntry
}

func (m *LegacyBossBoardLabelsPacket) String() string {
	parts := make([]string, 0, minInt(len(m.Entries), 6))
	for i, entry := range m.Entries {
		if i == 6 {
			break
		}
		if entry.Key == "" {
			parts = append(parts, entry.Value)
		} else {
			parts = append(parts, fmt.Sprintf("%s=%s", entry.Key, entry.Value))
		}
	}
	return fmt.Sprintf("LegacyBossBoardLabelsPacket cmd=%s(82) param=%s(9) entries=[%s]",
		commandName(82), paramName(82, 9), strings.Join(parts, ", "))
}

type LegacyCommand82EmptyPacket49 struct{}

func (m *LegacyCommand82EmptyPacket49) String() string {
	return fmt.Sprintf("LegacyCommand82EmptyPacket49 cmd=%s(82) param=%s(49) empty", commandName(82), paramName(82, 49))
}

type LegacyCommand83StatusPacket struct {
	Field3 uint64
	Field4 uint64
	Field5 uint64
}

func (m *LegacyCommand83StatusPacket) String() string {
	return fmt.Sprintf("LegacyCommand83StatusPacket cmd=%s(83) param=%s(10) 3=%d 4=%d 5=%d",
		commandName(83), paramName(83, 10), m.Field3, m.Field4, m.Field5)
}

type LegacyCommand83EmptyPacket21 struct{}

func (m *LegacyCommand83EmptyPacket21) String() string {
	return fmt.Sprintf("LegacyCommand83EmptyPacket21 cmd=%s(83) param=%s(21) empty", commandName(83), paramName(83, 21))
}
