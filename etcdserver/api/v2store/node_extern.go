package v2store

import "time"

//NodeExtern是带有附加字段的内部节点的外部表示
type NodeExtern struct {
	Key           string      `json:"key,omitempty"`
	Value         *string     `json:"value,omitempty"`
	Dir           bool        `json:"dir,omitempty"`
	Expiration    *time.Time  `json:"expiration,omitempty"`
	TTL           int64       `json:"ttl,omitempty"`
	Nodes         NodeExterns `json:"nodes,omitempty"`
	ModifiedIndex uint64      `json:"modifiedIndex,omitempty"`
	CreatedIndex  uint64      `json:"createdIndex,omitempty"`
}

type NodeExterns []*NodeExtern
