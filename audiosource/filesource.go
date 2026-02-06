package audiosource

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/drgolem/go-flac/flac"
	"github.com/drgolem/go-mpg123/mpg123"

	"github.com/drgolem/musiclab/decoders"
	"github.com/drgolem/musiclab/types"
)

type AudioSamplesPacket struct {
	Format       types.FrameFormat
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

type AudioStream interface {
	GetFormat() types.FrameFormat
	Status() map[string]string
	Stream() <-chan AudioSamplesPacket
	Close() error
}

type fileAudioStream struct {
	audioFormat types.FrameFormat
	stream      <-chan AudioSamplesPacket

	decoder musicDecoder
	mx      sync.Mutex

	elapsedSamples atomic.Int64

	done chan bool

	closeFunc func() error
	seekFunc  func(offset int64, whence int) (int64, error)
}

func NewMusicAudioProducer(ctx context.Context,
	fileName string,
	opts ...SetOptionsFn,
) (AudioStream, error) {
	opt := ProducerOptions{
		FramesPerBuffer: 2048,
	}
	for _, sf := range opts {
		sf(&opt)
	}

	audioPacketStream := make(chan AudioSamplesPacket, 1)

	audioStream := fileAudioStream{
		stream: audioPacketStream,
		done:   make(chan bool, 1),
	}

	ext := filepath.Ext(fileName)
	fileFormat := types.FileFormatType(ext)

	var closeFn func() error
	var seekFunc func(offset int64, whence int) (int64, error)

	var decoder musicDecoder

	switch fileFormat {
	case types.FileFormat_MP3:
		mp3Decoder, err := mpg123.NewDecoder("")
		if err != nil {
			return nil, err
		}

		slog.Info("MP3 decoder initialized", "decoder", mp3Decoder.CurrentDecoder())
		decoder = mp3Decoder
		closeFn = func() error {
			err := decoder.Close()
			mp3Decoder.Delete()
			return err
		}
		seekFunc = mp3Decoder.Seek
	case types.FileFormat_OGG:
		streamType, err := decoders.GetOggFileStreamType(fileName)
		if err != nil {
			return nil, err
		}
		slog.Info("OGG stream type detected", "file", fileName, "stream_type", streamType)
		switch streamType {
		case decoders.StreamType_Vorbis:
			vorbisDecoder, err := decoders.NewOggVorbisDecoder()
			if err != nil {
				return nil, err
			}
			decoder = vorbisDecoder
		case decoders.StreamType_Opus:
			opusDecoder, err := decoders.NewOggOpusDecoder()
			if err != nil {
				return nil, err
			}
			decoder = opusDecoder
		default:
			return nil, fmt.Errorf("unsupported OGG stream type: %v", streamType)
		}
		closeFn = func() error {
			return decoder.Close()
		}
	case types.FileFormat_FLAC:
		flacDecoder, err := flac.NewFlacFrameDecoder(16)
		if err != nil {
			return nil, err
		}
		decoder = flacDecoder
		closeFn = func() error {
			return decoder.Close()
		}
		seekFunc = flacDecoder.Seek
	case types.FileFormat_WAV:
		wavDecoder, err := decoders.NewWavDecoder()
		if err != nil {
			return nil, err
		}
		decoder = wavDecoder
		closeFn = func() error {
			return decoder.Close()
		}
	default:
		return nil, fmt.Errorf("unsupported file format: %s", ext)
	}

	err := decoder.Open(fileName)
	if err != nil {
		return nil, err
	}

	sampleRate, numChannels, bitsPerSample := decoder.GetFormat()
	if bitsPerSample != 16 && bitsPerSample != 8 {
		return nil, fmt.Errorf("unsupported bit depth: %d (only 8 and 16 bit supported)", bitsPerSample)
	}
	audioFormat := types.FrameFormat{
		SampleRate:    sampleRate,
		Channels:      numChannels,
		BitsPerSample: bitsPerSample,
	}

	audioStream.audioFormat = audioFormat
	audioStream.decoder = decoder
	audioStream.closeFunc = closeFn
	audioStream.seekFunc = seekFunc

	go func(ctx context.Context) {
		startSamplesPos := int(opt.Start.Seconds() * float64(audioFormat.SampleRate))
		outSamplesCnt := int(opt.Duration.Seconds() * float64(audioFormat.SampleRate))
		samplesPos := 0
		samplesCnt := 0

		if startSamplesPos > 0 && audioStream.seekFunc != nil {
			_, err := audioStream.seekFunc(int64(startSamplesPos), io.SeekCurrent)
			if err != nil {
				slog.Error("seek failed", "error", err, "position", startSamplesPos)
				close(audioPacketStream)
				return
			}
			samplesPos = startSamplesPos
		}

		audioStream.elapsedSamples.Store(int64(samplesCnt))

		// Allocate buffer once for better performance
		audioBufSize := (bitsPerSample / 8) * numChannels * opt.FramesPerBuffer
		audio := make([]byte, audioBufSize)

		for {
			audioStream.mx.Lock()
			if audioStream.decoder == nil {
				audioStream.mx.Unlock()
				close(audioPacketStream)
				break
			}
			nSamples, err := audioStream.decoder.DecodeSamples(opt.FramesPerBuffer, audio)
			audioStream.mx.Unlock()
			if nSamples == 0 {
				// done reading audio, close output channel
				close(audioPacketStream)
				break
			}
			if err != nil {
				slog.Error("failed to decode samples", "error", err)
				close(audioPacketStream)
				return
			}

			bytesSize := nSamples * audioFormat.Channels * audioFormat.BitsPerSample / 8

			pct := AudioSamplesPacket{
				Format:       audioFormat,
				Audio:        audio[:bytesSize],
				SamplesCount: nSamples,
			}

			skipPacket := false
			if startSamplesPos > samplesPos+pct.SamplesCount {
				skipPacket = true
			}

			if !skipPacket {
				select {
				case audioPacketStream <- pct:
				case <-ctx.Done():
					slog.Info("context cancelled in audio producer")
					close(audioPacketStream)
					return
				case <-audioStream.done:
					close(audioPacketStream)
					return
				}
				samplesCnt += pct.SamplesCount
			}

			samplesPos += pct.SamplesCount

			audioStream.elapsedSamples.Store(int64(samplesCnt))

			if outSamplesCnt > 0 && samplesCnt >= outSamplesCnt {
				close(audioPacketStream)
				return
			}
		}
		slog.Info("audio producer exiting")
	}(ctx)

	return &audioStream, nil
}

func (s *fileAudioStream) GetFormat() types.FrameFormat {
	return s.audioFormat
}

func (s *fileAudioStream) Status() map[string]string {
	elapsedSamples := s.elapsedSamples.Load()

	attrs := make(map[string]string)

	attrs["format"] = fmt.Sprintf("%d:%d:%d",
		s.audioFormat.SampleRate,
		s.audioFormat.BitsPerSample,
		s.audioFormat.Channels,
	)

	attrs["elapsed_samples"] = fmt.Sprintf("%d", elapsedSamples)

	elapsed := float64(elapsedSamples) / float64(s.audioFormat.SampleRate)

	dur := time.Second * time.Duration(elapsed)

	attrs["elapsed_str"] = dur.String()

	attrs["elapsed"] = fmt.Sprintf("%.6f", elapsed)

	return attrs
}

func (s *fileAudioStream) Stream() <-chan AudioSamplesPacket {
	return s.stream
}

func (s *fileAudioStream) Close() error {
	s.mx.Lock()

	var err error
	if s.closeFunc != nil {
		err = s.closeFunc()
	}

	s.decoder = nil
	s.mx.Unlock()

	// Signal goroutine to stop
	select {
	case s.done <- true:
	default:
	}

	return err
}
