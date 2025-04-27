/*
Package channel
*/
package channel

import (
	"errors"
	"sync"
)

type Broker struct {
	mutex sync.RWMutex
	chans []chan Msg
	// 有主题的chans
	//chans map[string][]chan Msg
}

// Send 发送
func (b *Broker) Send(m Msg) error {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	for _, ch := range b.chans {
		select {
		case ch <- m:
		default:
			return errors.New("chanel 已满")
		}
	}
	return nil
}

// Subscribe 订阅
func (b *Broker) Subscribe(capacity int) (<-chan Msg, error) {
	res := make(chan Msg, capacity)
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.chans = append(b.chans, res)
	return res, nil
}

// Close 关闭
func (b *Broker) Close() error {
	b.mutex.Lock()
	chans := b.chans
	b.chans = nil
	b.mutex.Unlock()

	// 避免了重复Close chan的问题
	for _, ch := range chans {
		close(ch)
	}
	return nil
}

type Msg struct {
	// Topic 主题
	//Topic   string
	Content string
}
