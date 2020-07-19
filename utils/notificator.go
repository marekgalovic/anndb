package utils

import (
	"sync";
	"errors";

	"github.com/satori/go.uuid";
)

var (
	ErrNotificatorChannelDoesNotExist error = errors.New("Notificator channel does not exist")
	ErrNotificatorReceiverNotAvailable error = errors.New("Notificator receiver not available")
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

func (this *Notificator) Create(bufSize int) (<- chan interface{}, uuid.UUID) {
	id := uuid.NewV4()
	c := make(chan interface{}, bufSize)
	this.mu.Lock()
	this.chans[id] = c
	this.mu.Unlock()

	return c, id
}

func (this *Notificator) Remove(id uuid.UUID) error {
	this.mu.Lock()
	defer this.mu.Unlock()
	if c, exists := this.chans[id]; exists {
		delete(this.chans, id)
		close(c)
		return nil
	}

	return ErrNotificatorChannelDoesNotExist
}

func (this *Notificator) Notify(id uuid.UUID, v interface{}, blocking bool) error {
	this.mu.RLock()
	defer this.mu.RUnlock()

	if c, exists := this.chans[id]; exists {
		if blocking {
			c <- v
		} else {
			select {
			case c <- v:
			default:
				return ErrNotificatorReceiverNotAvailable
			}	
		}
		return nil
	}

	return ErrNotificatorChannelDoesNotExist
}

