package utils

import (
	"sync";
	"time";
    "testing";

    "github.com/stretchr/testify/assert";

    "github.com/satori/go.uuid";
)

func TestNotificatorNotifyBlocking(t *testing.T) {
	n := NewNotificator()
	c, id := n.Create(0)

	wg := &sync.WaitGroup{}
	wg.Add(2)

	var val interface{} = nil
	go func() {
		defer wg.Done()

		select {
		case v := <- c:
			val = v
		}
	}()

	go func() {
		defer wg.Done()
		n.Notify(id, 123, true)
	}()

	wg.Wait()

	assert.NotNil(t, val)
	if val != nil {
		assert.Equal(t, 123, val.(int))
	}
}

func TestNotificatorNotifyNonBlockingBuffered(t *testing.T) {
	done := make(chan bool)
	n := NewNotificator()
	c, id := n.Create(1)

	wg := &sync.WaitGroup{}
	wg.Add(2)

	var val interface{} = nil
	go func() {
		defer wg.Done()

		select {
		case v := <- c:
			val = v
		case <- done:
			return
		}
	}()

	go func() {
		defer wg.Done()
		n.Notify(id, 123, false)
		time.Sleep(10 * time.Millisecond)
		close(done)
	}()

	wg.Wait()

	assert.NotNil(t, val)
	if val != nil {
		assert.Equal(t, 123, val.(int))
	}
}

func TestNotificatorRemoveRemovesChannel(t *testing.T) {
	n := NewNotificator()
	_, id := n.Create(10)

	err := n.Notify(id, 123, false)
	assert.Nil(t, err)

	err = n.Remove(id)
	assert.Nil(t, err)

	err = n.Notify(id, 123, false)
	assert.NotNil(t, err)
	assert.Equal(t, err, ErrNotificatorChannelDoesNotExist)
}

func TestNotificatorNotifyReturnsErrorOnNonBlockingWithoutBuffer(t *testing.T) {
	n := NewNotificator()
	_, id := n.Create(0)

	err := n.Notify(id, 123, false)
	assert.NotNil(t, err)
	assert.Equal(t, err, ErrNotificatorReceiverNotAvailable)
}

func TestNotificatorNotifyOnChannelThatDoesNotExist(t *testing.T) {
	n := NewNotificator()
	err := n.Notify(uuid.NewV4(), 123, true)
	assert.NotNil(t, err)
	assert.Equal(t, err, ErrNotificatorChannelDoesNotExist)
}

func TestNotificatorRemoveChannelThatDoesNotExist(t *testing.T) {
	n := NewNotificator()
	err := n.Remove(uuid.NewV4())
	assert.NotNil(t, err)
	assert.Equal(t, err, ErrNotificatorChannelDoesNotExist)
}