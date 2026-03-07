package audiosource

import (
	"fmt"

	"github.com/drgolem/musiclab/internal/decoders"
)

type AudioSamples struct {
	Audio      []float64
	SampleRate int
}

// AudioSamplesFromFile decodes an audio file and returns mono float64 samples.
// Stereo audio is mixed to mono by averaging channels.
// Samples are normalized to [-1.0, 1.0] range.
// Supports 16-bit, 24-bit, and 32-bit audio.
func AudioSamplesFromFile(fileName string) (AudioSamples, error) {
	var out AudioSamples

	dec, err := decoders.NewDecoder(fileName)
	if err != nil {
		return out, fmt.Errorf("failed to open %s: %w", fileName, err)
	}
	defer dec.Close()

	sampleRate, channels, bitsPerSample := dec.GetFormat()

	audioData, _, err := decoders.DecodeAll(dec)
	if err != nil {
		return out, fmt.Errorf("failed to decode %s: %w", fileName, err)
	}

	bytesPerSample := bitsPerSample / 8
	frameSize := channels * bytesPerSample
	numFrames := len(audioData) / frameSize

	audioSamples := make([]float64, numFrames)

	for i := 0; i < numFrames; i++ {
		offset := i * frameSize
		var monoSum int64
		for ch := 0; ch < channels; ch++ {
			sampleOffset := offset + ch*bytesPerSample
			var sample int32
			switch bytesPerSample {
			case 2:
				sample = int32(int16(audioData[sampleOffset]) | int16(audioData[sampleOffset+1])<<8)
			case 3:
				// 24-bit little-endian, sign-extend from 24 to 32 bits
				sample = int32(audioData[sampleOffset]) | int32(audioData[sampleOffset+1])<<8 | int32(audioData[sampleOffset+2])<<16
				if sample&0x800000 != 0 {
					sample |= ^0xFFFFFF // sign extend
				}
			case 4:
				sample = int32(audioData[sampleOffset]) | int32(audioData[sampleOffset+1])<<8 |
					int32(audioData[sampleOffset+2])<<16 | int32(audioData[sampleOffset+3])<<24
			default:
				return out, fmt.Errorf("unsupported bits per sample: %d", bitsPerSample)
			}
			monoSum += int64(sample)
		}

		// Average channels and normalize to [-1.0, 1.0]
		// Chromaprint expects int16 input, so we scale to int16 range
		mono := monoSum / int64(channels)
		switch bytesPerSample {
		case 2:
			audioSamples[i] = float64(mono) / 32768.0
		case 3:
			audioSamples[i] = float64(mono) / 8388608.0 // 2^23
		case 4:
			audioSamples[i] = float64(mono) / 2147483648.0 // 2^31
		}
	}

	out.Audio = audioSamples
	out.SampleRate = sampleRate
	return out, nil
}
