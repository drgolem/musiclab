package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_PeaksHash(t *testing.T) {

	peaks := []float64{1, 2, 3, 4}

	h := peaksHash(peaks)

	// 400 300 200 1
	// 4002002000
	assert.Equal(t, int64(4002002000), h)
}
