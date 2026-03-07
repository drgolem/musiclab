package dsp

import (
	"math"
)

// Polyphase sinc resampler matching Chromaprint's internal avresample.
// Parameters match chromaprint/src/audio_processor.cpp:
//   filter_length=16, phase_shift=8, linear=0, cutoff=0.8
// Window: Kaiser with beta=9 (WINDOW_TYPE=9)

const (
	resampleFilterLength = 16
	resamplePhaseShift   = 8
	resamplePhaseCount   = 1 << resamplePhaseShift // 256
	resampleCutoff       = 0.8
	resampleKaiserBeta   = 9.0
	resampleFilterShift  = 15
)

// bessel computes the 0th order modified Bessel function of the first kind.
func bessel(x float64) float64 {
	v := 1.0
	lastv := 0.0
	t := 1.0
	x = x * x / 4
	for i := 1; v != lastv; i++ {
		lastv = v
		t *= x / float64(i*i)
		v += t
	}
	return v
}

// buildFilterBank builds the polyphase filter bank for the sinc resampler.
// Returns int16 filter coefficients matching the C implementation.
func buildFilterBank(factor float64) ([]int16, int) {
	if factor > 1.0 {
		factor = 1.0
	}

	filterLen := int(math.Ceil(float64(resampleFilterLength) / factor))
	if filterLen < 1 {
		filterLen = 1
	}

	bank := make([]int16, filterLen*(resamplePhaseCount+1))
	tab := make([]float64, filterLen)
	center := (filterLen - 1) / 2
	scale := float64(int(1) << resampleFilterShift)

	for ph := 0; ph < resamplePhaseCount; ph++ {
		norm := 0.0
		for i := 0; i < filterLen; i++ {
			x := math.Pi * (float64(i-center) - float64(ph)/float64(resamplePhaseCount)) * factor
			var y float64
			if x == 0 {
				y = 1.0
			} else {
				y = math.Sin(x) / x
			}

			// Kaiser window (type=9 means beta=9)
			w := 2.0 * x / (factor * float64(filterLen) * math.Pi)
			arg := 1 - w*w
			if arg < 0 {
				arg = 0
			}
			y *= bessel(resampleKaiserBeta * math.Sqrt(arg))

			tab[i] = y
			norm += y
		}

		// Normalize and quantize to int16
		for i := 0; i < filterLen; i++ {
			v := math.Round(tab[i] * scale / norm)
			if v > math.MaxInt16 {
				v = math.MaxInt16
			} else if v < math.MinInt16 {
				v = math.MinInt16
			}
			bank[ph*filterLen+i] = int16(v)
		}
	}

	// Copy wraparound for interpolation
	copy(bank[filterLen*resamplePhaseCount+1:], bank[:filterLen-1])
	bank[filterLen*resamplePhaseCount] = bank[filterLen-1]

	return bank, filterLen
}

// Resample converts audio samples from one sample rate to another using a
// polyphase sinc resampler matching Chromaprint's internal avresample.
// Input and output are float64 in [-1, 1] range.
func Resample(samples []float64, fromRate, toRate int) []float64 {
	if fromRate == toRate {
		return samples
	}

	// Convert to int16 (matching Chromaprint's int16 processing)
	srcLen := len(samples)
	src := make([]int16, srcLen)
	for i, v := range samples {
		val := math.Round(v * 32768.0)
		if val > 32767 {
			val = 32767
		} else if val < -32768 {
			val = -32768
		}
		src[i] = int16(val)
	}

	factor := math.Min(float64(toRate)*resampleCutoff/float64(fromRate), 1.0)
	filterBank, filterLen := buildFilterBank(factor)

	srcIncr := toRate
	dstIncr := fromRate * resamplePhaseCount
	index := -resamplePhaseCount * ((filterLen - 1) / 2)
	frac := 0

	dstIncrFrac := dstIncr % srcIncr
	dstIncrInt := dstIncr / srcIncr

	// Estimate output size
	maxOut := int(float64(srcLen)*float64(toRate)/float64(fromRate)) + 256
	dst := make([]int16, 0, maxOut)

	for {
		sampleIndex := index >> resamplePhaseShift
		if sampleIndex+filterLen > srcLen {
			break
		}

		filter := filterBank[(index&(resamplePhaseCount-1))*filterLen:]
		var val int32

		if sampleIndex < 0 {
			for i := 0; i < filterLen; i++ {
				idx := sampleIndex + i
				if idx < 0 {
					idx = -idx
				}
				idx = idx % srcLen
				val += int32(src[idx]) * int32(filter[i])
			}
		} else {
			for i := 0; i < filterLen; i++ {
				val += int32(src[sampleIndex+i]) * int32(filter[i])
			}
		}

		// Round and shift
		val = (val + (1 << (resampleFilterShift - 1))) >> resampleFilterShift
		// Clip
		if uint32(val+32768) > 65535 {
			if val < 0 {
				val = -32768
			} else {
				val = 32767
			}
		}
		dst = append(dst, int16(val))

		frac += dstIncrFrac
		index += dstIncrInt
		if frac >= srcIncr {
			frac -= srcIncr
			index++
		}
	}

	// Convert back to float64
	out := make([]float64, len(dst))
	for i, v := range dst {
		out[i] = float64(v) / 32768.0
	}
	return out
}
