package client

import (
	"encoding/hex"
	"fmt"

	pb "github.com/alienxp03/rom-agent/internal/proto/pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type messageKey struct {
	id1 byte
	id2 byte
}

type UnknownMessage struct {
	ID1     byte
	ID2     byte
	Payload []byte
	Fields  []unknownField
}

func (m *UnknownMessage) String() string {
	prefixLen := len(m.Payload)
	if prefixLen > 24 {
		prefixLen = 24
	}
	return fmt.Sprintf("unknown message cmd=%s(%d) param=%s(%d) len=%d fields=%s prefix_hex=%s",
		commandName(m.ID1), m.ID1, paramName(m.ID1, m.ID2), m.ID2, len(m.Payload), summarizeUnknownFields(m.Fields), hex.EncodeToString(m.Payload[:prefixLen]))
}

var inboundMessageRegistry = buildInboundMessageRegistry()

func buildInboundMessageRegistry() map[messageKey]func() proto.Message {
	registry := make(map[messageKey]func() proto.Message)

	protoregistry.GlobalTypes.RangeMessages(func(mt protoreflect.MessageType) bool {
		md := mt.Descriptor()
		cmdField := md.Fields().ByName("cmd")
		paramField := md.Fields().ByName("param")
		if cmdField == nil || paramField == nil {
			return true
		}
		if cmdField.Kind() != protoreflect.EnumKind || paramField.Kind() != protoreflect.EnumKind {
			return true
		}

		cmd := cmdField.Default().Enum()
		param := paramField.Default().Enum()
		if cmd < 0 || cmd > 255 || param < 0 || param > 255 {
			return true
		}

		key := messageKey{id1: byte(cmd), id2: byte(param)}
		if _, exists := registry[key]; exists {
			return true
		}

		registry[key] = func() proto.Message {
			return mt.New().Interface()
		}
		return true
	})

	registerInboundAliases(registry)

	return registry
}

func registerInboundAliases(registry map[messageKey]func() proto.Message) {
	// Some live SEA2 packets still arrive on the older item command family while
	// carrying the message shapes that later moved under SCENE_USER2.
	registerInboundAlias(registry, 6, 95, func() proto.Message { return &pb.HandStatusUserCmd{} })
	registerInboundAlias(registry, 6, 124, func() proto.Message { return &pb.ServantService{} })
	registerInboundAlias(registry, 6, 127, func() proto.Message { return &pb.ServantRewardStatusUserCmd{} })
	registerInboundAlias(registry, 6, 140, func() proto.Message { return &pb.UpdateBranchInfoUserCmd{} })
	registerInboundAlias(registry, 6, 142, func() proto.Message { return &pb.InviteWithMeUserCmd{} })
	registerInboundAlias(registry, 6, 153, func() proto.Message { return &pb.UseDeathTransferCmd{} })
	registerInboundAlias(registry, 6, 161, func() proto.Message { return &pb.UnlockFrameUserCmd{} })
	registerInboundAlias(registry, 6, 162, func() proto.Message { return &pb.KFCEnrollUserCmd{} })
	registerInboundAlias(registry, 6, 165, func() proto.Message { return &pb.SignInNtfUserCmd{} })
	registerInboundAlias(registry, 6, 168, func() proto.Message { return &pb.KFCEnrollCodeUserCmd{} })
}

func registerInboundAlias(registry map[messageKey]func() proto.Message, id1, id2 byte, factory func() proto.Message) {
	key := messageKey{id1: id1, id2: id2}
	if _, exists := registry[key]; exists {
		return
	}
	registry[key] = factory
}

func commandName(id1 byte) string {
	switch id1 {
	case 82:
		return "LEGACY_PROTOCMD_82"
	case 83:
		return "LEGACY_PROTOCMD_83"
	}
	return pb.Command(id1).String()
}

func paramName(id1, id2 byte) string {
	switch id1 {
	case 82:
		switch id2 {
		case 6:
			return "param_toggle"
		case 9:
			return "param_boss_board_labels"
		case 49:
			return "param_empty"
		}
	case 83:
		switch id2 {
		case 10:
			return "param_status"
		case 21:
			return "param_empty"
		}
	}
	switch pb.Command(id1) {
	case pb.Command_LOGIN_USER_PROTOCMD:
		return pb.LoginCmdParam(id2).String()
	case pb.Command_ERROR_USER_PROTOCMD:
		return pb.ErrCmdParam(id2).String()
	case pb.Command_SCENE_USER_PROTOCMD:
		return pb.CmdParam(id2).String()
	case pb.Command_SCENE_USER_ITEM_PROTOCMD:
		return pb.ItemParam(id2).String()
	case pb.Command_SCENE_USER_SKILL_PROTOCMD:
		return pb.SkillParam(id2).String()
	case pb.Command_SCENE_USER_QUEST_PROTOCMD:
		return pb.QuestParam(id2).String()
	case pb.Command_SCENE_USER2_PROTOCMD:
		return pb.User2Param(id2).String()
	case pb.Command_SCENE_USER_PET_PROTOCMD:
		return pb.PetParam(id2).String()
	case pb.Command_FUBEN_PROTOCMD:
		return pb.FuBenParam(id2).String()
	case pb.Command_SCENE_USER_MAP_PROTOCMD:
		return pb.MapParam(id2).String()
	case pb.Command_SCENE_BOSS_PROTOCMD:
		return pb.BossParam(id2).String()
	case pb.Command_SCENE_USER_CHAT_PROTOCMD:
		return pb.ChatParam(id2).String()
	case pb.Command_USER_EVENT_PROTOCMD:
		return pb.EventParam(id2).String()
	case pb.Command_SCENE_USER_MANUAL_PROTOCMD:
		return pb.ManualParam(id2).String()
	case pb.Command_SCENE_USER_ACHIEVE_PROTOCMD:
		return pb.AchieveParam(id2).String()
	case pb.Command_SCENE_USER_PHOTO_PROTOCMD:
		return pb.PhotoParam(id2).String()
	case pb.Command_SCENE_USER_TUTOR_PROTOCMD:
		return pb.TutorParam(id2).String()
	case pb.Command_SCENE_USER_BEING_PROTOCMD:
		return pb.BeingParam(id2).String()
	case pb.Command_SESSION_USER_GUILD_PROTOCMD:
		return pb.GuildParam(id2).String()
	case pb.Command_SESSION_USER_TEAM_PROTOCMD:
		return pb.TeamParam(id2).String()
	case pb.Command_SESSION_USER_SHOP_PROTOCMD:
		return pb.ShopParam(id2).String()
	case pb.Command_SESSION_USER_SOCIALITY_PROTOCMD:
		return pb.SocialityParam(id2).String()
	case pb.Command_RECORD_USER_TRADE_PROTOCMD:
		return pb.RecordUserTradeParam(id2).String()
	case pb.Command_MATCHC_PROTOCMD:
		return pb.MatchCParam(id2).String()
	case pb.Command_AUCTIONC_PROTOCMD:
		return pb.AuctionCParam(id2).String()
	case pb.Command_PVE_CARD_PROTOCMD:
		return pb.EPveCardParam(id2).String()
	case pb.Command_TEAM_RAID_PROTOCMD:
		return pb.TeamRaidParam(id2).String()
	case pb.Command_TEAM_GROUP_RAID_PROTOCMD:
		return pb.TeamGroupaidParam(id2).String()
	case pb.Command_ROGUELIKE_PROTOCMD:
		return pb.RoguelikeParam(id2).String()
	case pb.Command_NOVICE_BATTLE_PASS_PROTOCMD:
		return pb.NoviceBPParam(id2).String()
	case pb.Command_TECHTREE_PROTOCMD:
		return pb.TechTreeParam(id2).String()
	case pb.Command_RAID_PROTOCMD:
		return pb.RaidParam(id2).String()
	case pb.Command_ACTIVITY_PROTOCMD:
		return pb.ActivityParam(id2).String()
	case pb.Command_ACTIVITY_EVENT_PROTOCMD:
		return pb.ActivityEventParam(id2).String()
	case pb.Command_SESSION_OVERSEAS_TW_PROTOCMD:
		return pb.OverseasTaiwanParam(id2).String()
	case pb.Command_SCENE_OVERSEAS_PROTOCMD:
		return pb.OverseasSceneParam(id2).String()
	case pb.Command_FAMILY_PROTOCMD:
		return pb.FamilyParam(id2).String()
	default:
		return fmt.Sprintf("param_%d", id2)
	}
}
