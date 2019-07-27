package membership

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"sort"
	"sync"

	"github.com/iScript/etcd-cr/mvcc/backend"
	"github.com/iScript/etcd-cr/pkg/types"

	"github.com/coreos/go-semver/semver"
	"go.uber.org/zap"
)

const maxLearners = 1

// RaftCluster 在同一个raft集群中的一组member
type RaftCluster struct {
	lg *zap.Logger

	localID types.ID
	cid     types.ID
	token   string

	//v2store v2store.Store
	be backend.Backend

	sync.Mutex // guards the fields below
	version    *semver.Version
	members    map[types.ID]*Member
	// removed contains the ids of removed members in the cluster.
	// removed id cannot be reused.
	removed map[types.ID]bool
}

// 从url map中新建raft集群
func NewClusterFromURLsMap(lg *zap.Logger, token string, urlsmap types.URLsMap) (*RaftCluster, error) {

	c := NewCluster(lg, token)

	for name, urls := range urlsmap {
		m := NewMember(name, urls, token, nil)
		if _, ok := c.members[m.ID]; ok { //判断该id的member是否已存在
			return nil, fmt.Errorf("member exists with identical ID %v", m)
		}
		// if uint64(m.ID) == raft.None {
		// 	return nil, fmt.Errorf("cannot use %x as member id", raft.None)
		// }
		c.members[m.ID] = m
	}
	c.genID()

	return c, nil
}

func NewCluster(lg *zap.Logger, token string) *RaftCluster {
	return &RaftCluster{
		lg:      lg,
		token:   token,
		members: make(map[types.ID]*Member), //Member赋值前要初始化
		removed: make(map[types.ID]bool),
	}
}

func (c *RaftCluster) MemberIDs() []types.ID {
	c.Lock()
	defer c.Unlock()
	var ids []types.ID
	for _, m := range c.members {
		ids = append(ids, m.ID)
	}
	sort.Sort(types.IDSlice(ids))
	return ids
}

func (c *RaftCluster) genID() {
	mIDs := c.MemberIDs()
	b := make([]byte, 8*len(mIDs))
	for i, id := range mIDs {
		binary.BigEndian.PutUint64(b[8*i:], uint64(id))
	}
	hash := sha1.Sum(b)
	c.cid = types.ID(binary.BigEndian.Uint64(hash[:8]))
}

//通过名称返回member
func (c *RaftCluster) MemberByName(name string) *Member {
	c.Lock()
	defer c.Unlock()
	var memb *Member
	for _, m := range c.members {
		if m.Name == name {
			if memb != nil {
				if c.lg != nil {
					c.lg.Panic("two member with same name found", zap.String("name", name))
				}
			}
			memb = m
		}
	}
	return memb.Clone()
}
