package v3rpc

import (
	"crypto/tls"
	"fmt"
	"math"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/iScript/etcd-cr/etcdserver"
	pb "github.com/iScript/etcd-cr/etcdserver/etcdserverpb"
	"google.golang.org/grpc"
)

const (
	grpcOverheadBytes = 512 * 1024
	maxStreams        = math.MaxUint32
	maxSendBytes      = math.MaxInt32
)

// 创建grpc server
// ...任意个grpc.ServerOption
func Server(s *etcdserver.EtcdServer, tls *tls.Config, gopts ...grpc.ServerOption) *grpc.Server {
	var opts []grpc.ServerOption
	opts = append(opts, grpc.CustomCodec(&codec{})) //自定义解编码

	if tls != nil {
		//bundle := credentials.NewBundle(credentials.Config{TLSConfig: tls})
		//opts = append(opts, grpc.Creds(bundle.TransportCredentials()))
	}

	//设置grpc拦截器
	opts = append(opts, grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
		// newLogUnaryInterceptor(s),
		// newUnaryInterceptor(s),
		grpc_prometheus.UnaryServerInterceptor,
	)))

	opts = append(opts, grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
		//newStreamInterceptor(s),
		grpc_prometheus.StreamServerInterceptor,
	)))

	// // 最大接收发送大小
	opts = append(opts, grpc.MaxRecvMsgSize(int(s.Cfg.MaxRequestBytes+grpcOverheadBytes)))
	opts = append(opts, grpc.MaxSendMsgSize(maxSendBytes))
	opts = append(opts, grpc.MaxConcurrentStreams(maxStreams))

	// 创建一个grpc server
	grpcServer := grpc.NewServer(append(opts, gopts...)...) // 3个点，切片被打散传入

	pb.RegisterKVServer(grpcServer, NewQuotaKVServer(s)) // 注册服务，传入实现了KVServer接口的对象,如put range

	fmt.Println("return grpc server")
	return grpcServer
}
