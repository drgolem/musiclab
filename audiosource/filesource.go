package audiosource

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sync"
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
	FramesPerBuffer     int
	Start               time.Duration
	Duration            time.Duration
	ProducerContextData string
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

func WithContextData(data string) SetOptionsFn {
	return func(opt *ProducerOptions) {
		opt.ProducerContextData = data
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

	mxStatus       sync.RWMutex
	elapsedSamples int

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

		fmt.Printf("Decoder: %s\n", mp3Decoder.CurrentDecoder())
		decoder = mp3Decoder
		closeFn = func() error {
			decoder.Close()
			mp3Decoder.Delete()
			return nil
		}
		seekFunc = mp3Decoder.Seek
	case types.FileFormat_OGG:
		streamType, err := decoders.GetOggFileStreamType(fileName)
		if err != nil {
			return nil, err
		}
		fmt.Printf("file %s, stream type: %v\n", fileName, streamType)
		if streamType == decoders.StreamType_Vorbis {
			vorbisDecoder, err := decoders.NewOggVorbisDecoder()
			if err != nil {
				return nil, err
			}
			decoder = vorbisDecoder
		} else if streamType == decoders.StreamType_Opus {
			//opusDecoder, err := decoders.NewOggOpusDecoder()
			opusDecoder, err := decoders.NewOggOpusFileDecoder()
			if err != nil {
				return nil, err
			}
			decoder = opusDecoder
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

	if decoder == nil {
		return nil, fmt.Errorf("unknown decoder")
	}
	err := decoder.Open(fileName)
	if err != nil {
		return nil, err
	}

	sampleRate, numChannels, bitsPerSample := decoder.GetFormat()
	if bitsPerSample == 24 {
		bitsPerSample = 16
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
		outSamplesCnt := int(opt.Duration.Seconds() * float64(audioStream.audioFormat.SampleRate))
		samplesPos := 0
		samplesCnt := 0

		if startSamplesPos > 0 && audioStream.seekFunc != nil {
			_, err := audioStream.seekFunc(int64(startSamplesPos), io.SeekCurrent)
			if err != nil {
				fmt.Printf("ERR seek %v\n", err)
				return
			}
			samplesPos = startSamplesPos
		}

		audioStream.mxStatus.Lock()
		audioStream.elapsedSamples = samplesCnt
		audioStream.mxStatus.Unlock()

		for {
			framesPerBuffer := opt.FramesPerBuffer
			audioBufSize := 4 * numChannels * framesPerBuffer
			audio := make([]byte, audioBufSize)
			audioStream.mx.Lock()
			if audioStream.decoder == nil {
				audioStream.mx.Unlock()
				close(audioPacketStream)
				break
			}
			nSamples, err := audioStream.decoder.DecodeSamples(framesPerBuffer, audio)
			audioStream.mx.Unlock()
			if nSamples == 0 {
				// done reading audio, close output channel
				close(audioPacketStream)
				break
			}
			if err != nil {
				fmt.Printf("ERR: %v\n", err)
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
				//audioPacketStream <- pct
				select {
				case audioPacketStream <- pct:
				case <-ctx.Done():
					fmt.Println("context done in MusicAudioProducer")
					close(audioPacketStream)
					return
				case <-audioStream.done:
					close(audioPacketStream)
					return
				}
				samplesCnt += pct.SamplesCount
			}

			samplesPos += pct.SamplesCount

			audioStream.mxStatus.Lock()
			audioStream.elapsedSamples = samplesCnt
			audioStream.mxStatus.Unlock()

			if outSamplesCnt > 0 && samplesCnt >= outSamplesCnt {
				close(audioPacketStream)
				return
			}

			select {
			case <-ctx.Done():
				fmt.Println("context done in MusicAudioProducer")
				close(audioPacketStream)
				return
			case <-audioStream.done:
				close(audioPacketStream)
				return
			default:
			}
		}
		fmt.Println("exit MusicAudioProducer")
	}(ctx)

	return &audioStream, nil
}

func (s *fileAudioStream) GetFormat() types.FrameFormat {
	return s.audioFormat
}

func (s *fileAudioStream) Status() map[string]string {
	s.mxStatus.RLock()
	defer s.mxStatus.RUnlock()

	attrs := make(map[string]string)

	attrs["format"] = fmt.Sprintf("%d:%d:%d",
		s.audioFormat.SampleRate,
		s.audioFormat.BitsPerSample,
		s.audioFormat.Channels,
	)

	attrs["elapsed_samples"] = fmt.Sprintf("%d", s.elapsedSamples)

	elapsed := float64(s.elapsedSamples) / float64(s.audioFormat.SampleRate)

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
	defer s.mx.Unlock()

	if s.closeFunc != nil {
		return s.closeFunc()
	}

	s.decoder = nil

	s.done <- true
	return nil
}
