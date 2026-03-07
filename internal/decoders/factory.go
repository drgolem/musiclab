package decoders

import (
	"fmt"

	"github.com/drgolem/audiokit/pkg/decoder"
	"github.com/drgolem/audiokit/pkg/decoder/flac"
	"github.com/drgolem/audiokit/pkg/decoder/mp3"
	"github.com/drgolem/audiokit/pkg/decoder/opus"
	"github.com/drgolem/audiokit/pkg/decoder/vorbis"
	"github.com/drgolem/audiokit/pkg/decoder/wav"
	"github.com/drgolem/audiokit/pkg/oggutil"
)

// NewRegistry creates a decoder registry pre-loaded with all supported codecs.
// OGG files are auto-detected as Vorbis or Opus based on stream headers.
func NewRegistry(fileName string) *decoder.Registry {
	r := decoder.NewRegistry()
	r.Register(".mp3", func(int) (decoder.AudioDecoder, error) { return mp3.NewDecoder(), nil })
	r.Register(".flac", func(bps int) (decoder.AudioDecoder, error) { return flac.NewDecoder(bps) })
	r.Register(".wav", func(int) (decoder.AudioDecoder, error) { return wav.NewDecoder(), nil })

	r.Register(".ogg", func(bps int) (decoder.AudioDecoder, error) {
		streamType, err := oggutil.GetOggFileStreamType(fileName)
		if err != nil {
			return nil, fmt.Errorf("detecting OGG stream type: %w", err)
		}
		switch streamType {
		case oggutil.StreamTypeVorbis:
			return vorbis.NewDecoder(bps)
		case oggutil.StreamTypeOpus:
			return opus.NewDecoder(), nil
		default:
			return nil, fmt.Errorf("unsupported OGG stream type: %v", streamType)
		}
	})

	return r
}

// NewDecoder creates and opens the appropriate decoder based on file extension.
// Supports .mp3, .flac, .wav, and .ogg (Vorbis/Opus) formats.
func NewDecoder(fileName string) (decoder.AudioDecoder, error) {
	return NewRegistry(fileName).NewFromFile(fileName, 0)
}

// DecodeAll reads all audio data from a decoder into memory.
// Returns raw audio bytes and total sample count.
func DecodeAll(dec decoder.AudioDecoder) ([]byte, int, error) {
	_, channels, bitsPerSample := dec.GetFormat()

	const bufferSamples = 4096
	bytesPerSample := bitsPerSample / 8
	bufferSize := bufferSamples * channels * bytesPerSample

	buffer := make([]byte, bufferSize)
	audioData := make([]byte, 0, bufferSize*10)
	totalSamples := 0

	for {
		samplesRead, err := dec.DecodeSamples(bufferSamples, buffer)
		if samplesRead > 0 {
			bytesRead := samplesRead * channels * bytesPerSample
			audioData = append(audioData, buffer[:bytesRead]...)
			totalSamples += samplesRead
		}
		if err != nil || samplesRead == 0 {
			break
		}
	}

	return audioData, totalSamples, nil
}
