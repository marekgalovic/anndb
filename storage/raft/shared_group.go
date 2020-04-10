package raft

import (
	"context";

	pb "github.com/marekgalovic/anndb/protobuf";

	"github.com/golang/protobuf/proto";
)

// Shared group
type sharedGroup struct {
	group *RaftGroup
	proxies map[string]*sharedGroupProxy
}

func NewSharedGroup(group *RaftGroup) (*sharedGroup, error) {
	sg := &sharedGroup {
		group: group,
		proxies: make(map[string]*sharedGroupProxy),
	}

	if err := group.RegisterProcessFn(sg.process); err != nil {
		return nil, err
	}
	if err := group.RegisterProcessSnapshotFn(sg.processSnapshot); err != nil {
		return nil, err
	}
	if err := group.RegisterSnapshotFn(sg.snapshot); err != nil {
		return nil, err
	}
	return sg, nil
}

func (this *sharedGroup) Get(name string) *sharedGroupProxy {
	if proxy, exists := this.proxies[name]; exists {
		return proxy
	}

	this.proxies[name] = newSharedGroupProxy(this.group, name)
	return this.proxies[name]
}

func (this *sharedGroup) process(data []byte) error {
	var proposal pb.SharedGroupProposal
	if err := proto.Unmarshal(data, &proposal); err != nil {
		return err
	}

	if proxy, exists := this.proxies[proposal.GetProxyName()]; exists {
		return proxy.processFn(proposal.GetData())
	}
	return nil
}

func (this *sharedGroup) processSnapshot(data []byte) error {
	var snapshot pb.SharedGroupSnapshot
	if err := proto.Unmarshal(data, &snapshot); err != nil {
		return err
	}

	for proxyName, proxySnapshot := range snapshot.GetProxySnapshots() {
		proxy := this.proxies[proxyName]
		if err := proxy.processSnapshotFn(proxySnapshot); err != nil {
			return err
		}
	}
	return nil
}

func (this *sharedGroup) snapshot() ([]byte, error) {
	var err error
	proxySnapshots := make(map[string][]byte)
	for _, proxy := range this.proxies {
		if proxy.snapshotFn != nil {
			proxySnapshots[proxy.name], err = proxy.snapshotFn()
			if err != nil {
				return nil, err
			}
		}
	}

	return proto.Marshal(&pb.SharedGroupSnapshot{ProxySnapshots: proxySnapshots})
}

// Shared group proxy
type sharedGroupProxy struct {
	name string
	group *RaftGroup

	processFn ProcessFn
	processSnapshotFn ProcessFn
	snapshotFn SnapshotFn
}

func newSharedGroupProxy(group *RaftGroup, name string) *sharedGroupProxy {
	return &sharedGroupProxy {
		name: name,
		group: group,
		processFn: nil,
		processSnapshotFn: nil,
		snapshotFn: nil,
	}
}

func (this *sharedGroupProxy) RegisterProcessFn(fn ProcessFn) error {
	if this.processFn != nil {
		return ProcessFnAlreadyRegisteredErr
	}
	this.processFn = fn
	return nil
}

func (this *sharedGroupProxy) RegisterProcessSnapshotFn(fn ProcessFn) error {
	if this.processSnapshotFn != nil {
		return ProcessFnAlreadyRegisteredErr
	}
	this.processSnapshotFn = fn
	return nil
}

func (this *sharedGroupProxy) RegisterSnapshotFn(fn SnapshotFn) error {
	if this.snapshotFn != nil {
		return SnapshotFnAlreadyRegisteredErr
	}
	this.snapshotFn = fn
	return nil
}

func (this *sharedGroupProxy) LeaderId() uint64 {
	return this.group.LeaderId()
}

func (this *sharedGroupProxy) Propose(ctx context.Context, data []byte) error {
	proposal := &pb.SharedGroupProposal {
		ProxyName: this.name,
		Data: data,
	}
	proposalData, err := proto.Marshal(proposal)
	if err != nil {
		return err
	}
	return this.group.Propose(ctx, proposalData)
}