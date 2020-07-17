package index

import (
	"bytes";
    "testing";

    "github.com/stretchr/testify/assert";

    "github.com/satori/go.uuid";
)

func TestMetadataSaveAndLoad(t *testing.T) {
	m := make(Metadata)
	for i := 0; i < 1000; i++ {
		m[uuid.NewV4().String()] = uuid.NewV4().String()
	}

	var buf bytes.Buffer
	err := m.save(&buf)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	nm := make(Metadata)
	err = nm.load(&buf)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	assert.Equal(t, len(m), len(nm))
	for k, v := range m {
		assert.Equal(t, v, nm[k])
	}
}