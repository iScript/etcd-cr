package wait

import (
	"log"
	"sync"
)

type Wait interface {

	// 注册
	Register(id uint64) <-chan interface{}
	//根据id触发等待中的通道
	//Trigger(id uint64, x interface{})
	//IsRegistered(id uint64) bool
}

type list struct {
	l sync.RWMutex
	m map[uint64]chan interface{} // value是通道，通道值是任意类型
}

// new一个Wait实例
func New() Wait {
	return &list{m: make(map[uint64]chan interface{})}
}

//实现register方法
func (w *list) Register(id uint64) <-chan interface{} {
	w.l.Lock()
	defer w.l.Unlock()
	ch := w.m[id]
	if ch == nil {
		ch = make(chan interface{}, 1)
		w.m[id] = ch
	} else {
		log.Panicf("dup id %x", id)
	}
	return ch
}
