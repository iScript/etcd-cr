package backend

// txBuffer处理txWriteBuffer和txReadBufferhandles共享的功能，一般被他们直接嵌入结构体
type txBuffer struct {
	buckets map[string]*bucketBuffer //记录了bucket名称与对应bucketBuffer的映射关系，bucketbuffer中缓存了对应bucket中的键值对数据
}

func (txb *txBuffer) reset() {
	//fmt.Println("reset")
	for k, v := range txb.buckets {
		if v.used == 0 {
			// demote
			delete(txb.buckets, k)
		}
		v.used = 0
	}
}

// txWriteBuffer buffers writes of pending updates that have not yet committed.
type txWriteBuffer struct {
	txBuffer // 相当于继承
	seq      bool
}

type kv struct {
	key []byte
	val []byte
}

// 保存着缓冲的键值对，准备提交
type bucketBuffer struct {
	buf  []kv // 每一个元素都表示一个键值对
	used int  // 记录buf中目前使用的下标位置
}

type txReadBuffer struct {
	txBuffer
} // 组合，相当于继承，调用是可以是 txReadBuffer.x 或txReadBuffer.txBuffer.x
