package asset

import "encoding/json"

type Asset struct {
	AssetID string          `json:"assetId"`
	Metadata json.RawMessage `json:"metadata"`
	ACL     []string        `json:"acl"`
}
