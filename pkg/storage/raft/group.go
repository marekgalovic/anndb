package raft

import (
	"github.com/satori/go.uuid"
)

type raftGroup struct {
	id uuid.UUID
}

func (this *raftGroup) reportUnreachable(nodeId uint64) {
	
}