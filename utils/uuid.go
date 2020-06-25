package utils

import (
	"encoding/binary";

	"github.com/satori/go.uuid";
)

func UuidMod(x uuid.UUID, mod uint64) uint64 {
	res := ((binary.LittleEndian.Uint64(x[:8]) % mod) + (binary.LittleEndian.Uint64(x[8:]) % mod))
    return res % mod;
}