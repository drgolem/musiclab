package dsp

import (
	"math"
	"math/rand"
)

// WaveFunc generates a waveform sample for a given phase (0.0 to 1.0).
type WaveFunc func(phase float64) float64

// Sine waveform — smooth, pure tone.
func Sine(phase float64) float64 {
	return math.Sin(2 * math.Pi * phase)
}

// Square waveform — hollow, clarinet-like.
func Square(phase float64) float64 {
	if phase < 0.5 {
		return 1.0
	}
	return -1.0
}

// Triangle waveform — soft, recorder-like.
func Triangle(phase float64) float64 {
	if phase < 0.25 {
		return 4.0 * phase
	}
	if phase < 0.75 {
		return 2.0 - 4.0*phase
	}
	return -4.0 + 4.0*phase
}

// Sawtooth waveform — bright, string-like.
func Sawtooth(phase float64) float64 {
	return 2.0*phase - 1.0
}

// WaveFuncByName returns a waveform generator by name.
// Supported: "sine", "square", "triangle", "sawtooth" (or "saw").
func WaveFuncByName(name string) WaveFunc {
	switch name {
	case "square":
		return Square
	case "triangle":
		return Triangle
	case "sawtooth", "saw":
		return Sawtooth
	default:
		return Sine
	}
}

// TrackEvent represents a single note/rest in a multi-track score.
type TrackEvent struct {
	Track    int     // track number (0-based)
	Wave     string  // waveform name
	Note     string  // note name (e.g. "C4") or rest ("-", "rest", "R")
	StartMs  int     // start time in milliseconds
	DurMs    int     // duration in milliseconds
	Velocity float64 // 0.0-1.0, defaults to 1.0 if zero
}

// MixTracks renders multi-track events into a stereo float64 buffer.
// Returns the mixed samples as interleaved stereo (L, R, L, R, ...).
func MixTracks(events []TrackEvent, sampleRate int) []float64 {
	if len(events) == 0 {
		return nil
	}

	// Find total duration
	totalMs := 0
	for _, ev := range events {
		end := ev.StartMs + ev.DurMs
		if end > totalMs {
			totalMs = end
		}
	}

	totalSamples := sampleRate * totalMs / 1000
	// stereo: 2 values per sample
	buf := make([]float64, totalSamples*2)

	for _, ev := range events {
		if ev.Note == "-" || ev.Note == "rest" || ev.Note == "R" || ev.Note == "" {
			continue
		}

		freq := NoteToFrequency(ev.Note)
		if freq <= 0 {
			continue
		}

		vel := ev.Velocity
		if vel <= 0 {
			vel = 1.0
		}

		startSample := sampleRate * ev.StartMs / 1000
		nSamples := sampleRate * ev.DurMs / 1000

		if ev.Wave == "guitar" || ev.Wave == "pluck" {
			// Karplus-Strong plucked string
			ks := KarplusStrong(freq, sampleRate, nSamples, 0.996)
			for i, s := range ks {
				idx := (startSample + i) * 2
				if idx+1 < len(buf) {
					sample := vel * s
					buf[idx] += sample
					buf[idx+1] += sample
				}
			}
			continue
		}

		waveFn := WaveFuncByName(ev.Wave)
		step := freq / float64(sampleRate)

		durSec := float64(ev.DurMs) / 1000.0
		attack := 0.1 * durSec
		decay := 0.3 * durSec
		release := 0.3 * durSec
		sustain := 0.8 * vel

		var phase float64
		for i := 0; i < nSamples; i++ {
			amp := adsr(vel, durSec, attack, decay, sustain, release, float64(sampleRate), i)
			sample := amp * waveFn(phase)

			idx := (startSample + i) * 2
			if idx+1 < len(buf) {
				buf[idx] += sample   // left
				buf[idx+1] += sample // right
			}

			_, phase = math.Modf(phase + step)
		}
	}

	// Normalize if any sample exceeds [-1, 1]
	peak := 0.0
	for _, v := range buf {
		if a := math.Abs(v); a > peak {
			peak = a
		}
	}
	if peak > 1.0 {
		scale := 1.0 / peak
		for i := range buf {
			buf[i] *= scale
		}
	}

	return buf
}

func adsr(maxamp, duration, attacktime, decaytime, sus, releasetime, controlrate float64, frame int) float64 {
	dur := duration * controlrate
	at := attacktime * controlrate
	dt := decaytime * controlrate
	rt := releasetime * controlrate
	cnt := float64(frame)

	if cnt >= dur {
		return 0
	}
	if cnt <= at {
		return cnt * (maxamp / at)
	}
	if cnt <= at+dt {
		return ((sus-maxamp)/dt)*(cnt-at) + maxamp
	}
	if cnt <= dur-rt {
		return sus
	}
	return -(sus/rt)*(cnt-(dur-rt)) + sus
}

// KarplusStrong generates a plucked string sound using the Karplus-Strong algorithm.
// Returns mono float64 samples for the given frequency and duration.
// decay controls damping (0.990=muted, 0.998=ringing). blend mixes noise vs square init.
func KarplusStrong(freq float64, sampleRate, nSamples int, decay float64) []float64 {
	bufLen := int(float64(sampleRate) / freq)
	if bufLen < 2 {
		bufLen = 2
	}

	// Initialize delay line with noise burst
	buf := make([]float64, bufLen)
	for i := range buf {
		buf[i] = rand.Float64()*2 - 1
	}

	samples := make([]float64, nSamples)
	pos := 0
	for i := range samples {
		samples[i] = buf[pos]
		next := (pos + 1) % bufLen
		// Low-pass filter: weighted average of current and next sample
		buf[pos] = decay * 0.5 * (buf[pos] + buf[next])
		pos = next
	}
	return samples
}

// Float64ToInt16Bytes converts interleaved stereo float64 samples to 16-bit PCM bytes.
func Float64ToInt16Bytes(samples []float64) []byte {
	buf := make([]byte, len(samples)*2)
	for i, v := range samples {
		frame := int16(v * 0x7FFF)
		buf[i*2] = byte(frame)
		buf[i*2+1] = byte(frame >> 8)
	}
	return buf
}
