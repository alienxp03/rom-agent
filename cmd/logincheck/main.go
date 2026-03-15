package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alienxp03/rom-agent/internal/client"
	"github.com/alienxp03/rom-agent/internal/config"
)

func loadGameVersion(cfg *config.Config) (int, int) {
	type blueberryFile struct {
		Blueberry int `json:"blueberry"`
		Version   int `json:"version"`
	}

	paths := []string{
		filepath.Join("..", "src", "resources", "blueberry.json"),
		filepath.Join("src", "resources", "blueberry.json"),
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var meta blueberryFile
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}

		version := meta.Version
		if cfg.ClientVersion > 0 {
			version = cfg.ClientVersion
		}
		return meta.Blueberry, version
	}

	if cfg.ClientVersion > 0 {
		return 0, cfg.ClientVersion
	}
	return 0, 0
}

func main() {
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		panic(err)
	}

	blueberry, version := loadGameVersion(cfg)
	clientCfg := cfg.Clients[0]

	authClient := client.NewAuthClient(cfg, clientCfg.Account, blueberry, version)
	authData, err := authClient.Authenticate(context.Background())
	if err != nil {
		panic(err)
	}
	fmt.Println("auth ok", authData.GameServerHost, authData.GameServerIP, authData.GameServerPort, authData.GameServerVersion)

	gameClient := client.NewGameClient(blueberry, version, cfg.Lang, cfg.AppPreVersion, clientCfg.Account.SmDeviceId, cfg.GameLinegroup)
	defer gameClient.Close()

	if err := gameClient.LoginWithAuth(context.Background(), authData, clientCfg.Account.DeviceId, clientCfg.Character); err != nil {
		panic(err)
	}
	fmt.Println("login ok", gameClient.GetServer(), gameClient.GetZoneIdStr())

	if err := gameClient.PostLogin(context.Background()); err != nil {
		panic(err)
	}
	fmt.Println("post-login ok", gameClient.GetServer(), gameClient.GetZoneIdStr(), gameClient.GetMapId())
}
