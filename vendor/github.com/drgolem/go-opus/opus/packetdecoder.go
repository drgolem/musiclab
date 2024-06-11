package opus

/*
#cgo pkg-config: opus
#include <opus.h>
*/
import "C"
import (
	"errors"
	"fmt"
	"unsafe"
)

type OpusPacketDecoder struct {
	opusDecoder *C.OpusDecoder
	channels    int
	sampleRate  int
}

func NewOpusPacketDecoder(channels int, sampleRate int) (*OpusPacketDecoder, error) {
	var errCode C.int
	d := C.opus_decoder_create(C.int(sampleRate), C.int(channels), &errCode)
	if errCode != 0 {
		return nil, errors.New(fmt.Sprintf("create decoder err: %d", errCode))
	}
	dec := OpusPacketDecoder{
		opusDecoder: d,
		channels:    channels,
		sampleRate:  sampleRate,
	}
	return &dec, nil
}

func (d *OpusPacketDecoder) Close() {
	if d.opusDecoder != nil {
		C.opus_decoder_destroy(d.opusDecoder)
	}
}

func (d *OpusPacketDecoder) Channels() int {
	return d.channels
}

func (d *OpusPacketDecoder) SampleRate() int {
	return d.sampleRate
}

// returns samplerate, channels, bitsPerSample
func (d *OpusPacketDecoder) GetFormat() (int, int, int) {
	return d.sampleRate, d.channels, 16
}

func (d *OpusPacketDecoder) DecodeSamples(packet []byte, samples int, audio []byte) (int, error) {
	if d.opusDecoder == nil {
		return 0, errors.New("invalid decoder")
	}
	dataPtr := (*C.uint8_t)(unsafe.Pointer(&packet[0]))
	dataLen := C.int32_t(len(packet))
	pcmPtr := (*C.int16_t)(unsafe.Pointer(&audio[0]))
	res := C.opus_decode(d.opusDecoder, dataPtr, dataLen, pcmPtr, C.int(samples), 0)
	if res < 0 {
		return 0, errors.New(fmt.Sprintf("opus read samples, err: %d", res))
	}
	return int(res), nil
}
