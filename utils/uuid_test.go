package utils

import (
	"math/rand";
    "encoding/binary";
    "testing";

    "github.com/stretchr/testify/assert";

    "github.com/satori/go.uuid";
)

func TestUuidMod(t *testing.T) {
	p1 := rand.Uint64()
	p2 := rand.Uint64()
	mod := uint64(1 + rand.Intn(1024))

	uuidBytes := make([]byte, 16)
	binary.LittleEndian.PutUint64(uuidBytes[:8], p1)
	binary.LittleEndian.PutUint64(uuidBytes[8:], p2)
	uuid := uuid.Must(uuid.FromBytes(uuidBytes))

	assert.Equal(t, ((p1 % mod) + (p2 % mod)) % mod, UuidMod(uuid, mod))
}