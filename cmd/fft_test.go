package cmd

import (
	"testing"

	"github.com/drgolem/musiclab/dsp"
	"github.com/stretchr/testify/assert"
)

func Test_PeaksHash(t *testing.T) {

	peaks := []float64{1, 2, 3, 4}

	h := dsp.PeaksHash(peaks)

	// 400 300 200 1
	// 4002002000
	assert.Equal(t, uint64(4002002000), h)
}
