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

type OpusPacketEncoder struct {
	opusEncoder *C.OpusEncoder
	channels    int
	sampleRate  int
}

func NewOpusPacketEncoder(channels int) (*OpusPacketEncoder, error) {
	var errCode C.int
	OPUS_APPLICATION_AUDIO := 2049
	sampleRate := 48000
	enc := C.opus_encoder_create(C.opus_int32(sampleRate), C.int(channels), C.int(OPUS_APPLICATION_AUDIO), &errCode)
	if errCode != 0 {
		return nil, errors.New(fmt.Sprintf("create encoder err: %d", errCode))
	}
	dec := OpusPacketEncoder{
		opusEncoder: enc,
		channels:    channels,
		sampleRate:  sampleRate,
	}
	return &dec, nil
}

func (d *OpusPacketEncoder) Close() {
	if d.opusEncoder != nil {
		C.opus_encoder_destroy(d.opusEncoder)
	}
}

func (d *OpusPacketEncoder) Channels() int {
	return d.channels
}

func (d *OpusPacketEncoder) SampleRate() int {
	return d.sampleRate
}

// returns samplerate, channels, bitsPerSample
func (d *OpusPacketEncoder) GetFormat() (int, int, int) {
	return d.sampleRate, d.channels, 16
}

// Encodes number of samples from audio buffer to packet
// audio buffer must contain pcm channel interleaved 16bit samples
// The passed samples must an opus frame size for the encoder's sampling rate.
// For example, at 48kHz the permitted values are 120, 240, 480, or 960.
// Returns length of the data payload (in bytes)
func (d *OpusPacketEncoder) EncodeSamples(packet []byte, samples int, audio []byte) (int, error) {
	if d.opusEncoder == nil {
		return 0, errors.New("invalid encoder")
	}
	pcmPtr := (*C.opus_int16)(unsafe.Pointer(&audio[0]))
	frameSize := C.int(samples)
	dataPtr := (*C.uint8_t)(unsafe.Pointer(&packet[0]))
	max_data_bytes := C.int(len(packet))
	res := C.opus_encode(d.opusEncoder, pcmPtr, frameSize, dataPtr, max_data_bytes)
	if res < 0 {
		return 0, errors.New(fmt.Sprintf("opus read samples, err: %d", res))
	}
	return int(res), nil
}
