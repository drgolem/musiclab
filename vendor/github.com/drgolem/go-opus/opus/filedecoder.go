package opus

/*
#cgo pkg-config: opusfile
#include <opusfile.h>
#include <stdlib.h>
*/
import "C"
import (
	"errors"
	"fmt"
	"unsafe"
)

func IsOpusData(data []byte) bool {
	dataLen := C.ulong(len(data))
	dataPtr := (*C.uint8_t)(unsafe.Pointer(&data[0]))
	var oh C.OpusHead
	res := C.op_test(&oh, dataPtr, dataLen)
	fmt.Printf("op_test: %d", res)
	fmt.Printf("Channels: %d\nInput sample rate: %d\n", oh.channel_count, oh.input_sample_rate)
	return res == 0
}

type OpusFileDecoder struct {
	oggOpusFile *C.OggOpusFile
	data        []byte
}

func NewOpusFileDecoder(filePath string) (*OpusFileDecoder, error) {

	filename := C.CString(filePath)
	defer C.free(unsafe.Pointer(filename))

	var errCode C.int
	oggOpusFile := C.op_open_file(filename, &errCode)
	if errCode != 0 {
		return nil, errors.New(fmt.Sprintf("opus open mem data, err: %d", errCode))
	}

	dec := OpusFileDecoder{
		oggOpusFile: oggOpusFile,
	}
	return &dec, nil
}

func NewOpusFileDecoderFromMemory(data []byte) (*OpusFileDecoder, error) {

	dataBuf := make([]byte, len(data))
	copy(dataBuf, data)

	dataLen := C.size_t(len(data))
	dataPtr := (*C.uint8_t)(unsafe.Pointer(&dataBuf[0]))
	var errCode C.int
	oggOpusFile := C.op_open_memory(dataPtr, dataLen, &errCode)
	if errCode != 0 {
		return nil, errors.New(fmt.Sprintf("opus open mem data, err: %d", errCode))
	}

	dec := OpusFileDecoder{
		oggOpusFile: oggOpusFile,
		data:        dataBuf,
	}
	return &dec, nil
}

func (dec *OpusFileDecoder) Close() {
	if dec.oggOpusFile != nil {
		C.op_free(dec.oggOpusFile)
	}
}

func (dec *OpusFileDecoder) Channels() int {
	if dec.oggOpusFile == nil {
		return 0
	}

	return int(C.op_channel_count(dec.oggOpusFile, -1))
}

func (dec *OpusFileDecoder) SampleRate() int {
	return 48000
}

func (dec *OpusFileDecoder) GetFormat() (int, int, int) {
	return dec.SampleRate(), dec.Channels(), 16
}

func (dec *OpusFileDecoder) TellCurrentSample() int64 {
	if dec.oggOpusFile == nil {
		return -1
	}
	return int64(C.op_pcm_tell(dec.oggOpusFile))
}

func (dec *OpusFileDecoder) TotalSamples() int64 {
	if dec.oggOpusFile == nil {
		return 0
	}
	return int64(C.op_pcm_total(dec.oggOpusFile, -1))
}

func (dec *OpusFileDecoder) DecodeSamples(samples int, audio []byte) (int, error) {
	if dec.oggOpusFile == nil {
		return 0, errors.New("invalid decoder")
	}
	dataPtr := (*C.int16_t)(unsafe.Pointer(&audio[0]))
	res := C.op_read(dec.oggOpusFile, dataPtr, C.int(samples), nil)
	if res < 0 {
		return 0, errors.New(fmt.Sprintf("opus read samples, err: %d", res))
	}
	return int(res), nil
}
