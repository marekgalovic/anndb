package cluster

import (
	"sync";
	"errors";

	"google.golang.org/grpc";
)

var (
	NodeAddressNotFoundError error = errors.New("Node address not found")
)

type Conn struct {
	addresses map[uint64]string
	addressesMu sync.RWMutex
	conns map[uint64]*grpc.ClientConn
	connsMu sync.RWMutex
}

func NewConn() *Conn {
	return &Conn {
		addresses: make(map[uint64]string),
		addressesMu: sync.RWMutex{},
		conns: make(map[uint64]*grpc.ClientConn),
		connsMu: sync.RWMutex{},
	}
}

func (this *Conn) Nodes() map[uint64]string {
	this.addressesMu.RLock()
	defer this.addressesMu.RUnlock()

	nodes := make(map[uint64]string)
	for nodeId, address := range this.addresses {
		nodes[nodeId] = address
	}
	return nodes
}

func (this *Conn) NodeIds() []uint64 {
	this.addressesMu.RLock()
	defer this.addressesMu.RUnlock()

	ids := make([]uint64, 0, len(this.addresses))
	for id, _ := range this.addresses {
		ids = append(ids, id)
	}

	return ids
}

func (this *Conn) AddNode(id uint64, address string) {
	this.addressesMu.Lock()
	defer this.addressesMu.Unlock()

	if _, exists := this.addresses[id]; !exists {
		this.addresses[id] = address
	}
}

func (this *Conn) RemoveNode(id uint64) {
	this.addressesMu.Lock()
	defer this.addressesMu.Unlock()
	this.connsMu.Lock()
	defer this.connsMu.Unlock()

	delete(this.addresses, id)
	if conn, exists := this.conns[id]; exists {
		conn.Close()
		delete(this.conns, id)
	}
}

func (this *Conn) Dial(id uint64) (*grpc.ClientConn, error) {
	conn := this.getCachedConn(id)
	if conn != nil {
		return conn, nil
	}
	address, err := this.getAddress(id)
	if err != nil {
		return nil, err
	}

	conn, err = grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	this.connsMu.Lock()
	defer this.connsMu.Unlock()
	if existingConn, exists := this.conns[id]; exists {
		conn.Close()
		return existingConn, nil
	}

	this.conns[id] = conn
	return conn, nil
}

func (this *Conn) getCachedConn(id uint64) *grpc.ClientConn {
	this.connsMu.RLock()
	defer this.connsMu.RUnlock()

	if conn, exists := this.conns[id]; exists {
		return conn
	}
	return nil
}

func (this *Conn) getAddress(id uint64) (string, error) {
	this.addressesMu.RLock()
	defer this.addressesMu.RUnlock()

	if address, exists := this.addresses[id]; exists {
		return address, nil
	}

	return "", NodeAddressNotFoundError
}