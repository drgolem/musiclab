package decoders

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"

	"github.com/drgolem/go-ogg/ogg"
	"github.com/drgolem/ringbuffer"
	"github.com/jfreymuth/vorbis"
)

type oggVorbisDecoder struct {
	oggReader  oggReader
	decoder    vorbis.Decoder
	file       *os.File
	reader     *bufio.Reader
	ringBuffer ringbuffer.RingBuffer
	channels   int
	samplesReq int
}

func NewOggVorbisDecoder() (*oggVorbisDecoder, error) {
	dec := oggVorbisDecoder{}

	return &dec, nil
}

func (d *oggVorbisDecoder) Open(fileName string) error {
	f, err := os.Open(fileName)
	if err != nil {
		return err
	}
	d.file = f

	d.reader = bufio.NewReader(d.file)

	oggReader, err := ogg.NewOggReader(d.reader)
	if err != nil {
		return err
	}
	d.oggReader = oggReader

	pktCnt := 0
	var streamType StreamType
	streamHeaders := make([][]byte, 0)
	headersCount := 0
	for oggReader.Next() {
		p, err := oggReader.Scan()
		if err != nil {
			return err
		}
		pktCnt++

		if streamType == StreamType_Unknown {
			bytesReader := bytes.NewReader(p)
			var ch VorbisCommonHeader
			err = binary.Read(bytesReader, binary.LittleEndian, &ch)
			if err == nil {
				if ch.PacketType == 1 && ch.VorbisPattern == VorbisPattern {
					streamType = StreamType_Vorbis
				}
			}
			bytesReader = bytes.NewReader(p)
			var coh OpusCommonHeader
			err = binary.Read(bytesReader, binary.LittleEndian, &coh)
			if err == nil {
				if coh.OpusPattern == OpusHeadPattern {
					streamType = StreamType_Opus
				}
			}

			fmt.Printf("stream type: %v\n", streamType)

			switch streamType {
			case StreamType_Vorbis:
				headersCount = 3
			}
		}

		switch streamType {
		case StreamType_Vorbis:
			streamHeaders = append(streamHeaders, p)
			headersCount--
		}

		if headersCount == 0 {
			break
		}
	}

	for _, header := range streamHeaders {
		err = d.decoder.ReadHeader(header)
		if err != nil {
			return err
		}
	}

	d.samplesReq = 4096
	d.channels = d.decoder.Channels()
	d.ringBuffer = ringbuffer.NewRingBuffer(2 * d.channels * d.samplesReq)

	return nil
}

func (d *oggVorbisDecoder) Close() error {
	if d.oggReader != nil {
		d.oggReader.Close()
	}
	if d.file != nil {
		d.file.Close()
	}
	return nil
}

func (d *oggVorbisDecoder) GetFormat() (int, int, int) {
	channels := d.decoder.Channels()
	sampleRate := d.decoder.SampleRate()

	return sampleRate, channels, 16
}

func (d *oggVorbisDecoder) DecodeSamples(samples int, audio []byte) (int, error) {
	outputBytesPerSample := 2
	var b16 [2]byte
	for {
		sampleBytes := d.ringBuffer.Size()
		samplesAvail := sampleBytes / (d.channels * outputBytesPerSample)
		if samplesAvail >= samples {
			bytesRequest := samples * d.channels * outputBytesPerSample
			bytesRead, err := d.ringBuffer.Read(bytesRequest, audio)
			if err != nil {
				return 0, err
			}
			samplesRead := bytesRead / (d.channels * outputBytesPerSample)
			return samplesRead, nil
		}

		if !d.oggReader.Next() {
			return 0, nil
		}

		packet, err := d.oggReader.Scan()
		if err != nil {
			return 0, err
		}

		out, err := d.decoder.Decode(packet)
		if err != nil {
			return 0, err
		}
		nSamples := len(out)

		for idx := 0; idx < nSamples; {
			for j := 0; j < 2; j++ {
				sv := int16(math.Floor(float64(out[idx+j]) * float64(32767)))

				b16[0] = byte(sv & 0xFF)
				b16[1] = byte(sv >> 8)
				d.ringBuffer.Write(b16[:2])
			}

			idx += 2
		}
	}
}
