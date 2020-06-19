package cluster

import (
	"fmt";
	"sync";
	"errors";

	"google.golang.org/grpc";
	"google.golang.org/grpc/credentials";
	log "github.com/sirupsen/logrus";
)

var (
	NodeAddressNotFoundError error = errors.New("Node address not found")
)

type nodesChangeType uint

const (
	NodesChangeAddNode nodesChangeType = iota
	NodesChangeRemoveNode
)

type nodesChange struct {
	Type nodesChangeType
	NodeId uint64
}

type Conn struct {
	id uint64
	address string
	addresses map[uint64]string
	addressesMu sync.RWMutex
	conns map[uint64]*grpc.ClientConn
	connsMu sync.RWMutex
	notifications []chan *nodesChange
	notificationsMu *sync.RWMutex

	transportCredentials credentials.TransportCredentials

	log *log.Entry
}

func NewConn(id uint64, address string, tlsCertFile string) (*Conn, error) {
	c := &Conn {
		id: id,
		address: address,
		addresses: make(map[uint64]string),
		addressesMu: sync.RWMutex{},
		conns: make(map[uint64]*grpc.ClientConn),
		connsMu: sync.RWMutex{},
		notifications: make([]chan *nodesChange, 0),
		notificationsMu: &sync.RWMutex{},
		log: log.WithFields(log.Fields {
			"node_id": fmt.Sprintf("%16x", id),
		}),
	}

	if tlsCertFile != "" {
		var err error
		c.transportCredentials, err = credentials.NewClientTLSFromFile(tlsCertFile, "anndb-server")
		if err != nil {
			return nil, err
		}
	}

	return c, nil
}

func (this *Conn) Id() uint64 {
	return this.id
}

func (this *Conn) Address() string {
	return this.address
}

func (this *Conn) Close() {
	this.connsMu.Lock()
	defer this.connsMu.Unlock()

	for _, conn := range this.conns {
		if err := conn.Close(); err != nil {
			log.Error(err)
		}
	}

	this.notificationsMu.Lock()
	defer this.notificationsMu.Unlock()
	for _, c := range this.notifications {
		close(c)
	}
}

func (this *Conn) NodeChangesNotifications() <- chan *nodesChange {
	this.notificationsMu.Lock()
	defer this.notificationsMu.Unlock()

	c := make(chan *nodesChange, 10)
	this.notifications = append(this.notifications, c)
	return c
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
		this.sendNodesChangeNotification(&nodesChange {
			Type: NodesChangeAddNode,
			NodeId: id,
		})
		this.log.Infof("Conn: Added node: %16x", id)
	}
}

func (this *Conn) RemoveNode(id uint64) {
	this.addressesMu.Lock()
	defer this.addressesMu.Unlock()
	this.connsMu.Lock()
	defer this.connsMu.Unlock()

	if _, exists := this.addresses[id]; exists {
		delete(this.addresses, id)
		if conn, exists := this.conns[id]; exists {
			if err := conn.Close(); err != nil {
				log.Error(err)
			}
			delete(this.conns, id)
		}
		this.sendNodesChangeNotification(&nodesChange {
			Type: NodesChangeRemoveNode,
			NodeId: id,
		})
		this.log.Infof("Conn: Removed node: %16x", id)
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

	conn, err = grpc.Dial(address, this.grpcDialOptions()...)
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

func (this *Conn) DialAddress(address string) (*grpc.ClientConn, error) {
	return grpc.Dial(address, this.grpcDialOptions()...)
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

func (this *Conn) grpcDialOptions() []grpc.DialOption {
	options := make([]grpc.DialOption, 0)
	if this.transportCredentials != nil {
		options = append(options, grpc.WithTransportCredentials(this.transportCredentials))
	} else {
		options = append(options, grpc.WithInsecure())
	}
	
	return options
}

func (this *Conn) sendNodesChangeNotification(n *nodesChange) {
	this.notificationsMu.RLock()
	defer this.notificationsMu.RUnlock()

	for _, c := range this.notifications {
		c <- n
	}
}