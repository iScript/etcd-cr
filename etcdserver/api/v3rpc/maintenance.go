package v3rpc

import (
	"github.com/iScript/etcd-cr/mvcc/backend"
)

type KVGetter interface {
	//KV() mvcc.ConsistentWatchableKV
}

type BackendGetter interface {
	Backend() backend.Backend
}

type Alarmer interface {

	//Alarms() []*pb.AlarmMember
	//Alarm(ctx context.Context, ar *pb.AlarmRequest) (*pb.AlarmResponse, error)
}
