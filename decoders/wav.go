package decoders

import (
	"os"

	"github.com/drgolem/ringbuffer"
	"github.com/youpy/go-wav"
)

type wavDecoder struct {
	file   *os.File
	reader *wav.Reader

	ringBuffer ringbuffer.RingBuffer
	channels   int
	samplesReq int
}

func NewWavDecoder() (*wavDecoder, error) {
	wd := wavDecoder{}
	return &wd, nil
}

func (wd *wavDecoder) GetFormat() (int, int, int) {
	if wd.reader == nil {
		return 0, 0, 0
	}

	ft, _ := wd.reader.Format()
	return int(ft.SampleRate), int(ft.NumChannels), int(ft.BitsPerSample)
}

func (wd *wavDecoder) DecodeSamples(samples int, audio []byte) (int, error) {
	if wd.reader == nil {
		return 0, nil
	}

	outputBytesPerSample := 2
	var b16 [2]byte
	for {
		sampleBytes := wd.ringBuffer.Size()
		samplesAvail := sampleBytes / (wd.channels * outputBytesPerSample)
		if samplesAvail >= samples {
			bytesRequest := samples * wd.channels * outputBytesPerSample
			bytesRead, err := wd.ringBuffer.Read(bytesRequest, audio)
			if err != nil {
				return 0, err
			}
			samplesRead := bytesRead / (wd.channels * outputBytesPerSample)
			return samplesRead, nil
		}

		smData, err := wd.reader.ReadSamples(uint32(samples))
		if err != nil {
			return 0, err
		}

		nSamples := len(smData)

		for idx := 0; idx < nSamples; {
			for j := 0; j < 2; j++ {
				sv := smData[idx].Values[j]

				b16[0] = byte(sv & 0xFF)
				b16[1] = byte(sv >> 8)
				wd.ringBuffer.Write(b16[:2])
			}

			idx += 1
		}
	}
}

func (wd *wavDecoder) Open(fileName string) error {
	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	wd.file = file
	wd.reader = wav.NewReader(wd.file)

	ft, err := wd.reader.Format()
	if err != nil {
		return err
	}

	wd.samplesReq = 4096
	wd.channels = int(ft.NumChannels)
	wd.ringBuffer = ringbuffer.NewRingBuffer(2 * wd.channels * wd.samplesReq)

	return nil
}

func (wd *wavDecoder) Close() error {
	if wd.file != nil {
		return wd.file.Close()
	}
	return nil
}
