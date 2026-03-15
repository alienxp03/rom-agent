package client

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	pb "github.com/alienxp03/rom-agent/internal/proto/pb"
	"github.com/alienxp03/rom-agent/internal/resources"
	"google.golang.org/protobuf/proto"
)

const (
	commandRetryLimit = 4
	recvTimeout       = 20 * time.Second
	partyInviteTTL    = 10 * time.Second
)

// ExchangeItemRecord represents a single exchange item listing
type ExchangeItemRecord struct {
	BaseInfo    *pb.TradeItemBaseInfo
	SellInfo    *pb.ItemSellInfoRecordTradeCmd
	BuyerCount  int32
	ItemData    *pb.ItemData
	SellerName  string
	SellerLevel int32
}

// GameClient handles the game protocol and state
type GameClient struct {
	// Configuration
	blueberry     int
	version       int
	lang          int
	appPreVersion int
	smDeviceId    string
	linegroup     int

	// Connection
	conn *Connection

	// State
	mu               sync.RWMutex
	charId           int64
	charName         string
	deviceId         string
	myZoneId         int
	myZeny           int64
	mapId            int
	needsMapSync     bool
	isDead           bool
	server           string
	gameDomain       string
	serverTimeOffset time.Duration
	serverZoneIds    map[int]string

	// Map data
	mapNpcs map[int64]interface{} // Will use protobuf types later

	// Inventory
	myInventory map[string]interface{} // Will use protobuf types later

	// Party
	myPartyGuid            int64
	myPartyMembers         map[int64]string
	pendingPartyInviteGuid int64
	pendingPartyInviteTs   time.Time

	// Communication channels
	recvQueue    chan interface{}
	rawRecvQueue chan []byte

	// Context
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Heartbeat
	lastHeartbeat time.Time

	// Nonce generator
	nonceGen *NonceGen
}

// NewGameClient creates a new game client instance
func NewGameClient(blueberry, version, lang, appPreVersion int, smDeviceId string, linegroup int) *GameClient {
	ctx, cancel := context.WithCancel(context.Background())

	return &GameClient{
		blueberry:      blueberry,
		version:        version,
		lang:           lang,
		appPreVersion:  appPreVersion,
		smDeviceId:     smDeviceId,
		linegroup:      linegroup,
		conn:           NewConnection(),
		mapNpcs:        make(map[int64]interface{}),
		myInventory:    make(map[string]interface{}),
		myPartyMembers: make(map[int64]string),
		serverZoneIds:  make(map[int]string),
		recvQueue:      make(chan interface{}, 100),
		rawRecvQueue:   make(chan []byte, 100),
		ctx:            ctx,
		cancel:         cancel,
		nonceGen:       NewNonceGen(),
	}
}

// Close closes the game client and all its resources
func (gc *GameClient) Close() error {
	gc.cancel()
	gc.wg.Wait()

	close(gc.rawRecvQueue)
	close(gc.recvQueue)

	return gc.conn.Close()
}

// GetServerTime returns the current server time
func (gc *GameClient) GetServerTime() time.Time {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return time.Now().Add(gc.serverTimeOffset)
}

// IsDead returns true if the character is dead
func (gc *GameClient) IsDead() bool {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return gc.isDead
}

// SetDead sets the dead state of the character
func (gc *GameClient) SetDead(dead bool) {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	gc.isDead = dead
}

// IsInParty returns true if the character is in a party
func (gc *GameClient) IsInParty() bool {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return gc.myPartyGuid > 0
}

// GetPartyMemberCount returns the number of real players in the current party.
func (gc *GameClient) GetPartyMemberCount() int {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	if gc.myPartyGuid == 0 {
		return 0
	}
	return len(gc.myPartyMembers)
}

// HasPendingPartyInvite returns true when a fresh party invite is available.
func (gc *GameClient) HasPendingPartyInvite() bool {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	if gc.pendingPartyInviteGuid == 0 {
		return false
	}
	if time.Since(gc.pendingPartyInviteTs) <= partyInviteTTL {
		return true
	}
	gc.pendingPartyInviteGuid = 0
	gc.pendingPartyInviteTs = time.Time{}
	return false
}

// GetMapId returns the current map ID
func (gc *GameClient) GetMapId() int {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return gc.mapId
}

// SetMapId sets the current map ID
func (gc *GameClient) SetMapId(mapId int) {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	gc.mapId = mapId
}

// GetZoneIdStr returns the current zone ID as a string
func (gc *GameClient) GetZoneIdStr() string {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return fmt.Sprintf("%d", gc.myZoneId)
}

// GetServer returns the server name
func (gc *GameClient) GetServer() string {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return gc.server
}

// SetServer sets the server name
func (gc *GameClient) SetServer(server string) {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	gc.server = server
}

// ClearRecvBuffer drains the receive queue
func (gc *GameClient) ClearRecvBuffer() {
	for {
		select {
		case <-gc.recvQueue:
			// Drain
		default:
			return
		}
	}
}

// startReceiveLoop starts the goroutines for receiving and processing messages
func (gc *GameClient) startReceiveLoop() {
	// Raw receiver goroutine
	gc.wg.Add(1)
	go func() {
		defer gc.wg.Done()
		for {
			select {
			case <-gc.ctx.Done():
				return
			default:
				data, err := gc.conn.Recv()
				if err != nil {
					// Log error (will add logging later)
					time.Sleep(100 * time.Millisecond)
					continue
				}
				if data != nil {
					gc.rawRecvQueue <- data
				}
			}
		}
	}()

	// Message processor goroutine
	gc.wg.Add(1)
	go func() {
		defer gc.wg.Done()
		buffer := make([]byte, 0, 65536)
		for {
			select {
			case <-gc.ctx.Done():
				return
			case rawData, ok := <-gc.rawRecvQueue:
				if !ok {
					return
				}
				buffer = append(buffer, rawData...)
				frames, remainder, err := ParseGameStream(buffer)
				if err != nil {
					prefixLen := len(buffer)
					if prefixLen > 32 {
						prefixLen = 32
					}
					slog.Warn("Failed to parse game stream",
						"error", err,
						"buffer_len", len(buffer),
						"prefix_hex", hex.EncodeToString(buffer[:prefixLen]))
					buffer = buffer[:0]
					continue
				}
				buffer = remainder
				for _, frame := range frames {
					msg := gc.parseMessage(frame)
					if msg == nil {
						continue
					}
					logAttrs := []any{"id1", frame.ID1, "id2", frame.ID2, "type", fmt.Sprintf("%T", msg)}
					if pm, ok := msg.(interface{ String() string }); ok {
						logAttrs = append(logAttrs, "body", pm.String())
					}
					slog.Debug("Received message", logAttrs...)
					gc.handleAsyncMessage(msg)
					select {
					case gc.recvQueue <- msg:
					case <-gc.ctx.Done():
						return
					}
				}
			}
		}
	}()
}

// parseMessage parses raw data into a protobuf message
func (gc *GameClient) parseMessage(frame GameFrame) interface{} {
	if factory, ok := inboundMessageRegistry[messageKey{id1: frame.ID1, id2: frame.ID2}]; ok {
		msg := factory()
		if parseIncomingPayload(frame.Payload, msg) {
			return msg
		}

		prefixLen := len(frame.Payload)
		if prefixLen > 24 {
			prefixLen = 24
		}
		slog.Warn("Failed to decode registered payload",
			"id1", frame.ID1,
			"id2", frame.ID2,
			"type", fmt.Sprintf("%T", msg),
			"len", len(frame.Payload),
			"prefix_hex", hex.EncodeToString(frame.Payload[:prefixLen]))
	}

	if msg := decodeLegacyPacket(frame); msg != nil {
		return msg
	}

	return &UnknownMessage{
		ID1:     frame.ID1,
		ID2:     frame.ID2,
		Payload: append([]byte(nil), frame.Payload...),
		Fields:  parseUnknownFields(frame.Payload),
	}
}

func (gc *GameClient) handleAsyncMessage(msg interface{}) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	switch m := msg.(type) {
	case *pb.UserSyncCmd:
		for _, data := range m.GetDatas() {
			switch data.GetType() {
			case pb.EUserDataType_EUSERDATATYPE_NAME:
				gc.charName = cleanCharName(string(data.GetData()))
			case pb.EUserDataType_EUSERDATATYPE_REAL_ZONEID:
				gc.myZoneId = int(data.GetValue())
			case pb.EUserDataType_EUSERDATATYPE_SILVER:
				gc.myZeny = int64(data.GetValue())
			case pb.EUserDataType_EUSERDATATYPE_STATUS:
				gc.isDead = pb.ECreatureStatus(data.GetValue()) == pb.ECreatureStatus_ECREATURESTATUS_DEAD
			}
		}
	case *pb.ChangeSceneUserCmd:
		gc.mapId = int(m.GetMapID())
		gc.needsMapSync = true
		gc.mapNpcs = make(map[int64]interface{})
	case *pb.ServerTimeUserCmd:
		if ts := m.GetTime(); ts > 0 {
			gc.serverTimeOffset = time.UnixMilli(int64(ts)).Sub(time.Now())
		}
	case *pb.AddMapNpc:
		for _, npc := range m.GetNpcs() {
			gc.mapNpcs[int64(npc.GetId())] = npc
		}
	case *pb.AddMapObjNpc:
		for _, npc := range m.GetNpcs() {
			gc.mapNpcs[int64(npc.GetId())] = npc
		}
	case *pb.DeleteStaticEntryUserCmd:
		for _, guid := range m.GetList() {
			deleteByGUID(gc.mapNpcs, guid)
		}
	case *pb.InviteMember:
		if guid := int64(m.GetUserguid()); guid > 0 {
			gc.pendingPartyInviteGuid = guid
			gc.pendingPartyInviteTs = time.Now()
		}
	case *pb.EnterTeam:
		data := m.GetData()
		if data == nil || data.GetGuid() == 0 {
			break
		}
		gc.myPartyGuid = int64(data.GetGuid())
		gc.myPartyMembers = make(map[int64]string)
		for _, member := range data.GetMembers() {
			if member.GetAccid() == 0 {
				continue
			}
			name := cleanCharName(member.GetName())
			if name == "" {
				continue
			}
			gc.myPartyMembers[int64(member.GetGuid())] = name
		}
	case *pb.TeamMemberUpdate:
		for _, member := range m.GetUpdates() {
			if member.GetAccid() == 0 {
				continue
			}
			name := cleanCharName(member.GetName())
			if name == "" {
				continue
			}
			gc.myPartyMembers[int64(member.GetGuid())] = name
		}
		for _, guid := range m.GetDeletes() {
			delete(gc.myPartyMembers, int64(guid))
		}
	case *pb.ExitTeam:
		gc.myPartyGuid = 0
		gc.myPartyMembers = make(map[int64]string)
	case *pb.JoinRoomCCmd:
		if teamID := int64(m.GetTeamid()); teamID > 0 {
			gc.myPartyGuid = teamID
		}
	case *pb.EnterGuildGuildCmd:
		// Keep zone information fresh from the guild roster for the logged-in character.
		for _, member := range m.GetData().GetMembers() {
			if int64(member.GetCharid()) != gc.charId {
				continue
			}
			if zoneID := int(member.GetZoneid() % 10000); zoneID > 0 {
				gc.myZoneId = zoneID
			}
			break
		}
	}
}

// recv receives a message from the queue with timeout
func (gc *GameClient) recv(timeout time.Duration) interface{} {
	select {
	case msg := <-gc.recvQueue:
		return msg
	case <-time.After(timeout):
		return nil
	case <-gc.ctx.Done():
		return nil
	}
}

// send sends data to the game server
func (gc *GameClient) send(data []byte) error {
	if err := gc.conn.Send(data); err != nil {
		return err
	}

	gc.mu.Lock()
	gc.lastHeartbeat = time.Now()
	gc.mu.Unlock()

	return nil
}

// startHeartbeat starts the heartbeat ticker
func (gc *GameClient) startHeartbeat() {
	ticker := time.NewTicker(10 * time.Second)

	gc.wg.Add(1)
	go func() {
		defer gc.wg.Done()
		defer ticker.Stop()

		for {
			select {
			case <-gc.ctx.Done():
				return
			case <-ticker.C:
				if err := gc.sendHeartbeat(); err != nil {
					// Log error (will add logging later)
				}
			}
		}
	}()
}

// sendHeartbeat sends a heartbeat message to keep the connection alive
func (gc *GameClient) sendHeartbeat() error {
	ts := uint64(gc.nonceGen.GetServerTime())
	return gc.sendMessage(context.Background(), &pb.HeartBeatUserCmd{Time: &ts})
}

// sendGameMessage sends a game protocol message
func (gc *GameClient) sendGameMessage(ctx context.Context, cmd pb.Command, param int32, msg proto.Message) error {
	// Serialize to protobuf
	var data []byte
	var err error
	if msg != nil {
		data, err = proto.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to serialize: %w", err)
		}
	}

	// Get command and param numbers
	cmdNum := int32(cmd)

	// Update nonce
	gc.nonceGen.Update()
	nonce := gc.nonceGen.GetNonce()
	nonceBytes, err := proto.Marshal(nonce)
	if err != nil {
		return fmt.Errorf("failed to serialize nonce: %w", err)
	}

	// Encode message
	message := GameSend(byte(cmdNum), byte(param), nonceBytes, data)

	// Send message
	return gc.send(message)
}

// LoginWithAuth performs the login sequence using authenticated HTTP auth data.
func (gc *GameClient) LoginWithAuth(ctx context.Context, authData *AuthData, deviceID, character string) error {
	// Connect to game server
	serverAddr := fmt.Sprintf("%s:%d", authData.GameServerIP, authData.GameServerPort)
	slog.Info("Connecting to game server", "addr", serverAddr)
	if err := gc.conn.Connect(ctx, serverAddr); err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	gc.deviceId = deviceID
	gc.gameDomain = authData.GameServerHost
	gc.SetServer(gameServerName(authData.GameServerHost, gc.linegroup))

	// Start receive loop
	gc.startReceiveLoop()

	// Step 1: Request server time (initial)
	slog.Info("Sending initial ServerTimeUserCmd")
	if err := gc.sendGameMessage(ctx, pb.Command_LOGIN_USER_PROTOCMD, int32(pb.LoginCmdParam_SERVERTIME_USER_CMD), &pb.ServerTimeUserCmd{}); err != nil {
		return fmt.Errorf("failed to send initial server time request: %w", err)
	}

	// Step 2: Send login request
	slog.Info("Sending ReqLoginUserCmd")
	accid := uint64(authData.AccID)
	sha1Str := authData.SHA1
	timestamp := uint32(authData.Timestamp)
	ip := authData.GameServerIP
	domain := authData.GameServerHost
	zoneid := uint32(authData.ServerID)
	version := authData.GameServerVersion
	serverid := uint32(gc.linegroup)
	langU32 := uint32(gc.lang)
	device := "android"
	emptyStr := ""
	authorize := ""
	safeDevice := ""
	zeroU32 := uint32(0)
	clientversion := uint32(gc.appPreVersion)
	fingerprint := gc.smDeviceId

	reqLogin := &pb.ReqLoginUserCmd{
		Accid:         &accid,
		Sha1:          &sha1Str,
		Timestamp:     &timestamp,
		Ip:            &ip,
		Domain:        &domain,
		Zoneid:        &zoneid,
		Language:      &langU32,
		Device:        &device,
		Phone:         &emptyStr,
		Site:          &zeroU32,
		Version:       &version,
		Authorize:     &authorize,
		SafeDevice:    &safeDevice,
		Serverid:      &serverid,
		Deviceid:      &deviceID,
		Clientversion: &clientversion,
		Langzone:      &langU32,
		Fingerprint:   &fingerprint,
	}
	slog.Info("ReqLoginUserCmd payload",
		"accid", reqLogin.GetAccid(),
		"zoneid", reqLogin.GetZoneid(),
		"serverid", reqLogin.GetServerid(),
		"timestamp", reqLogin.GetTimestamp(),
		"version", reqLogin.GetVersion(),
		"domain", reqLogin.GetDomain(),
		"ip", reqLogin.GetIp(),
		"language", reqLogin.GetLanguage(),
		"langzone", reqLogin.GetLangzone(),
		"device", reqLogin.GetDevice(),
		"deviceid_present", reqLogin.GetDeviceid() != "",
		"safe_device_present", reqLogin.GetSafeDevice() != "",
		"authorize_present", reqLogin.GetAuthorize() != "",
		"clientversion", reqLogin.GetClientversion(),
		"fingerprint_present", reqLogin.GetFingerprint() != "",
		"sha1_prefix", shortSHA(reqLogin.GetSha1()))

	if err := gc.sendGameMessage(ctx, pb.Command_LOGIN_USER_PROTOCMD, int32(pb.LoginCmdParam_REQ_LOGIN_USER_CMD), reqLogin); err != nil {
		return fmt.Errorf("failed to send login request: %w", err)
	}

	// Step 3: Send client info
	slog.Info("Sending ClientInfoUserCmd")
	delay := gc.conn.Delay()
	clientInfo := &pb.ClientInfoUserCmd{
		Delay: &delay,
	}
	slog.Info("ClientInfoUserCmd payload", "body", clientInfo.String())
	if err := gc.sendGameMessage(ctx, pb.Command_LOGIN_USER_PROTOCMD, int32(pb.LoginCmdParam_CLIENT_INFO_USER_CMD), clientInfo); err != nil {
		return fmt.Errorf("failed to send client info: %w", err)
	}
	gc.startHeartbeat()

	// Step 4: Wait for snapshot (character list)
	slog.Info("Waiting for SnapShotUserCmd")
	var snapshot *pb.SnapShotUserCmd
	timeout := time.After(30 * time.Second)
	for snapshot == nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for character snapshot")
		default:
		}

		obj := gc.recv(1 * time.Second)
		if snap, ok := obj.(*pb.SnapShotUserCmd); ok {
			snapshot = snap
			break
		}
	}

	// Extract charId
	slog.Info("Received SnapShotUserCmd", "characters", len(snapshot.Data))
	var charId uint64
	if character != "" {
		// Find character by name
		for _, charData := range snapshot.Data {
			if charData.Name != nil && *charData.Name == character {
				if charData.Id != nil {
					charId = *charData.Id
					break
				}
			}
		}
		if charId == 0 {
			return fmt.Errorf("character '%s' not found", character)
		}
	} else {
		// Use default character
		if snapshot.Maincharid == nil {
			return fmt.Errorf("no default character")
		}
		charId = *snapshot.Maincharid
	}

	gc.mu.Lock()
	gc.charId = int64(charId)
	gc.mu.Unlock()

	// Step 5: Request server time again (for sync)
	slog.Info("Sending second ServerTimeUserCmd")
	if err := gc.sendGameMessage(ctx, pb.Command_LOGIN_USER_PROTOCMD, int32(pb.LoginCmdParam_SERVERTIME_USER_CMD), &pb.ServerTimeUserCmd{}); err != nil {
		return fmt.Errorf("failed to send second server time request: %w", err)
	}

	// Step 6: Wait for server time response
	slog.Info("Waiting for synced ServerTimeUserCmd")
	timeout = time.After(10 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for server time")
		default:
		}

		obj := gc.recv(1 * time.Second)
		if serverTime, ok := obj.(*pb.ServerTimeUserCmd); ok {
			if serverTime.GetTime() > 0 {
				gc.nonceGen.SetServerTime(int64(serverTime.GetTime()))
				break
			}
		}
	}

	// Step 7: Send set options
	slog.Info("Sending SetOptionUserCmd")
	optType := pb.EQueryType_EQUERYTYPE_ALL
	fashionhide := uint32(0)
	weddingType := pb.EQueryType_EQUERYTYPE_WEDDING_ALL

	setOptions := &pb.SetOptionUserCmd{
		Type:        &optType,
		Fashionhide: &fashionhide,
		WeddingType: &weddingType,
	}
	if err := gc.sendGameMessage(ctx, pb.Command_SCENE_USER2_PROTOCMD, int32(pb.User2Param_USER2PARAM_SETOPTION), setOptions); err != nil {
		return fmt.Errorf("failed to send set options: %w", err)
	}

	// Step 8: Select character
	slog.Info("Sending SelectRoleUserCmd", "char_id", charId)
	clickpos := uint32(16570100 + (time.Now().UnixNano() % 100))
	langU32Ptr := uint32(gc.lang)
	system := "Android"
	model := "OnePlus ONEPLUS A5000"
	systemVersion := "Android OS 7.1.1 / API-25 (NMF26X/327)"

	selectRole := &pb.SelectRoleUserCmd{
		Id:       &charId,
		Clickpos: &clickpos,
		Deviceid: &deviceID,
		Language: &langU32Ptr,
		ExtraData: &pb.ExtraData{
			System:  &system,
			Model:   &model,
			Version: &systemVersion,
		},
	}
	if err := gc.sendGameMessage(ctx, pb.Command_LOGIN_USER_PROTOCMD, int32(pb.LoginCmdParam_SELECT_ROLE_USER_CMD), selectRole); err != nil {
		return fmt.Errorf("failed to send select role: %w", err)
	}

	// Step 9: Wait for login result
	slog.Info("Waiting for LoginResultUserCmd")
	timeout = time.After(30 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for login result")
		default:
		}

		obj := gc.recv(1 * time.Second)
		if result, ok := obj.(*pb.LoginResultUserCmd); ok {
			if result.Ret != nil && *result.Ret == 0 {
				// Login successful
				return nil
			}
			return fmt.Errorf("login failed with code: %d", *result.Ret)
		}
		if regErr, ok := obj.(*pb.RegErrUserCmd); ok {
			return fmt.Errorf("login error: %v", regErr)
		}
	}
}

// PostLogin performs post-login initialization
func (gc *GameClient) PostLogin(ctx context.Context) error {
	time.Sleep(2200 * time.Millisecond)

	var serverInfo *pb.ServerInfoNtf
	var userSync *pb.UserSyncCmd
	timeout := time.After(10 * time.Second)
	for serverInfo == nil || userSync == nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for initial post-login sync")
		default:
		}

		obj := gc.recv(1 * time.Second)
		switch msg := obj.(type) {
		case *pb.ServerInfoNtf:
			serverInfo = msg
		case *pb.UserSyncCmd:
			userSync = msg
		}
	}

	gc.applyServerInfo(serverInfo)

	for _, packType := range []pb.EPackType{
		pb.EPackType_EPACKTYPE_EQUIP,
		pb.EPackType_EPACKTYPE_FASHION,
		pb.EPackType_EPACKTYPE_MAIN,
		pb.EPackType_EPACKTYPE_STORE,
		pb.EPackType_EPACKTYPE_BARROW,
		pb.EPackType_EPACKTYPE_TEMP_MAIN,
		pb.EPackType_EPACKTYPE_QUEST,
		pb.EPackType_EPACKTYPE_FOOD,
		pb.EPackType_EPACKTYPE_PET,
	} {
		packType := packType
		if err := gc.sendMessage(ctx, &pb.PackageItem{Type: &packType}); err != nil {
			return fmt.Errorf("send package item: %w", err)
		}
	}

	for _, manualType := range []pb.EManualType{
		pb.EManualType_EMANUALTYPE_FASHION,
		pb.EManualType_EMANUALTYPE_CARD,
		pb.EManualType_EMANUALTYPE_EQUIP,
		pb.EManualType_EMANUALTYPE_ITEM,
		pb.EManualType_EMANUALTYPE_MOUNT,
		pb.EManualType_EMANUALTYPE_MONSTER,
		pb.EManualType_EMANUALTYPE_PET,
		pb.EManualType_EMANUALTYPE_NPC,
		pb.EManualType_EMANUALTYPE_MAP,
		pb.EManualType_EMANUALTYPE_SCENERY,
		pb.EManualType_EMANUALTYPE_COLLECTION,
	} {
		manualType := manualType
		if err := gc.sendMessage(ctx, &pb.QueryManualData{Type: &manualType}); err != nil {
			return fmt.Errorf("send query manual data: %w", err)
		}
	}

	if err := gc.sendMessage(ctx, &pb.QueryShopGotItem{}); err != nil {
		return fmt.Errorf("send query shop got item: %w", err)
	}
	if err := gc.sendMessage(ctx, &pb.QueryShortcut{}); err != nil {
		return fmt.Errorf("send query shortcut: %w", err)
	}

	var changeScene *pb.ChangeSceneUserCmd
	timeout = time.After(20 * time.Second)
	for changeScene == nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for ChangeSceneUserCmd")
		default:
		}

		obj := gc.recv(1 * time.Second)
		if msg, ok := obj.(*pb.ChangeSceneUserCmd); ok {
			changeScene = msg
		}
	}

	frame := uint32(500 + (time.Now().UnixNano() % 400))
	if err := gc.sendMessage(ctx, &pb.ClientFrameUserCmd{Frame: &frame}); err != nil {
		return fmt.Errorf("send client frame: %w", err)
	}

	time.Sleep(5 * time.Second)

	mapID := changeScene.GetMapID()
	emptyMap := ""
	zero := int32(0)
	if err := gc.sendMessage(ctx, &pb.ChangeSceneUserCmd{
		MapID:   &mapID,
		MapName: &emptyMap,
		Pos:     &pb.ScenePos{X: &zero, Y: &zero, Z: &zero},
	}); err != nil {
		return fmt.Errorf("send change scene: %w", err)
	}

	gc.mu.Lock()
	gc.needsMapSync = false
	gc.mu.Unlock()

	time.Sleep(2 * time.Second)

	if err := gc.sendMessage(ctx, &pb.GetVoiceIDChatCmd{}); err != nil {
		return fmt.Errorf("send get voice id: %w", err)
	}
	if err := gc.sendMessage(ctx, &pb.QueryChargeCnt{}); err != nil {
		return fmt.Errorf("send query charge cnt: %w", err)
	}

	return nil
}

// GetMatchingZones returns zones matching the given pattern
func (gc *GameClient) GetMatchingZones(pattern string) ([]string, error) {
	gc.mu.RLock()
	defer gc.mu.RUnlock()

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	zones := make([]string, 0)
	for _, zone := range gc.serverZoneIds {
		if zone != "" && re.MatchString(zone) {
			zones = append(zones, zone)
		}
	}
	return zones, nil
}

// GetCombatTime fetches the combat time information
func (gc *GameClient) GetCombatTime(ctx context.Context) error {
	if err := gc.sendMessage(ctx, &pb.BattleTimelenUserCmd{}); err != nil {
		return fmt.Errorf("send battle time: %w", err)
	}

	timeout := time.After(recvTimeout)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for BattleTimelenUserCmd")
		default:
		}

		if _, ok := gc.recv(time.Second).(*pb.BattleTimelenUserCmd); ok {
			return nil
		}
	}
}

// LeaveParty leaves the current party
func (gc *GameClient) LeaveParty(ctx context.Context) error {
	gc.mu.RLock()
	teamID := uint64(gc.myPartyGuid)
	gc.mu.RUnlock()
	if teamID == 0 {
		return nil
	}

	if err := gc.sendMessage(ctx, &pb.ExitTeam{Teamid: &teamID}); err != nil {
		return fmt.Errorf("send exit team: %w", err)
	}

	gc.mu.Lock()
	gc.myPartyGuid = 0
	gc.myPartyMembers = make(map[int64]string)
	gc.mu.Unlock()

	time.Sleep(900 * time.Millisecond)
	return nil
}

// AcceptPartyInvite accepts a pending party invitation when one is still valid.
func (gc *GameClient) AcceptPartyInvite(ctx context.Context) error {
	gc.mu.Lock()
	inviteGuid := gc.pendingPartyInviteGuid
	inviteTs := gc.pendingPartyInviteTs
	if inviteGuid == 0 {
		gc.mu.Unlock()
		return nil
	}
	if time.Since(inviteTs) > partyInviteTTL {
		gc.pendingPartyInviteGuid = 0
		gc.pendingPartyInviteTs = time.Time{}
		gc.mu.Unlock()
		return nil
	}
	gc.pendingPartyInviteGuid = 0
	gc.pendingPartyInviteTs = time.Time{}
	gc.mu.Unlock()

	agree := pb.ETeamInviteType_ETEAMINVITETYPE_AGREE
	guid := uint64(inviteGuid)
	if err := gc.sendMessage(ctx, &pb.ProcessTeamInvite{
		Type:     &agree,
		Userguid: &guid,
	}); err != nil {
		return fmt.Errorf("send process team invite: %w", err)
	}

	time.Sleep(900 * time.Millisecond)
	return nil
}

// ReviveToTown revives the character in town
func (gc *GameClient) ReviveToTown(ctx context.Context) error {
	reliveType := pb.EReliveType_ERELIVETYPE_RETURN
	if err := gc.sendMessage(ctx, &pb.Relive{Type: &reliveType}); err != nil {
		return fmt.Errorf("send relive: %w", err)
	}

	timeout := time.After(recvTimeout)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for relive result")
		default:
		}

		if result, ok := gc.recv(time.Second).(*pb.LoginResultUserCmd); ok {
			if result.GetRet() != 0 {
				return fmt.Errorf("relive failed with code: %d", result.GetRet())
			}
			gc.SetDead(false)
			gc.mu.Lock()
			gc.needsMapSync = true
			gc.mu.Unlock()
			return gc.SyncMapIfNeeded(ctx)
		}
	}
}

// JumpZone jumps to the specified zone
func (gc *GameClient) JumpZone(ctx context.Context, zone string) error {
	gc.mu.RLock()
	currentZoneID := gc.myZoneId
	currentZeny := gc.myZeny
	serverZoneIDs := make(map[int]string, len(gc.serverZoneIds))
	for id, name := range gc.serverZoneIds {
		serverZoneIDs[id] = name
	}
	gc.mu.RUnlock()

	zoneID := -1
	for id, name := range serverZoneIDs {
		if name == zone {
			zoneID = id
			break
		}
	}
	if zoneID < 0 {
		return fmt.Errorf("unknown zone %q", zone)
	}
	if zoneID == currentZoneID {
		return nil
	}
	if currentZeny < 1000 {
		return fmt.Errorf("not enough zeny to jump zones: have %d need at least 1000", currentZeny)
	}

	if err := gc.sendMessage(ctx, &pb.QueryZoneStatusUserCmd{}); err != nil {
		return fmt.Errorf("send query zone status: %w", err)
	}

	time.Sleep(1500 * time.Millisecond)

	zoneID32 := uint32(zoneID)
	isAnywhere := true
	if err := gc.sendMessage(ctx, &pb.JumpZoneUserCmd{
		Zoneid:     &zoneID32,
		Isanywhere: &isAnywhere,
	}); err != nil {
		return fmt.Errorf("send jump zone: %w", err)
	}

	gc.mu.Lock()
	gc.myZeny = maxInt64(0, gc.myZeny-1000)
	gc.needsMapSync = true
	gc.mu.Unlock()

	return gc.SyncMapIfNeeded(ctx)
}

// SyncMapIfNeeded synchronizes the map if needed
func (gc *GameClient) SyncMapIfNeeded(ctx context.Context) error {
	gc.mu.RLock()
	needsSync := gc.needsMapSync
	mapID := uint32(gc.mapId)
	gc.mu.RUnlock()

	if !needsSync {
		return nil
	}

	if err := gc.sendMessage(ctx, &pb.ServerTimeUserCmd{}); err != nil {
		return fmt.Errorf("send server time for map sync: %w", err)
	}

	delay := gc.conn.Delay()
	if err := gc.sendMessage(ctx, &pb.ClientInfoUserCmd{Delay: &delay}); err != nil {
		return fmt.Errorf("send client info for map sync: %w", err)
	}

	time.Sleep(2200 * time.Millisecond)

	emptyMap := ""
	zero := int32(0)
	if err := gc.sendMessage(ctx, &pb.ChangeSceneUserCmd{
		MapID:   &mapID,
		MapName: &emptyMap,
		Pos:     &pb.ScenePos{X: &zero, Y: &zero, Z: &zero},
	}); err != nil {
		return fmt.Errorf("send map sync change scene: %w", err)
	}

	gc.mu.Lock()
	gc.needsMapSync = false
	gc.mu.Unlock()

	return nil
}

// ExitExchange closes the exchange window
func (gc *GameClient) ExitExchange(ctx context.Context) error {
	charID := uint64(gc.charId)
	oper := pb.EPanelOperType_EPANEL_CLOSE
	return gc.sendMessage(ctx, &pb.PanelRecordTrade{
		Charid: &charID,
		Oper:   &oper,
	})
}

// OpenExchange opens the exchange window
func (gc *GameClient) OpenExchange(ctx context.Context) error {
	charID := uint64(gc.charId)
	if err := gc.sendMessage(ctx, &pb.MyPendingListRecordTradeCmd{Charid: &charID}); err != nil {
		return fmt.Errorf("send my pending trade list: %w", err)
	}

	time.Sleep(100 * time.Millisecond)

	index := uint32(0)
	if err := gc.sendMessage(ctx, &pb.MyTradeLogRecordTradeCmd{
		Charid: &charID,
		Index:  &index,
	}); err != nil {
		return fmt.Errorf("send my trade log: %w", err)
	}

	oper := pb.EPanelOperType_EPANEL_OPEN
	if err := gc.sendMessage(ctx, &pb.PanelRecordTrade{
		Charid: &charID,
		Oper:   &oper,
	}); err != nil {
		return fmt.Errorf("send exchange panel open: %w", err)
	}

	return nil
}

// QueryExchangeCategory queries the exchange for a category and returns the response
func (gc *GameClient) QueryExchangeCategory(ctx context.Context, category resources.ExchangeCategory) (*pb.BriefPendingListRecordTradeCmd, error) {
	if gc.conn == nil {
		return nil, fmt.Errorf("connection is nil")
	}

	retries := commandRetryLimit
	for retries > 0 {
		retries--

		// Build the request message
		categoryID := uint32(category.ID)
		job := uint32(0)
		charIdUint := uint64(gc.charId)

		briefCmd := &pb.BriefPendingListRecordTradeCmd{
			Charid:   &charIdUint,
			Category: &categoryID,
			Job:      &job,
		}

		// Serialize to protobuf
		data, err := SerializeProtobuf(briefCmd)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize: %w", err)
		}

		// Get command and param numbers
		cmdNum := int32(pb.Command_RECORD_USER_TRADE_PROTOCMD)
		paramNum := int32(pb.RecordUserTradeParam_BRIEF_PENDING_LIST_RECORDTRADE)

		// Update nonce
		gc.nonceGen.Update()
		nonce := gc.nonceGen.GetNonce()
		nonceBytes, err := SerializeProtobuf(nonce)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize nonce: %w", err)
		}

		// Encode message
		message := GameSend(byte(cmdNum), byte(paramNum), nonceBytes, data)

		// Send message
		if err := gc.send(message); err != nil {
			return nil, fmt.Errorf("failed to send: %w", err)
		}

		// Wait for response
		initialTs := time.Now()
		for {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			if time.Since(initialTs) > recvTimeout {
				break // Timeout, retry
			}

			obj := gc.recv(1 * time.Second)
			if obj == nil {
				continue
			}

			// Check if it's the response we're looking for
			if resp, ok := obj.(*pb.BriefPendingListRecordTradeCmd); ok {
				// Verify it's for the correct category
				if resp.Category != nil && *resp.Category == uint32(category.ID) {
					return resp, nil
				}
				// Category mismatch, retry
				break
			}
		}
	}

	return nil, fmt.Errorf("did not receive BriefPendingListRecordTradeCmd after %d retries", commandRetryLimit)
}

func mergeExchangeItemIDs(resp *pb.BriefPendingListRecordTradeCmd) []int {
	if resp == nil {
		return nil
	}

	merged := make([]int, 0, len(resp.GetPubLists())+len(resp.GetLists()))
	for _, itemID := range resp.GetPubLists() {
		merged = append(merged, int(itemID))
	}
	for _, itemID := range resp.GetLists() {
		merged = append(merged, int(itemID))
	}
	return merged
}

func (gc *GameClient) queryExchangeSellInfo(
	ctx context.Context,
	itemID uint32,
	orderID uint64,
	publicityID uint32,
) (*pb.ItemSellInfoRecordTradeCmd, error) {
	retries := commandRetryLimit

	for retries > 0 {
		retries--

		charIDUint := uint64(gc.charId)
		tradeType := pb.ETradeType_ETRADETYPE_TRADE
		itemSellInfoCmd := &pb.ItemSellInfoRecordTradeCmd{
			Charid:      &charIDUint,
			Type:        &tradeType,
			Itemid:      &itemID,
			OrderId:     &orderID,
			PublicityId: &publicityID,
		}

		data, err := SerializeProtobuf(itemSellInfoCmd)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize item sell info request: %w", err)
		}

		cmdNum := int32(pb.Command_RECORD_USER_TRADE_PROTOCMD)
		paramNum := int32(pb.RecordUserTradeParam_ITEM_SELL_INFO_RECORDTRADE)

		gc.nonceGen.Update()
		nonce := gc.nonceGen.GetNonce()
		nonceBytes, err := SerializeProtobuf(nonce)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize nonce: %w", err)
		}

		message := GameSend(byte(cmdNum), byte(paramNum), nonceBytes, data)
		if err := gc.send(message); err != nil {
			return nil, fmt.Errorf("failed to send item sell info request: %w", err)
		}

		initialTs := time.Now()
		for {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			if time.Since(initialTs) > recvTimeout {
				break
			}

			obj := gc.recv(1 * time.Second)
			if obj == nil {
				continue
			}

			resp, ok := obj.(*pb.ItemSellInfoRecordTradeCmd)
			if !ok {
				continue
			}
			if resp.GetItemid() != itemID {
				break
			}
			if orderID != 0 && resp.GetOrderId() != 0 && resp.GetOrderId() != orderID {
				break
			}
			if publicityID != 0 && resp.GetPublicityId() != 0 && resp.GetPublicityId() != publicityID {
				break
			}
			return resp, nil
		}
	}

	return nil, fmt.Errorf("did not receive ItemSellInfoRecordTradeCmd after %d retries", commandRetryLimit)
}

// QueryExchangeItem queries the exchange for a specific item and returns listings
func (gc *GameClient) QueryExchangeItem(ctx context.Context, itemID int, category resources.ExchangeCategory) ([]*ExchangeItemRecord, error) {
	if gc.conn == nil {
		return nil, fmt.Errorf("connection is nil")
	}

	retries := commandRetryLimit
	var detailResp *pb.DetailPendingListRecordTradeCmd

	for retries > 0 {
		retries--

		// Build the request message
		itemIDUint := uint32(itemID)
		pageIndex := uint32(0)
		tradeType := pb.ETradeType_ETRADETYPE_ALL
		rankType := pb.RankType_RANKTYPE_PENDING_TIME_DES
		charIdUint := uint64(gc.charId)

		searchCond := &pb.SearchCond{
			ItemId:    &itemIDUint,
			PageIndex: &pageIndex,
			TradeType: &tradeType,
			RankType:  &rankType,
		}

		detailCmd := &pb.DetailPendingListRecordTradeCmd{
			Charid:     &charIdUint,
			SearchCond: searchCond,
		}

		// Serialize to protobuf
		data, err := SerializeProtobuf(detailCmd)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize: %w", err)
		}

		// Get command and param numbers
		cmdNum := int32(pb.Command_RECORD_USER_TRADE_PROTOCMD)
		paramNum := int32(pb.RecordUserTradeParam_DETAIL_PENDING_LIST_RECORDTRADE)

		// Update nonce
		gc.nonceGen.Update()
		nonce := gc.nonceGen.GetNonce()
		nonceBytes, err := SerializeProtobuf(nonce)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize nonce: %w", err)
		}

		// Encode message
		message := GameSend(byte(cmdNum), byte(paramNum), nonceBytes, data)

		// Send message
		if err := gc.send(message); err != nil {
			return nil, fmt.Errorf("failed to send: %w", err)
		}

		// Wait for response
		initialTs := time.Now()
		for {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			if time.Since(initialTs) > recvTimeout {
				break // Timeout, retry
			}

			obj := gc.recv(1 * time.Second)
			if obj == nil {
				continue
			}

			// Check if it's the response we're looking for
			if resp, ok := obj.(*pb.DetailPendingListRecordTradeCmd); ok {
				// Verify it's for the correct item
				if resp.SearchCond != nil && resp.SearchCond.ItemId != nil &&
					*resp.SearchCond.ItemId == uint32(itemID) {
					detailResp = resp
					break
				}
				// Item mismatch, retry
				break
			}
		}

		if detailResp != nil {
			break
		}
	}

	if detailResp == nil {
		return nil, fmt.Errorf("did not receive DetailPendingListRecordTradeCmd after %d retries", commandRetryLimit)
	}

	// Build result records from the response
	results := make([]*ExchangeItemRecord, 0, len(detailResp.Lists))
	for _, baseInfo := range detailResp.Lists {
		record := &ExchangeItemRecord{
			BaseInfo:   baseInfo,
			ItemData:   baseInfo.GetItemData(),
			SellerName: baseInfo.GetName(),
		}

		sellInfo, err := gc.queryExchangeSellInfo(
			ctx,
			baseInfo.GetItemid(),
			baseInfo.GetOrderId(),
			baseInfo.GetPublicityId(),
		)
		if err != nil {
			slog.Warn("Missing exchange sell info; using base record only",
				"category_id", category.ID,
				"item_id", baseInfo.GetItemid(),
				"order_id", baseInfo.GetOrderId(),
				"publicity_id", baseInfo.GetPublicityId(),
				"error", err)
		} else {
			record.SellInfo = sellInfo
			record.BuyerCount = int32(sellInfo.GetBuyerCount())
		}

		results = append(results, record)
	}

	return results, nil
}

// GetNpcGuid returns the GUID of an NPC by ID
func (gc *GameClient) GetNpcGuid(npcId int64) int64 {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	for guid, raw := range gc.mapNpcs {
		npc, ok := raw.(*pb.MapNpc)
		if !ok {
			continue
		}
		if int64(npc.GetNpcID()) == npcId {
			return guid
		}
	}
	return 0
}

// ClickAndVisitNpc clicks on and visits an NPC
func (gc *GameClient) ClickAndVisitNpc(ctx context.Context, npcGuid int64) error {
	guid := uint64(npcGuid)
	if err := gc.sendMessage(ctx, &pb.VisitNpcUserCmd{Npctempid: &guid}); err != nil {
		return fmt.Errorf("send visit npc: %w", err)
	}
	return nil
}

func (gc *GameClient) sendMessage(ctx context.Context, msg proto.Message) error {
	switch m := msg.(type) {
	case *pb.PackageItem:
		return gc.sendGameMessage(ctx, m.GetCmd(), int32(m.GetParam()), m)
	case *pb.QueryManualData:
		return gc.sendGameMessage(ctx, m.GetCmd(), int32(m.GetParam()), m)
	case *pb.QueryShopGotItem:
		return gc.sendGameMessage(ctx, m.GetCmd(), int32(m.GetParam()), m)
	case *pb.QueryShortcut:
		return gc.sendGameMessage(ctx, m.GetCmd(), int32(m.GetParam()), m)
	case *pb.ClientFrameUserCmd:
		return gc.sendGameMessage(ctx, m.GetCmd(), int32(m.GetParam()), m)
	case *pb.ChangeSceneUserCmd:
		return gc.sendGameMessage(ctx, m.GetCmd(), int32(m.GetParam()), m)
	case *pb.GetVoiceIDChatCmd:
		return gc.sendGameMessage(ctx, m.GetCmd(), int32(m.GetParam()), m)
	case *pb.QueryChargeCnt:
		return gc.sendGameMessage(ctx, m.GetCmd(), int32(m.GetParam()), m)
	case *pb.HeartBeatUserCmd:
		return gc.sendGameMessage(ctx, m.GetCmd(), int32(m.GetParam()), m)
	case *pb.BattleTimelenUserCmd:
		return gc.sendGameMessage(ctx, m.GetCmd(), int32(m.GetParam()), m)
	case *pb.ExitTeam:
		return gc.sendGameMessage(ctx, m.GetCmd(), int32(m.GetParam()), m)
	case *pb.Relive:
		return gc.sendGameMessage(ctx, m.GetCmd(), int32(m.GetParam()), m)
	case *pb.QueryZoneStatusUserCmd:
		return gc.sendGameMessage(ctx, m.GetCmd(), int32(m.GetParam()), m)
	case *pb.JumpZoneUserCmd:
		return gc.sendGameMessage(ctx, m.GetCmd(), int32(m.GetParam()), m)
	case *pb.VisitNpcUserCmd:
		return gc.sendGameMessage(ctx, m.GetCmd(), int32(m.GetParam()), m)
	case *pb.MyPendingListRecordTradeCmd:
		return gc.sendGameMessage(ctx, m.GetCmd(), int32(m.GetParam()), m)
	case *pb.MyTradeLogRecordTradeCmd:
		return gc.sendGameMessage(ctx, m.GetCmd(), int32(m.GetParam()), m)
	case *pb.PanelRecordTrade:
		return gc.sendGameMessage(ctx, m.GetCmd(), int32(m.GetParam()), m)
	case *pb.ProcessTeamInvite:
		return gc.sendGameMessage(ctx, m.GetCmd(), int32(m.GetParam()), m)
	default:
		return fmt.Errorf("unsupported message type %T", msg)
	}
}

func (gc *GameClient) applyServerInfo(serverInfo *pb.ServerInfoNtf) {
	if serverInfo == nil || serverInfo.GetServerinfo() == nil {
		return
	}

	gc.mu.Lock()
	defer gc.mu.Unlock()

	serverZoneIDs := make(map[int]string)
	for _, info := range serverInfo.GetServerinfo().GetServerinfos() {
		if int(info.GetGroupid()) != gc.linegroup {
			continue
		}
		for _, zone := range info.GetZoneinfos() {
			serverZoneIDs[int(zone.GetZoneid()%10000)] = zone.GetName()
		}
	}
	if len(serverZoneIDs) > 0 {
		gc.serverZoneIds = serverZoneIDs
	}
}

func cleanCharName(name string) string {
	return strings.TrimRight(name, "\x02")
}

func shortSHA(sha string) string {
	if len(sha) <= 8 {
		return sha
	}
	return sha[:8]
}

func gameServerName(domain string, linegroup int) string {
	switch {
	case regexp.MustCompile(`^na-prod-gate.*\.ro\.com$`).MatchString(domain):
		return fmt.Sprintf("GL%d", linegroup)
	case regexp.MustCompile(`^sea.*-gate.*\.ro\.com$`).MatchString(domain):
		return fmt.Sprintf("SEA%d", linegroup)
	case regexp.MustCompile(`^kr-prod-gate.*\.ro\.com$`).MatchString(domain):
		return fmt.Sprintf("KR%d", linegroup)
	case regexp.MustCompile(`^eu-prod-gatex2.*\.ro\.com$`).MatchString(domain):
		return fmt.Sprintf("EU%d", linegroup)
	default:
		return ""
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func deleteByGUID(mapNpcs map[int64]interface{}, guid *pb.GUID) {
	if guid == nil {
		return
	}
	index := guid.GetIndex()
	for id := range mapNpcs {
		if uint32(id) == index {
			delete(mapNpcs, id)
		}
	}
}

func parseIncomingPayload(payload []byte, msg proto.Message) bool {
	if err := ParseProtobuf(payload, msg); err == nil {
		return true
	}

	if len(payload) < 2 {
		return false
	}

	nonceLen := int(payload[0]) | int(payload[1])<<8
	if nonceLen < 0 || len(payload) < 2+nonceLen {
		return false
	}

	return ParseProtobuf(payload[2+nonceLen:], msg) == nil
}
