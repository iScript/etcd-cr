package etcdserver

type notifier struct {
	//特殊的struct{}类型的channel，它不能被写入任何数据，只有通过close()函数进行关闭操作，才能进行输出操作。。struct类型的channel不占用任何内存！
	//一般应用场景 做一件事情， <-channel一直阻塞 ， 做完close之后， <-channel接收到消息，继续执行下面的
	c   chan struct{}
	err error
}

func newNotifier() *notifier {
	return &notifier{
		c: make(chan struct{}),
	}
}
