package audiosource

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/drgolem/go-flac/flac"
	"github.com/drgolem/go-mpg123/mpg123"

	"github.com/drgolem/musiclab/decoders"
	"github.com/drgolem/musiclab/types"
)

type AudioSamplesPacket struct {
	Audio        []byte
	SamplesCount int
}

type musicDecoder interface {
	GetFormat() (int, int, int)
	DecodeSamples(samples int, audio []byte) (int, error)

	Open(fileName string) error
	Close() error
}

type ProducerOptions struct {
	FramesPerBuffer int
	Start           time.Duration
	Duration        time.Duration
}

type SetOptionsFn func(opt *ProducerOptions)

func WithFramesPerBuffer(framesPerBuffer int) SetOptionsFn {
	return func(opt *ProducerOptions) {
		opt.FramesPerBuffer = framesPerBuffer
	}
}

func WithPlayDuration(dur time.Duration) SetOptionsFn {
	return func(opt *ProducerOptions) {
		opt.Duration = dur
	}
}

func WithPlayStartPos(start time.Duration) SetOptionsFn {
	return func(opt *ProducerOptions) {
		opt.Start = start
	}
}

func MusicAudioProducer(ctx context.Context,
	fileName string, opts ...SetOptionsFn) (<-chan AudioSamplesPacket, types.FrameFormat, func() error, error) {

	audioChan := make(chan AudioSamplesPacket)
	var audioFormat types.FrameFormat

	opt := ProducerOptions{
		FramesPerBuffer: 2048,
	}
	for _, sf := range opts {
		sf(&opt)
	}

	closeFn := func() error {
		return nil
	}

	ext := filepath.Ext(fileName)
	fileFormat := types.FileFormatType(ext)

	var decoder musicDecoder

	switch fileFormat {
	case types.FileFormat_MP3:
		mp3Decoder, err := mpg123.NewDecoder("")
		if err != nil {
			return audioChan, audioFormat, closeFn, err
		}

		fmt.Printf("Decoder: %s\n", mp3Decoder.CurrentDecoder())
		decoder = mp3Decoder
		closeFn = func() error {
			decoder.Close()
			mp3Decoder.Delete()
			return nil
		}
	case types.FileFormat_OGG:
		streamType, err := decoders.GetOggFileStreamType(fileName)
		if err != nil {
			return audioChan, audioFormat, closeFn, err
		}
		fmt.Printf("file %s, stream type: %v\n", fileName, streamType)
		if streamType == decoders.StreamType_Vorbis {
			vorbisDecoder, err := decoders.NewOggVorbisDecoder()
			if err != nil {
				return audioChan, audioFormat, closeFn, err
			}
			decoder = vorbisDecoder
		} else if streamType == decoders.StreamType_Opus {
			//opusDecoder, err := decoders.NewOggOpusDecoder()
			opusDecoder, err := decoders.NewOggOpusFileDecoder()
			if err != nil {
				return audioChan, audioFormat, closeFn, err
			}
			decoder = opusDecoder
		}
		closeFn = func() error {
			return decoder.Close()
		}
	case types.FileFormat_FLAC:
		flacDecoder, err := flac.NewFlacFrameDecoder(16)
		if err != nil {
			return audioChan, audioFormat, closeFn, err
		}
		decoder = flacDecoder
		closeFn = func() error {
			return decoder.Close()
		}
	case types.FileFormat_WAV:
		wavDecoder, err := decoders.NewWavDecoder()
		if err != nil {
			return audioChan, audioFormat, closeFn, err
		}
		decoder = wavDecoder
		closeFn = func() error {
			return decoder.Close()
		}
	default:
		return audioChan, audioFormat, closeFn, fmt.Errorf("unsupported file format: %s", ext)
	}

	if decoder == nil {
		return audioChan, audioFormat, closeFn, fmt.Errorf("unknown decoder")
	}
	err := decoder.Open(fileName)
	if err != nil {
		return audioChan, audioFormat, closeFn, err
	}

	sampleRate, numChannels, bitsPerSample := decoder.GetFormat()
	audioFormat = types.FrameFormat{
		SampleRate:    sampleRate,
		Channels:      numChannels,
		BitsPerSample: bitsPerSample,
	}

	go func(ctx context.Context) {
		startSamplesPos := int(opt.Start.Seconds() * float64(audioFormat.SampleRate))
		samplesPos := 0
		for {
			framesPerBuffer := opt.FramesPerBuffer
			audioBufSize := 4 * numChannels * framesPerBuffer
			audio := make([]byte, audioBufSize)
			nSamples, err := decoder.DecodeSamples(framesPerBuffer, audio)
			if nSamples == 0 {
				// done reading audio, close output channel
				close(audioChan)
				break
			}
			if err != nil {
				fmt.Printf("ERR: %v\n", err)
				close(audioChan)
				return
			}

			pct := AudioSamplesPacket{
				Audio:        audio,
				SamplesCount: nSamples,
			}

			skipPacket := false
			if startSamplesPos > samplesPos+pct.SamplesCount {
				samplesPos += pct.SamplesCount
				skipPacket = true
			}

			if !skipPacket {
				audioChan <- pct
			}

			samplesPos += pct.SamplesCount

			select {
			case <-ctx.Done():
				close(audioChan)
				return
			default:
			}
		}
	}(ctx)

	return audioChan, audioFormat, closeFn, nil
}
