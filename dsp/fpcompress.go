package dsp

import (
	"encoding/base64"
	"fmt"
)

// Chromaprint fingerprint encoding/decoding.
// Matches the reference C++ implementation: fingerprint_compressor.cpp / fingerprint_decompressor.cpp
//
// Format:
// 1. For each subfingerprint (first one raw, rest XORed with previous):
//    - Find set bits, encode as run-length of inter-bit gaps
//    - Gaps ≤ 6 stored as 3-bit "normal" values
//    - Gaps > 6 stored as normal=7 + 5-bit "exceptional" overflow
//    - A zero terminates each subfingerprint
// 2. Pack normal values as int3 array, exceptional values as int5 array
// 3. Prepend 4-byte header: [algorithm, size_hi, size_mid, size_lo]
// 4. Base64 encode

const (
	fpAlgorithm      = 1 // algorithm 2 is stored as value 1 (0-indexed)
	kMaxNormalValue   = 7 // (1 << 3) - 1
)

// EncodeFingerprint encodes subfingerprints into a Chromaprint-compatible base64 string.
func EncodeFingerprint(subfingerprints []uint32) (string, error) {
	if len(subfingerprints) == 0 {
		return "", nil
	}

	var normalBits []byte
	var exceptionalBits []byte

	processSubfingerprint := func(x uint32) {
		bit := 1
		lastBit := 0
		for x != 0 {
			if x&1 != 0 {
				value := bit - lastBit
				if value >= kMaxNormalValue {
					normalBits = append(normalBits, byte(kMaxNormalValue))
					exceptionalBits = append(exceptionalBits, byte(value-kMaxNormalValue))
				} else {
					normalBits = append(normalBits, byte(value))
				}
				lastBit = bit
			}
			x >>= 1
			bit++
		}
		normalBits = append(normalBits, 0) // terminator
	}

	processSubfingerprint(subfingerprints[0])
	for i := 1; i < len(subfingerprints); i++ {
		processSubfingerprint(subfingerprints[i] ^ subfingerprints[i-1])
	}

	n := len(subfingerprints)
	packedNormal := packInt3Array(normalBits)
	packedExceptional := packInt5Array(exceptionalBits)

	result := make([]byte, 0, 4+len(packedNormal)+len(packedExceptional))
	result = append(result, byte(fpAlgorithm))
	result = append(result, byte((n>>16)&0xFF))
	result = append(result, byte((n>>8)&0xFF))
	result = append(result, byte(n&0xFF))
	result = append(result, packedNormal...)
	result = append(result, packedExceptional...)

	return base64.RawURLEncoding.EncodeToString(result), nil
}

// DecodeFingerprint decodes a Chromaprint fingerprint string to subfingerprints.
func DecodeFingerprint(encoded string) ([]uint32, error) {
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		data, err = base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("invalid base64: %w", err)
		}
	}

	if len(data) < 4 {
		return nil, fmt.Errorf("fingerprint data too short")
	}

	// algorithm := data[0]
	n := int(data[1])<<16 | int(data[2])<<8 | int(data[3])
	if n <= 0 || n > 1_000_000 {
		return nil, fmt.Errorf("invalid fingerprint length: %d", n)
	}

	offset := 4

	// Unpack normal bits (int3 array)
	bits := unpackInt3Array(data[offset:])

	// Find terminator positions and count exceptional values
	foundValues := 0
	numExceptional := 0
	var bitsLen int
	for i := 0; i < len(bits); i++ {
		if bits[i] == 0 {
			foundValues++
			if foundValues == n {
				bitsLen = i + 1
				break
			}
		} else if bits[i] == kMaxNormalValue {
			numExceptional++
		}
	}

	if foundValues != n {
		return nil, fmt.Errorf("invalid fingerprint: not enough data")
	}

	bits = bits[:bitsLen]

	// Unpack exceptional bits (int5 array)
	normalPackedSize := packedInt3Size(bitsLen)
	offset += normalPackedSize

	if numExceptional > 0 && offset < len(data) {
		exceptional := unpackInt5Array(data[offset:])
		j := 0
		for i := 0; i < len(bits); i++ {
			if bits[i] == kMaxNormalValue && j < len(exceptional) {
				bits[i] += exceptional[j]
				j++
			}
		}
	}

	// Reconstruct subfingerprints
	output := make([]uint32, n)
	idx := 0
	lastBit := 0
	var value uint32
	for _, b := range bits {
		if b == 0 {
			output[idx] = value
			// XOR decode
			if idx > 0 {
				output[idx] ^= output[idx-1]
			}
			lastBit = 0
			value = 0
			idx++
		} else {
			lastBit += int(b)
			value ^= 1 << uint(lastBit-1)
		}
	}

	return output, nil
}

// packInt3Array packs an array of 3-bit values into bytes.
// Matches Chromaprint's PackInt3Array in pack_int3_array.h.
func packInt3Array(src []byte) []byte {
	size := len(src)
	outSize := (size*3 + 7) / 8
	out := make([]byte, outSize)

	i := 0
	o := 0
	for size >= 8 {
		out[o] = (src[i] & 0x07) | ((src[i+1] & 0x07) << 3) | ((src[i+2] & 0x03) << 6)
		out[o+1] = ((src[i+2] & 0x04) >> 2) | ((src[i+3] & 0x07) << 1) | ((src[i+4] & 0x07) << 4) | ((src[i+5] & 0x01) << 7)
		out[o+2] = ((src[i+5] & 0x06) >> 1) | ((src[i+6] & 0x07) << 2) | ((src[i+7] & 0x07) << 5)
		i += 8
		o += 3
		size -= 8
	}

	switch size {
	case 7:
		out[o] = (src[i] & 0x07) | ((src[i+1] & 0x07) << 3) | ((src[i+2] & 0x03) << 6)
		out[o+1] = ((src[i+2] & 0x04) >> 2) | ((src[i+3] & 0x07) << 1) | ((src[i+4] & 0x07) << 4) | ((src[i+5] & 0x01) << 7)
		out[o+2] = ((src[i+5] & 0x06) >> 1) | ((src[i+6] & 0x07) << 2)
	case 6:
		out[o] = (src[i] & 0x07) | ((src[i+1] & 0x07) << 3) | ((src[i+2] & 0x03) << 6)
		out[o+1] = ((src[i+2] & 0x04) >> 2) | ((src[i+3] & 0x07) << 1) | ((src[i+4] & 0x07) << 4) | ((src[i+5] & 0x01) << 7)
		out[o+2] = ((src[i+5] & 0x06) >> 1)
	case 5:
		out[o] = (src[i] & 0x07) | ((src[i+1] & 0x07) << 3) | ((src[i+2] & 0x03) << 6)
		out[o+1] = ((src[i+2] & 0x04) >> 2) | ((src[i+3] & 0x07) << 1) | ((src[i+4] & 0x07) << 4)
	case 4:
		out[o] = (src[i] & 0x07) | ((src[i+1] & 0x07) << 3) | ((src[i+2] & 0x03) << 6)
		out[o+1] = ((src[i+2] & 0x04) >> 2) | ((src[i+3] & 0x07) << 1)
	case 3:
		out[o] = (src[i] & 0x07) | ((src[i+1] & 0x07) << 3) | ((src[i+2] & 0x03) << 6)
		out[o+1] = ((src[i+2] & 0x04) >> 2)
	case 2:
		out[o] = (src[i] & 0x07) | ((src[i+1] & 0x07) << 3)
	case 1:
		out[o] = (src[i] & 0x07)
	}

	return out
}

// packInt5Array packs an array of 5-bit values into bytes.
func packInt5Array(src []byte) []byte {
	size := len(src)
	outSize := (size*5 + 7) / 8
	out := make([]byte, outSize)

	i := 0
	o := 0
	for size >= 8 {
		out[o] = (src[i] & 0x1f) | ((src[i+1] & 0x07) << 5)
		out[o+1] = ((src[i+1] & 0x18) >> 3) | ((src[i+2] & 0x1f) << 2) | ((src[i+3] & 0x01) << 7)
		out[o+2] = ((src[i+3] & 0x1e) >> 1) | ((src[i+4] & 0x0f) << 4)
		out[o+3] = ((src[i+4] & 0x10) >> 4) | ((src[i+5] & 0x1f) << 1) | ((src[i+6] & 0x03) << 6)
		out[o+4] = ((src[i+6] & 0x1c) >> 2) | ((src[i+7] & 0x1f) << 3)
		i += 8
		o += 5
		size -= 8
	}

	switch size {
	case 7:
		out[o] = (src[i] & 0x1f) | ((src[i+1] & 0x07) << 5)
		out[o+1] = ((src[i+1] & 0x18) >> 3) | ((src[i+2] & 0x1f) << 2) | ((src[i+3] & 0x01) << 7)
		out[o+2] = ((src[i+3] & 0x1e) >> 1) | ((src[i+4] & 0x0f) << 4)
		out[o+3] = ((src[i+4] & 0x10) >> 4) | ((src[i+5] & 0x1f) << 1) | ((src[i+6] & 0x03) << 6)
		out[o+4] = ((src[i+6] & 0x1c) >> 2)
	case 6:
		out[o] = (src[i] & 0x1f) | ((src[i+1] & 0x07) << 5)
		out[o+1] = ((src[i+1] & 0x18) >> 3) | ((src[i+2] & 0x1f) << 2) | ((src[i+3] & 0x01) << 7)
		out[o+2] = ((src[i+3] & 0x1e) >> 1) | ((src[i+4] & 0x0f) << 4)
		out[o+3] = ((src[i+4] & 0x10) >> 4) | ((src[i+5] & 0x1f) << 1)
	case 5:
		out[o] = (src[i] & 0x1f) | ((src[i+1] & 0x07) << 5)
		out[o+1] = ((src[i+1] & 0x18) >> 3) | ((src[i+2] & 0x1f) << 2) | ((src[i+3] & 0x01) << 7)
		out[o+2] = ((src[i+3] & 0x1e) >> 1) | ((src[i+4] & 0x0f) << 4)
		out[o+3] = ((src[i+4] & 0x10) >> 4)
	case 4:
		out[o] = (src[i] & 0x1f) | ((src[i+1] & 0x07) << 5)
		out[o+1] = ((src[i+1] & 0x18) >> 3) | ((src[i+2] & 0x1f) << 2) | ((src[i+3] & 0x01) << 7)
		out[o+2] = ((src[i+3] & 0x1e) >> 1)
	case 3:
		out[o] = (src[i] & 0x1f) | ((src[i+1] & 0x07) << 5)
		out[o+1] = ((src[i+1] & 0x18) >> 3) | ((src[i+2] & 0x1f) << 2)
	case 2:
		out[o] = (src[i] & 0x1f) | ((src[i+1] & 0x07) << 5)
		out[o+1] = ((src[i+1] & 0x18) >> 3)
	case 1:
		out[o] = (src[i] & 0x1f)
	}

	return out
}

func packedInt3Size(n int) int {
	return (n*3 + 7) / 8
}

// unpackInt3Array unpacks 3-bit packed values.
func unpackInt3Array(data []byte) []byte {
	outSize := len(data) * 8 / 3
	out := make([]byte, outSize)

	i := 0
	o := 0
	remaining := len(data)
	for remaining >= 3 && o+7 < outSize {
		out[o] = data[i] & 0x07
		out[o+1] = (data[i] >> 3) & 0x07
		out[o+2] = ((data[i] >> 6) & 0x03) | ((data[i+1] & 0x01) << 2)
		out[o+3] = (data[i+1] >> 1) & 0x07
		out[o+4] = (data[i+1] >> 4) & 0x07
		out[o+5] = ((data[i+1] >> 7) & 0x01) | ((data[i+2] & 0x03) << 1)
		out[o+6] = (data[i+2] >> 2) & 0x07
		out[o+7] = (data[i+2] >> 5) & 0x07
		i += 3
		o += 8
		remaining -= 3
	}

	// Handle remaining bytes
	if remaining > 0 && o < outSize {
		var tmp [3]byte
		copy(tmp[:], data[i:i+remaining])
		for o < outSize {
			bitOffset := (o % 8) * 3
			byteIdx := bitOffset / 8
			bitPos := bitOffset % 8
			val := (tmp[byteIdx] >> uint(bitPos)) & 0x07
			if bitPos > 5 && byteIdx+1 < 3 {
				val |= (tmp[byteIdx+1] << uint(8-bitPos)) & 0x07
			}
			out[o] = val
			o++
		}
	}

	return out
}

// unpackInt5Array unpacks 5-bit packed values.
func unpackInt5Array(data []byte) []byte {
	outSize := len(data) * 8 / 5
	out := make([]byte, outSize)

	i := 0
	o := 0
	remaining := len(data)
	for remaining >= 5 && o+7 < outSize {
		out[o] = data[i] & 0x1f
		out[o+1] = ((data[i] >> 5) & 0x07) | ((data[i+1] & 0x03) << 3)
		out[o+2] = (data[i+1] >> 2) & 0x1f
		out[o+3] = ((data[i+1] >> 7) & 0x01) | ((data[i+2] & 0x0f) << 1)
		out[o+4] = ((data[i+2] >> 4) & 0x0f) | ((data[i+3] & 0x01) << 4)
		out[o+5] = (data[i+3] >> 1) & 0x1f
		out[o+6] = ((data[i+3] >> 6) & 0x03) | ((data[i+4] & 0x07) << 2)
		out[o+7] = (data[i+4] >> 3) & 0x1f
		i += 5
		o += 8
		remaining -= 5
	}

	// Handle remaining
	if remaining > 0 && o < outSize {
		var tmp [5]byte
		copy(tmp[:], data[i:i+remaining])
		for o < outSize {
			bitOffset := (o % 8) * 5
			byteIdx := bitOffset / 8
			bitPos := bitOffset % 8
			val := (tmp[byteIdx] >> uint(bitPos)) & 0x1f
			if bitPos > 3 && byteIdx+1 < 5 {
				val |= (tmp[byteIdx+1] << uint(8-bitPos)) & 0x1f
			}
			out[o] = val
			o++
		}
	}

	return out
}
