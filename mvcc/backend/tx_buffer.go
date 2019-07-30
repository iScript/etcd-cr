package backend

// txBuffer处理txWriteBuffer和txReadBufferhandles共享的功能，一般被他们直接嵌入结构体
type txBuffer struct {
	buckets map[string]*bucketBuffer
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
	txBuffer // 组合，相当于继承
	seq      bool
}

type kv struct {
	key []byte
	val []byte
}

// 保存着缓冲的键值对，准备提交
type bucketBuffer struct {
	buf []kv
	// used tracks number of elements in use so buf can be reused without reallocation.
	// 追踪元素数量，以便buf可以在不重新分配的情况下重用
	used int
}

type txReadBuffer struct{ txBuffer } // 组合，相当于继承，调用是可以是 txReadBuffer.x 或txReadBuffer.txBuffer.x
