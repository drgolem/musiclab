package flac

/*
#cgo pkg-config: flac
#include <FLAC/format.h>
#include <FLAC/stream_decoder.h>
#include <stdlib.h>

extern int
get_decoder_channels(FLAC__StreamMetadata *metadata);

extern int
get_decoder_depth(FLAC__StreamMetadata *metadata);

extern int
get_decoder_rate(FLAC__StreamMetadata *metadata);

extern FLAC__uint64
get_total_samples(FLAC__StreamMetadata *metadata);

extern void
decoderErrorCallback_cgo(const FLAC__StreamDecoder *,
                 FLAC__StreamDecoderErrorStatus,
                 void *);

extern void
decoderMetadataCallback_cgo(const FLAC__StreamDecoder *,
                const FLAC__StreamMetadata *,
                void *);

extern FLAC__StreamDecoderWriteStatus
decoderWriteCallback_cgo(const FLAC__StreamDecoder *,
                 const FLAC__Frame *,
                 const FLAC__int32 **,
                 void *);
*/
import "C"

import (
	"errors"
	"fmt"
	"io"
	"runtime/cgo"
	"unsafe"

	"github.com/drgolem/ringbuffer"
)

//type AudioFrameDecoder interface {
//	Open(filePath string) error
//	Close() error
//
//	TotalSamples() int64
//	TellCurrentSample() int64
//	GetFormat() (rate int, channels int, bitsPerSample int)
//
//	// Decodes audio samples, returns number of samples
//	DecodeSamples(samples int, audio []byte) (int, error)
//
//	io.Seeker
//}

func GetVersion() string {
	return C.GoString(C.FLAC__VERSION_STRING)
}

type FlacDecoder struct {
	decoder  *C.FLAC__StreamDecoder
	hDecoder cgo.Handle

	rate                    int64
	channels                int
	bitsPerSample           int
	outputBytesPerSample    int
	currentSample           int64
	totalSamples            int64
	maxOutputSampleBitDepth int
	streamBytesPerSample    int

	ringBuffer ringbuffer.RingBuffer
	b16        [2]byte
	b24        [3]byte
}

const (
	ringBufferCapacity = 2 * 2 * 4 * 4096
)

func NewFlacFrameDecoder(maxOutputSampleBitDepth int) (*FlacDecoder, error) {

	dec := C.FLAC__stream_decoder_new()

	fd := FlacDecoder{
		decoder:                 dec,
		ringBuffer:              ringbuffer.NewRingBuffer(ringBufferCapacity),
		maxOutputSampleBitDepth: maxOutputSampleBitDepth,
	}

	hDecoder := cgo.NewHandle(&fd)
	fd.hDecoder = hDecoder

	return &fd, nil
}

func (d *FlacDecoder) Delete() error {

	C.FLAC__stream_decoder_delete(d.decoder)

	d.hDecoder.Delete()

	return nil
}

func (d *FlacDecoder) GetResolvedState() string {
	return C.GoString(C.FLAC__stream_decoder_get_resolved_state_string(d.decoder))
}

//export decoderErrorCallback
func decoderErrorCallback(d *C.FLAC__StreamDecoder, status C.FLAC__StreamDecoderErrorStatus, data unsafe.Pointer) {
	fmt.Println("decoderErrorCallback")
}

//export decoderWriteCallback
func decoderWriteCallback(decoder *C.FLAC__StreamDecoder, frame *C.FLAC__Frame, buffer **C.FLAC__int32, client_data unsafe.Pointer) C.FLAC__StreamDecoderWriteStatus {

	h := *(*cgo.Handle)(client_data)
	dec := h.Value().(*FlacDecoder)

	//fmt.Println("Frame header: %#v\n", frame.header)

	samplesCount := int64(frame.header.blocksize)

	chLen := 2
	chSlice := unsafe.Slice(buffer, chLen)
	length := samplesCount
	chLeft := unsafe.Slice(chSlice[0], length)
	chRight := unsafe.Slice(chSlice[1], length)

	var sample int32
	for i := int64(0); i < samplesCount; i++ {
		for ch := 0; ch <= 1; ch++ {
			if ch == 0 {
				sample = int32(chLeft[i])
			} else if ch == 1 {
				sample = int32(chRight[i])
			}

			switch dec.streamBytesPerSample {
			case 3:
				int32toInt24LEBytes(sample, &dec.b24)
				if dec.maxOutputSampleBitDepth == 24 {
					dec.ringBuffer.Write(dec.b24[:3])
				} else {
					dec.ringBuffer.Write(dec.b24[1:3])
				}
			case 2:
				dec.b16[0] = byte(sample & 0xFF)
				dec.b16[1] = byte((sample & 0xFF00) >> 8)
				dec.ringBuffer.Write(dec.b16[:2])
			}
		}
	}

	return C.FLAC__STREAM_DECODER_WRITE_STATUS_CONTINUE
}

//export decoderMetadataCallback
func decoderMetadataCallback(d *C.FLAC__StreamDecoder, metadata *C.FLAC__StreamMetadata, client_data unsafe.Pointer) {
	//fmt.Printf("metadata: %#v\n", metadata)

	h := *(*cgo.Handle)(client_data)
	dec := h.Value().(*FlacDecoder)

	if metadata._type == C.FLAC__METADATA_TYPE_STREAMINFO {
		dec.channels = int(C.get_decoder_channels(metadata))
		dec.bitsPerSample = int(C.get_decoder_depth(metadata))
		dec.rate = int64(C.get_decoder_rate(metadata))
		dec.streamBytesPerSample = dec.bitsPerSample / 8
		dec.totalSamples = int64(C.get_total_samples(metadata))

		dec.outputBytesPerSample = 2
		if dec.bitsPerSample == dec.maxOutputSampleBitDepth {
			dec.outputBytesPerSample = dec.streamBytesPerSample
		}
	}
}

func (d *FlacDecoder) Open(filePath string) error {
	filename := C.CString(filePath)
	defer C.free(unsafe.Pointer(filename))

	var status C.FLAC__StreamDecoderInitStatus

	write_callback := C.FLAC__StreamDecoderWriteCallback(unsafe.Pointer(C.decoderWriteCallback_cgo))
	metadata_callback := C.FLAC__StreamDecoderMetadataCallback(unsafe.Pointer(C.decoderMetadataCallback_cgo))
	error_callback := C.FLAC__StreamDecoderErrorCallback(unsafe.Pointer(C.decoderErrorCallback_cgo))

	decClean := FlacDecoder{}

	d.rate = decClean.rate
	d.channels = decClean.channels
	d.bitsPerSample = decClean.bitsPerSample
	d.outputBytesPerSample = decClean.outputBytesPerSample
	d.currentSample = decClean.currentSample
	d.totalSamples = decClean.totalSamples

	d.ringBuffer.Reset()

	status = C.FLAC__stream_decoder_init_file(d.decoder, filename,
		write_callback,
		metadata_callback,
		error_callback,
		unsafe.Pointer(&d.hDecoder),
	)

	if status != C.FLAC__STREAM_DECODER_INIT_STATUS_OK {
		errStr := getStreamDecoderInitStatusString(status)
		return errors.New(fmt.Sprintf("init flac error: %s", errStr))
	}

	if C.FLAC__stream_decoder_process_until_end_of_metadata(d.decoder) == 0 {
		state := C.FLAC__stream_decoder_get_state(d.decoder)
		return errors.New(fmt.Sprintf("decode metadata error: %d", state))
	}

	return nil
}

func (d *FlacDecoder) Close() error {

	C.FLAC__stream_decoder_finish(d.decoder)

	decClean := FlacDecoder{}

	d.rate = decClean.rate
	d.channels = decClean.channels
	d.bitsPerSample = decClean.bitsPerSample
	d.outputBytesPerSample = decClean.outputBytesPerSample
	d.currentSample = decClean.currentSample
	d.totalSamples = decClean.totalSamples

	d.ringBuffer.Reset()

	return nil
}

func (d *FlacDecoder) TotalSamples() int64 {
	return d.totalSamples
}

func (d *FlacDecoder) TellCurrentSample() int64 {
	return d.currentSample
}

func (d *FlacDecoder) GetFormat() (int, int, int) {
	return int(d.rate), d.channels, d.bitsPerSample
}

func (d *FlacDecoder) DecodeSamples(samples int, audio []byte) (int, error) {

	for {
		state := C.FLAC__stream_decoder_get_state(d.decoder)

		sampleBytes := d.ringBuffer.Size()
		samplesAvail := sampleBytes / (d.channels * d.outputBytesPerSample)
		if state == C.FLAC__STREAM_DECODER_END_OF_STREAM || samplesAvail >= samples {

			bytesRequest := samples * d.channels * d.outputBytesPerSample
			if state == C.FLAC__STREAM_DECODER_END_OF_STREAM {
				bytesRequest = d.ringBuffer.Size()
			}

			bytesRead, err := d.ringBuffer.Read(bytesRequest, audio)
			samplesRead := bytesRead / (d.channels * d.outputBytesPerSample)
			d.currentSample += int64(samplesRead)
			return samplesRead, err
		}

		res := C.FLAC__stream_decoder_process_single(d.decoder)

		if res == 0 {
			return 0, errors.New(fmt.Sprintf("decode samples error: %d", state))
		}
	}

	return 0, nil
}

func (d *FlacDecoder) Seek(offset int64, whence int) (int64, error) {

	seekSample := offset
	if whence == io.SeekCurrent {
		seekSample = d.currentSample + offset
	}

	C.FLAC__stream_decoder_seek_absolute(d.decoder, C.FLAC__uint64(seekSample))

	d.currentSample = seekSample
	//d.ringBuffer.Reset()

	return d.currentSample, nil
}

func getStreamDecoderInitStatusString(status C.FLAC__StreamDecoderInitStatus) string {
	var theCArray **C.char = (**C.char)(unsafe.Pointer(&C.FLAC__StreamDecoderInitStatusString))
	length := 5
	slice := unsafe.Slice(theCArray, length)

	return C.GoString(slice[status])
}

func int32toInt24LEBytes(n int32, out *[3]byte) {
	if (n & 0x800000) > 0 {
		n |= ^0xffffff
	}
	out[2] = byte(n >> 16)
	out[1] = byte(n >> 8)
	out[0] = byte(n >> 0)
}
