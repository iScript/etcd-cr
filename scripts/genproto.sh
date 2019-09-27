#!/usr/bin/env bash
#
# Generate all etcd protobuf bindings.
# Run from repository root.
#
set -e  #执行的时候如果出现了返回值为非零，整个脚本 就会立即退出 

# 是否在根目录
if ! [[ "$0" =~ scripts/genproto.sh ]]; then
	echo "must be run from repository root"
	exit 255
fi

export PATH=$PATH:$GOPATH/bin

ETCD_IO_ROOT="/Users/ykq/Desktop"
ETCD_ROOT="${ETCD_IO_ROOT}/etcd"
GOGOPROTO_ROOT="${GOPATH}/src/github.com/gogo/protobuf"
GOGOPROTO_PATH="${GOGOPROTO_ROOT}:${GOGOPROTO_ROOT}/protobuf"
GRPC_GATEWAY_ROOT="${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway"

DIRS="./etcdserver/etcdserverpb ./mvcc/mvccpb ./auth/authpb"
export GO111MODULE=on
export GOPROXY=https://goproxy.io

go get -u github.com/gogo/protobuf/{proto,protoc-gen-gogo,gogoproto}
go get -u golang.org/x/tools/cmd/goimports

for dir in ${DIRS}; do
    pushd "${dir}"  # 将目录加入栈中并进入
    echo 22
   
    protoc --gofast_out=plugins=grpc,import_prefix=github.com/iScript/:. -I=".:${GOGOPROTO_PATH}:${ETCD_IO_ROOT}:${GRPC_GATEWAY_ROOT}/third_party/googleapis" *.proto
    sed -i.bak -E 's/github\.com\/iScript\/(gogoproto|github\.com|golang\.org|google\.golang\.org)/\1/g' ./*.pb.go
    
    sed -i.bak -E 's/github\.com\/iScript\/(errors|fmt|io|context)/\1/g' ./*.pb.go

    sed -i.bak -E 's/import _ \"gogoproto\"//g' ./*.pb.go

    sed -i.bak -E 's/import fmt \"fmt\"//g' ./*.pb.go
    
    sed -i.bak -E 's/import _ \"github\.com\/iScript\/google\/api\"//g' ./*.pb.go
		
	sed -i.bak -E 's/import _ \"google\.golang\.org\/genproto\/googleapis\/api\/annotations\"//g' ./*.pb.go
	rm -f ./*.bak
	goimports -w ./*.pb.go
    popd    # 退出
done