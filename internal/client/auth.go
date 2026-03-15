package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/alienxp03/rom-agent/internal/config"
)

const (
	defaultPhonePlat = "Android"
	defaultVer       = "6.x"
	defaultUserAgent = "UnityPlayer/5.6.0f3 (UnityWebRequest/1.0, libcurl/7.51.0-DEV)"
)

// AuthData contains the login data needed for the TCP game login.
type AuthData struct {
	AccID             int64
	SHA1              string
	Timestamp         int
	ServerID          int
	GameServerHost    string
	GameServerIP      string
	GameServerPort    int
	GameServerVersion string
}

// AuthClient implements the HTTP version/auth flow used before TCP login.
type AuthClient struct {
	cfg       *config.Config
	account   config.Account
	blueberry int
	version   int
	client    *http.Client
}

func NewAuthClient(cfg *config.Config, account config.Account, blueberry, version int) *AuthClient {
	return &AuthClient{
		cfg:       cfg,
		account:   account,
		blueberry: blueberry,
		version:   version,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Authenticate runs the version + auth requests and returns the selected linegroup data.
func (ac *AuthClient) Authenticate(ctx context.Context) (*AuthData, error) {
	versionRes, err := ac.sendVersionRequest(ctx)
	if err != nil {
		return nil, err
	}
	return ac.sendAuthRequest(ctx, versionRes.GameServerVersion, versionRes.GameServerPort)
}

type versionResponse struct {
	Status int `json:"status"`
	Data   struct {
		GatePort string `json:"gateport"`
		Server   string `json:"server"`
	} `json:"data"`
	Message string `json:"message"`
}

type authResponse struct {
	Status int `json:"status"`
	Data   struct {
		Gates []struct {
			Host string `json:"host"`
		} `json:"gates"`
		Regions []struct {
			Linegroup int      `json:"linegroup"`
			ServerID  jsonInt  `json:"serverid"`
			SHA1      string   `json:"sha1"`
			AccID     int64    `json:"accid"`
			Timestamp jsonInt  `json:"timestamp"`
			Gateways  []string `json:"gateways"`
		} `json:"regions"`
	} `json:"data"`
	Message string `json:"message"`
}

func (ac *AuthClient) sendVersionRequest(ctx context.Context) (*AuthData, error) {
	form := url.Values{
		"branch":         {"Release"},
		"phonename":      {"OnePlus ONEPLUS A5000"},
		"phoneplat":      {defaultPhonePlat},
		"client":         {strconv.Itoa(ac.version)},
		"appPreVersion":  {strconv.Itoa(ac.cfg.AppPreVersion)},
		"clientCode":     {strconv.Itoa(ac.cfg.ClientCode)},
		"memory":         {"4096"},
		"totalDriveSize": {"8125880"},
		"cpuName":        {"ARMv7 VFPv3 NEON VMH"},
		"screenWidth":    {"1600"},
		"screenHeight":   {"900"},
		"gpuName":        {"Adreno (TM) 540"},
		"gpuVersion":     {"OpenGL ES 3.0"},
		"gpuType":        {"OpenGLES3"},
		"plat":           {strconv.Itoa(ac.cfg.Plat)},
	}

	reqURL := fmt.Sprintf("%s/version?lang=%d", ac.cfg.AuthBaseUrl, ac.cfg.Lang)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build version request: %w", err)
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var res versionResponse
	if err := ac.doJSON(req, &res); err != nil {
		return nil, fmt.Errorf("version request failed: %w", err)
	}
	if res.Status != 0 {
		return nil, fmt.Errorf("version request returned status %d (%s)", res.Status, res.Message)
	}

	port, err := strconv.Atoi(res.Data.GatePort)
	if err != nil {
		return nil, fmt.Errorf("parse gate port %q: %w", res.Data.GatePort, err)
	}

	return &AuthData{
		GameServerPort:    port,
		GameServerVersion: res.Data.Server,
	}, nil
}

func (ac *AuthClient) sendAuthRequest(ctx context.Context, gameServerVersion string, gameServerPort int) (*AuthData, error) {
	params := url.Values{
		"sid":           {ac.account.Sid},
		"p":             {strconv.Itoa(ac.cfg.Plat)},
		"sver":          {gameServerVersion},
		"cver":          {strconv.Itoa(ac.cfg.ClientCode)},
		"client_id":     {ac.account.ClientId},
		"mac_key":       {ac.account.MacKey},
		"lang":          {strconv.Itoa(ac.cfg.Lang)},
		"appPreVersion": {strconv.Itoa(ac.cfg.AppPreVersion)},
		"phoneplat":     {defaultPhonePlat},
		"ver":           {defaultVer},
	}
	reqURL := fmt.Sprintf("%s/auth?%s", ac.cfg.AuthBaseUrl, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build auth request: %w", err)
	}
	req.Header.Set("User-Agent", defaultUserAgent)

	var res authResponse
	if err := ac.doJSON(req, &res); err != nil {
		return nil, fmt.Errorf("auth request failed: %w", err)
	}
	if res.Status != 0 {
		return nil, fmt.Errorf("auth request returned status %d (%s)", res.Status, res.Message)
	}
	if len(res.Data.Gates) == 0 {
		return nil, fmt.Errorf("auth response did not include any gates")
	}

	authData := &AuthData{
		GameServerHost:    res.Data.Gates[0].Host,
		GameServerPort:    gameServerPort,
		GameServerVersion: gameServerVersion,
	}

	for _, region := range res.Data.Regions {
		if region.Linegroup != ac.cfg.GameLinegroup {
			continue
		}
		if len(region.Gateways) == 0 {
			return nil, fmt.Errorf("linegroup %d has no gateways", region.Linegroup)
		}

		ipAddrs, err := net.DefaultResolver.LookupHost(ctx, region.Gateways[0])
		if err != nil || len(ipAddrs) == 0 {
			return nil, fmt.Errorf("resolve gateway %q: %w", region.Gateways[0], err)
		}

		authData.AccID = region.AccID
		authData.SHA1 = region.SHA1
		authData.Timestamp = int(region.Timestamp)
		authData.ServerID = int(region.ServerID)
		authData.GameServerIP = ipAddrs[0]
		return authData, nil
	}

	return nil, fmt.Errorf("server has no linegroup %d", ac.cfg.GameLinegroup)
}

func (ac *AuthClient) doJSON(req *http.Request, out interface{}) error {
	resp, err := ac.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	return nil
}

// jsonInt handles fields that can arrive as a JSON string or number.
type jsonInt int

func (i *jsonInt) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*i = 0
		return nil
	}

	var n int
	if err := json.Unmarshal(data, &n); err == nil {
		*i = jsonInt(n)
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		*i = 0
		return nil
	}

	n, err := strconv.Atoi(s)
	if err != nil {
		return err
	}
	*i = jsonInt(n)
	return nil
}
