package membership

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/iScript/etcd-cr/etcdserver/api/v2store"
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

	//fmt.Println(c, "cluster.go")
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

// 返回cluster id
func (c *RaftCluster) ID() types.ID { return c.cid }

// 返回cluster中的members
func (c *RaftCluster) Members() []*Member {
	c.Lock()
	defer c.Unlock()
	var ms MembersByID
	for _, m := range c.members {
		ms = append(ms, m.Clone())
	}
	sort.Sort(ms)
	return []*Member(ms)
}

// 根据id返回member
func (c *RaftCluster) Member(id types.ID) *Member {
	c.Lock()
	defer c.Unlock()
	return c.members[id].Clone()
}

//通过名称返回member
func (c *RaftCluster) MemberByName(name string) *Member {
	c.Lock()
	defer c.Unlock()
	var memb *Member
	for _, m := range c.members {
		if m.Name == name {
			//？？？？ 这是啥判断 memb上面赋值100% nil？
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

// 返回member id数组
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

// 输出结构体时自动调用
func (c *RaftCluster) String() string {
	c.Lock()
	defer c.Unlock()
	b := &bytes.Buffer{}
	fmt.Fprintf(b, "{ClusterID:%s ", c.cid)
	var ms []string
	for _, m := range c.members {
		ms = append(ms, fmt.Sprintf("%+v", m))
	}
	fmt.Fprintf(b, "Members:[%s] ", strings.Join(ms, " "))
	var ids []string
	for id := range c.removed {
		ids = append(ids, id.String())
	}
	fmt.Fprintf(b, "RemovedMemberIDs:[%s]}", strings.Join(ids, " "))
	return b.String()
}

func (c *RaftCluster) genID() {
	mIDs := c.MemberIDs()
	b := make([]byte, 8*len(mIDs))
	for i, id := range mIDs {
		//fmt.Println(b, b[8*i:], id)
		// 每次循环从0 8 16 等位置放入
		binary.BigEndian.PutUint64(b[8*i:], uint64(id)) //将id放入字节切片
	}
	hash := sha1.Sum(b)

	c.cid = types.ID(binary.BigEndian.Uint64(hash[:8])) // 从BigEndian中获取
	//fmt.Println(c.cid)
}

func (c *RaftCluster) SetID(localID, cid types.ID) {
	c.localID = localID
	c.cid = cid
}

func (c *RaftCluster) SetStore(st v2store.Store) { //c.v2store = st
}

func (c *RaftCluster) SetBackend(be backend.Backend) {
	c.be = be
	// mustCreateBackendBuckets(c.be)  还没设置batchTx.tx , 无法创建
}
