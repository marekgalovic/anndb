package utils

import (
	"sync";
)

type Notificator struct {
	chans map[interface{}]chan interface{}
	mu *sync.RWMutex
}

func NewNotificator() *Notificator {
	return &Notificator {
		chans: make(map[interface{}]chan interface{}),
		mu: &sync.RWMutex{},
	}
}

func (this *Notificator) Create(k interface{}) <- chan interface{} {
	c := make(chan interface{})
	this.mu.Lock()
	this.chans[k] = c
	this.mu.Unlock()

	return c
}

func (this *Notificator) Remove(k interface{}) {
	this.mu.Lock()
	defer this.mu.Unlock()
	if c, exists := this.chans[k]; exists {
		delete(this.chans, k)
		close(c)
	}
}

func (this *Notificator) Notify(k interface{}, v interface{}) {
	this.mu.RLock()
	defer this.mu.RUnlock()
	if c, exists := this.chans[k]; exists {
		select {
		case c <- v:
		default:
		}
	}
}

