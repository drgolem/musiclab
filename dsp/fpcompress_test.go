package dsp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeFingerprint_RoundTrip(t *testing.T) {
	original := []uint32{
		0x12345678, 0x9ABCDEF0, 0x11111111,
		0xDEADBEEF, 0xCAFEBABE, 0x00000000,
	}

	encoded, err := EncodeFingerprint(original)
	require.NoError(t, err)
	assert.NotEmpty(t, encoded)

	decoded, err := DecodeFingerprint(encoded)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestEncodeDecodeFingerprint_SingleValue(t *testing.T) {
	original := []uint32{42}
	encoded, err := EncodeFingerprint(original)
	require.NoError(t, err)

	decoded, err := DecodeFingerprint(encoded)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestEncodeDecodeFingerprint_Zeros(t *testing.T) {
	original := []uint32{0, 0, 0, 0, 0}
	encoded, err := EncodeFingerprint(original)
	require.NoError(t, err)

	decoded, err := DecodeFingerprint(encoded)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestEncodeDecodeFingerprint_IdenticalValues(t *testing.T) {
	// All identical values -> all deltas are 0 (should compress well)
	original := []uint32{0xAAAAAAAA, 0xAAAAAAAA, 0xAAAAAAAA}
	encoded, err := EncodeFingerprint(original)
	require.NoError(t, err)

	decoded, err := DecodeFingerprint(encoded)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestEncodeFingerprint_Empty(t *testing.T) {
	encoded, err := EncodeFingerprint(nil)
	require.NoError(t, err)
	assert.Empty(t, encoded)
}

func TestDecodeFingerprint_TooShort(t *testing.T) {
	_, err := DecodeFingerprint("AA")
	assert.Error(t, err)
}

func TestEncodeDecodeFingerprint_LargeValues(t *testing.T) {
	original := []uint32{0xFFFFFFFF, 0x00000001, 0xFFFFFFFE}
	encoded, err := EncodeFingerprint(original)
	require.NoError(t, err)

	decoded, err := DecodeFingerprint(encoded)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}
