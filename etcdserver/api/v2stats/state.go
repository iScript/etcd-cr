package v2stats

type Stats interface {
	// 返回server统计数据
	SelfStats() []byte
	// 如果本机是leader，返回follower统计数据
	//LeaderStats() []byte
	// 返回store统计数据
	//StoreStats() []byte
}
