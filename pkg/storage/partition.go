package storage

import (
	"github.com/satori/go.uuid";
)

type partition struct {
	id uuid.UUID
}

func newPartition(id uuid.UUID) *partition {
	return &partition {
		id: id,
	}
}