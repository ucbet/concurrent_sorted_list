package main

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

const (
	unchange = 0
	changed  = 1
)

type IntList struct {
	head   *intNode
	length int
}

type intNode struct {
	mtx    sync.Mutex
	value  int
	next   *intNode
	change int
}

func newIntNode(value int) *intNode {
	return &intNode{value: value, change: unchange}
}

func NewInt() *IntList {
	return &IntList{head: newIntNode(0)}
}

func (l *IntList) Insert(value int) bool {
	for {
		// 原子找到value需要插入的位置
		a := atomicNode(&l.head)
		b := atomicNode(&a.next)

		for b != nil &&
			atomicInt32(&b.value) < value {
			a = b
			b = atomicNode(&a.next)
		}

		a.mtx.Lock()
		// 如果a.next不为b或a已经被标记删除了，自旋
		if atomicNode(&a.next) !=
			atomicNode(&b) ||
			atomicInt32(&a.change) == changed {
			a.mtx.Unlock()
			continue
		}
		// 如果当前b的值与value相同，直接返回false，保证相同的值不重复插入
		if b != nil && atomicInt32(&b.value) == value {
			a.mtx.Unlock()
			return false
		}

		// 在a和b之间插入节点c
		c := newIntNode(value)
		atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&c.next)), unsafe.Pointer(b))
		atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&a.next)), unsafe.Pointer(c))

		// 长度原子++
		atomic.AddInt32((*int32)(unsafe.Pointer(&l.length)), 1)
		a.mtx.Unlock()
		return true

	}
}

func (l *IntList) Delete(value int) bool {
	for {
		// 原子找到value需要插入的位置
		a := atomicNode(&l.head)
		b := atomicNode(&a.next)

		for b != nil &&
			atomicInt32(&b.value) < value {
			a = b
			b = atomicNode(&a.next)
		}
		// 遍历到最后仍找不到value直接返回false
		if b == nil {
			return false
		}

		// 锁定b节点，如果b被删除了，自旋
		b.mtx.Lock()
		if atomicInt32(&b.change) == changed {
			b.mtx.Unlock()
			continue
		}
		// 如果a.next不为b或a已经被标记删除了，自旋
		a.mtx.Lock()
		if atomicNode(&a.next) != b ||
			atomicInt32(&a.change) == changed {
			a.mtx.Unlock()
			b.mtx.Unlock()
			continue
		}
		// 如果b节点的值不是value，找不到要删除的节点，返回false
		if atomicInt32(&b.value) != value {
			a.mtx.Unlock()
			b.mtx.Unlock()
			return false
		}
		// 标记b为要删除的节点并进行删除
		atomic.StoreInt32((*int32)(unsafe.Pointer(&b.change)), changed)
		atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&a.next)), unsafe.Pointer(b.next))

		// 长度原子--
		atomic.AddInt32((*int32)(unsafe.Pointer(&l.length)), -1)
		a.mtx.Unlock()
		b.mtx.Unlock()
		return true

	}
}

func (l *IntList) Contains(value int) bool {
	// 从第一个节点并依次原子遍历
	head := atomicNode(&l.head.next)
	for head != nil && atomicInt32(&head.value) <= value {
		if atomicInt32(&head.value) == value {
			return true
		}
		head = atomicNode(&head.next)
	}
	return false
}

func (l *IntList) Range(f func(value int) bool) {

	head := atomicNode(&l.head.next)
	for head != nil {
		if !f(atomicInt32(&head.value)) {
			break
		}
		head = atomicNode(&head.next)
	}
}

func (l *IntList) Len() int {
	return atomicInt32(&l.length)
}

//原子读node
func atomicNode(node **intNode) *intNode {
	return (*intNode)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(node))))
}

//原子读value
func atomicInt32(value *int) int {
	return int(atomic.LoadInt32((*int32)(unsafe.Pointer(value))))
}
