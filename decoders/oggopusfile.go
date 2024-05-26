package decoders

import (
	"github.com/drgolem/go-opus/opus"
	"github.com/drgolem/ringbuffer"
)

type oggOpusFileDecoder struct {
	decoder    *opus.OpusFileDecoder
	ringBuffer ringbuffer.RingBuffer
	channels   int
	samplesReq int
}

func NewOggOpusFileDecoder() (*oggOpusFileDecoder, error) {
	dec := oggOpusFileDecoder{}

	return &dec, nil
}

func (d *oggOpusFileDecoder) Open(fileName string) error {

	dec, err := opus.NewOpusFileDecoder(fileName)

	if err != nil {
		return err
	}
	d.decoder = dec

	d.samplesReq = 4096
	d.channels = d.decoder.Channels()
	d.ringBuffer = ringbuffer.NewRingBuffer(2 * d.channels * d.samplesReq)

	return nil
}

func (d *oggOpusFileDecoder) Close() error {
	if d.decoder != nil {
		d.decoder.Close()
	}
	return nil
}

func (d *oggOpusFileDecoder) GetFormat() (int, int, int) {
	channels := d.decoder.Channels()
	sampleRate := d.decoder.SampleRate()

	return sampleRate, channels, 16
}

func (d *oggOpusFileDecoder) DecodeSamples(samples int, audio []byte) (int, error) {
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

		samplesReq := 2048
		out := make([]byte, samplesReq*d.channels*2)
		nSamples, err := d.decoder.DecodeSamples(samplesReq, out)
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
