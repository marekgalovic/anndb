package utils

import (
	"sync";

	"github.com/satori/go.uuid";
)

type Notificator struct {
	chans map[uuid.UUID]chan interface{}
	mu *sync.RWMutex
}

func NewNotificator() *Notificator {
	return &Notificator {
		chans: make(map[uuid.UUID]chan interface{}),
		mu: &sync.RWMutex{},
	}
}

func (this *Notificator) Create() (<- chan interface{}, uuid.UUID) {
	id := uuid.NewV4()
	c := make(chan interface{})
	this.mu.Lock()
	this.chans[id] = c
	this.mu.Unlock()

	return c, id
}

func (this *Notificator) Remove(id uuid.UUID) {
	this.mu.Lock()
	defer this.mu.Unlock()
	if c, exists := this.chans[id]; exists {
		delete(this.chans, id)
		close(c)
	}
}

func (this *Notificator) Notify(id uuid.UUID, v interface{}) {
	this.mu.RLock()
	defer this.mu.RUnlock()
	if c, exists := this.chans[id]; exists {
		select {
		case c <- v:
		default:
		}
	}
}

