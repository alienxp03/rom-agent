package resources

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type itemDBEntry struct {
	ID     int    `json:"id"`
	NameZh string `json:"NameZh"`
}

var (
	itemNameOnce sync.Once
	itemNameMap  map[int]string
	itemNameErr  error
)

func LookupItemName(id int) string {
	itemNameOnce.Do(func() {
		itemNameMap, itemNameErr = loadItemNameMap()
	})
	if itemNameErr != nil || itemNameMap == nil {
		return ""
	}
	return itemNameMap[id]
}

func loadItemNameMap() (map[int]string, error) {
	paths := []string{
		filepath.Join("..", "src", "resources", "item_db.json"),
		filepath.Join("src", "resources", "item_db.json"),
	}

	var data []byte
	var err error
	var loadedPath string
	for _, path := range paths {
		data, err = os.ReadFile(path)
		if err == nil {
			loadedPath = path
			break
		}
	}
	if data == nil {
		return nil, fmt.Errorf("read item_db.json: %w", err)
	}

	var entries []itemDBEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse %s: %w", loadedPath, err)
	}

	itemNames := make(map[int]string, len(entries))
	for _, entry := range entries {
		if entry.ID == 0 || entry.NameZh == "" {
			continue
		}
		itemNames[entry.ID] = entry.NameZh
	}
	return itemNames, nil
}
