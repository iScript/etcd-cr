package v2store

import "time"

//比较函数结果枚举
const (
	CompareMatch = iota
	CompareIndexNotMatch
	CompareValueNotMatch
	CompareNotMatch
)

var Permanent time.Time

// node 是存储系统中的基本元素
type node struct {
	Path string

	CreatedIndex  uint64
	ModifiedIndex uint64

	Parent *node `json:"-"` // should not encode this field! avoid circular dependency.

	ExpireTime time.Time
	Value      string           // for key-value pair
	Children   map[string]*node // for directory

	// A reference to the store this node is attached to.
	store *store
}
