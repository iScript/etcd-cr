package membership

import (
	"fmt"
	"path"

	"github.com/iScript/etcd-cr/mvcc/backend"
	"github.com/iScript/etcd-cr/pkg/types"
)

const (
	attributesSuffix     = "attributes"
	raftAttributesSuffix = "raftAttributes"

	// the prefix for stroing membership related information in store provided by store pkg.
	storePrefix = "/0"
)

var (
	membersBucketName        = []byte("members")
	membersRemovedBucketName = []byte("members_removed")
	clusterBucketName        = []byte("cluster")

	StoreMembersPrefix        = path.Join(storePrefix, "members")
	storeRemovedMembersPrefix = path.Join(storePrefix, "removed_members")
)

func mustCreateBackendBuckets(be backend.Backend) {
	tx := be.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	// 创建库
	tx.UnsafeCreateBucket(membersBucketName)
	tx.UnsafeCreateBucket(membersRemovedBucketName)
	tx.UnsafeCreateBucket(clusterBucketName)
}

func MemberStoreKey(id types.ID) string {
	return path.Join(StoreMembersPrefix, id.String())
}

func StoreClusterVersionKey() string {
	return path.Join(storePrefix, "version")
}

func MemberAttributesStorePath(id types.ID) string {
	return path.Join(MemberStoreKey(id), attributesSuffix)
}

func MustParseMemberIDFromKey(key string) types.ID {

	id, err := types.IDFromString(path.Base(key)) //path.base 获取目录的最后一个 ， 即id
	if err != nil {
		fmt.Println("unexpected parse member id error: %v", err)
	}
	return id
}
