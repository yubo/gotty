package tty

import (
	"container/list"
	"sync"
	"sync/atomic"
)

type Slist struct {
	sync.RWMutex
	push_cnt      uint64
	push_last_cnt uint64
	pop_cnt       uint64
	pop_last_cnt  uint64
	push_rate     uint32
	pop_rate      uint32
	list          *list.List
}

func (l *Slist) Front() *list.Element {
	l.Lock()
	defer l.Unlock()

	return l.list.Front()
}

func (l *Slist) Push(v interface{}) *list.Element {
	l.Lock()
	defer l.Unlock()

	atomic.AddUint64(&l.push_cnt, 1)
	return l.list.PushBack(v)
}

func (l *Slist) Len() int {
	l.RLock()
	defer l.RUnlock()
	return l.list.Len()
}

func (l *Slist) Pop() interface{} {
	l.Lock()
	defer l.Unlock()

	atomic.AddUint64(&l.pop_cnt, 1)
	e := l.list.Front()
	if e != nil {
		l.list.Remove(e)
		return e.Value
	}
	return nil
}

func (l *Slist) Remove(e *list.Element) interface{} {
	l.Lock()
	defer l.Unlock()
	atomic.AddUint64(&l.pop_cnt, 1)
	if e != nil {
		l.list.Remove(e)
		return e.Value
	}
	return nil

}
