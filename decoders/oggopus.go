package decoders

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/drgolem/go-ogg/ogg"
	"github.com/drgolem/go-opus/opus"
	"github.com/drgolem/ringbuffer"
)

type oggOpusDecoder struct {
	oggReader  oggReader
	decoder    *opus.OpusPacketDecoder
	file       *os.File
	reader     *bufio.Reader
	ringBuffer ringbuffer.RingBuffer
	channels   int
	samplesReq int
}

func NewOggOpusDecoder() (*oggOpusDecoder, error) {
	dec := oggOpusDecoder{}

	return &dec, nil
}

func (d *oggOpusDecoder) Open(fileName string) error {
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
			var coh OpusCommonHeader
			err = binary.Read(bytesReader, binary.LittleEndian, &coh)
			if err == nil {
				if coh.OpusPattern == OpusHeadPattern {
					streamType = StreamType_Opus
				}
			}

			fmt.Printf("stream type: %v\n", streamType)

			switch streamType {
			case StreamType_Opus:
				headersCount = 2
			}
		}

		switch streamType {
		case StreamType_Opus:
			streamHeaders = append(streamHeaders, p)
			headersCount--
		}

		if headersCount == 0 {
			break
		}
	}

	channels := 2
	sampleRate := 48000

	dec, err := opus.NewOpusPacketDecoder(channels, sampleRate)
	if err != nil {
		return err
	}
	d.decoder = dec

	d.samplesReq = 4096
	d.channels = d.decoder.Channels()
	d.ringBuffer = ringbuffer.NewRingBuffer(2 * d.channels * d.samplesReq)

	return nil
}

func (d *oggOpusDecoder) Close() error {
	if d.oggReader != nil {
		d.oggReader.Close()
	}
	if d.decoder != nil {
		d.decoder.Close()
	}
	if d.file != nil {
		d.file.Close()
	}
	return nil
}

func (d *oggOpusDecoder) GetFormat() (int, int, int) {
	channels := d.decoder.Channels()
	sampleRate := d.decoder.SampleRate()

	return sampleRate, channels, 16
}

func (d *oggOpusDecoder) DecodeSamples(samples int, audio []byte) (int, error) {
	outputBytesPerSample := 2
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

		samplesReq := 2048
		out := make([]byte, samplesReq*d.channels*2)
		nSamples, err := d.decoder.DecodeSamples(packet, samplesReq, out)
		if err != nil {
			return 0, err
		}
		if nSamples == 0 {
			return 0, nil
		}
		bytesLen := nSamples * d.channels * 2
		_, err = d.ringBuffer.Write(out[:bytesLen])
		if err != nil {
			return 0, err
		}
	}
}
