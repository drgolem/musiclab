package portaudio

/*
#cgo pkg-config: portaudio-2.0
#include <portaudio.h>
extern PaStreamCallback* paStreamCallback;
*/
import "C"
import (
	"errors"
	"unsafe"
)

type PaSampleFormat int

const (
	SampleFmtFloat32 PaSampleFormat = C.paFloat32
	SampleFmtInt32   PaSampleFormat = C.paInt32
	SampleFmtInt24   PaSampleFormat = C.paInt24
	SampleFmtInt16   PaSampleFormat = C.paInt16
	SampleFmtInt8    PaSampleFormat = C.paInt8
	SampleFmtUInt8   PaSampleFormat = C.paUInt8
)

type PaTime float32

type PaStreamParameters struct {
	DeviceIndex      int
	ChannelCount     int
	SampleFormat     PaSampleFormat
	SuggestedLatency PaTime
}

type PaError struct {
	ErrorCode int
}

type PaStream struct {
	stream           unsafe.Pointer
	isOpen           bool
	OutputParameters PaStreamParameters
	SampleRate       float32
}

func (e *PaError) Error() string {
	return GetErrorText(e.ErrorCode)
}

func GetVersion() int {
	return int(C.Pa_GetVersion())
}

func GetVersionText() string {
	vi := C.Pa_GetVersionInfo()
	vt := C.GoString(vi.versionText)
	return vt
}

func GetErrorText(errorCode int) string {
	return C.GoString(C.Pa_GetErrorText(C.int(errorCode)))
}

func Initialize() error {
	errCode := C.Pa_Initialize()
	if errCode == C.paNoError {
		return nil
	}
	return &PaError{int(errCode)}
}

func Terminate() error {
	errCode := C.Pa_Terminate()
	if errCode == C.paNoError {
		return nil
	}
	return &PaError{int(errCode)}
}

func GetDeviceCount() (int, error) {
	dc := int(C.Pa_GetDeviceCount())
	if dc < 0 {
		return 0, &PaError{dc}
	}
	return dc, nil
}

// const PaDeviceInfo* Pa_GetDeviceInfo	(	PaDeviceIndex 	device	)

type DeviceInfo struct {
	// const char * 	name
	Name string
	// PaHostApiIndex 	hostApi
	HostApiIndex int
	// int 	maxInputChannels
	MaxInputChannels int
	// int 	maxOutputChannels
	MaxOutputChannels int
	// PaTime 	defaultLowInputLatency
	DefaultLowInputLatency float32
	// PaTime 	defaultLowOutputLatency
	DefaultLowOutputLatency float32
	// PaTime 	defaultHighInputLatency
	DefaultHighInputLatency float32
	// PaTime 	defaultHighOutputLatency
	DefaultHighOutputLatency float32
	// double 	defaultSampleRate
	DefaultSampleRate float32
}

func GetDeviceInfo(deviceIdx int) (*DeviceInfo, error) {
	di := C.Pa_GetDeviceInfo(C.int(deviceIdx))
	if di == nil {
		return nil, errors.New("invalid device index")
	}

	devInfo := DeviceInfo{
		Name:                     C.GoString(di.name),
		HostApiIndex:             int(di.hostApi),
		MaxInputChannels:         int(di.maxInputChannels),
		MaxOutputChannels:        int(di.maxOutputChannels),
		DefaultLowInputLatency:   float32(di.defaultLowInputLatency),
		DefaultLowOutputLatency:  float32(di.defaultLowOutputLatency),
		DefaultHighInputLatency:  float32(di.defaultHighInputLatency),
		DefaultHighOutputLatency: float32(di.defaultHighOutputLatency),
		DefaultSampleRate:        float32(di.defaultSampleRate),
	}

	return &devInfo, nil
}

func GetHostApiCount() (int, error) {
	hc := int(C.Pa_GetHostApiCount())
	if hc < 0 {
		return 0, &PaError{hc}
	}
	return hc, nil
}

type HostApiInfo struct {
	Type                int
	Name                string
	DeviceCount         int
	DefaultInputDevice  int
	DefaultOutputDevice int
}

func GetHostApiInfo(hostApiIdx int) (*HostApiInfo, error) {
	hi := C.Pa_GetHostApiInfo(C.int(hostApiIdx))
	if hi == nil {
		return nil, errors.New("invalid host API index")
	}

	devInfo := HostApiInfo{
		Type:                int(hi._type),
		Name:                C.GoString(hi.name),
		DeviceCount:         int(hi.deviceCount),
		DefaultInputDevice:  int(hi.defaultInputDevice),
		DefaultOutputDevice: int(hi.defaultOutputDevice),
	}

	return &devInfo, nil
}

func IsFormatSupported(inputParameters *PaStreamParameters, outputParameters *PaStreamParameters, sampleRate float32) error {

	var inParams, outParams *C.PaStreamParameters

	if outputParameters != nil {
		outParams = &C.PaStreamParameters{
			device:       C.int(outputParameters.DeviceIndex),
			channelCount: C.int(outputParameters.ChannelCount),
			sampleFormat: C.PaSampleFormat(outputParameters.SampleFormat),
		}
	}

	errCode := C.Pa_IsFormatSupported(inParams, outParams, C.double(sampleRate))
	if errCode != C.paFormatIsSupported {
		return &PaError{int(errCode)}
	}
	return nil
}

// TODO: current implementation only for output streams
func NewStream(outParams PaStreamParameters, sampleRate float32) (*PaStream, error) {
	err := IsFormatSupported(nil, &outParams, sampleRate)
	if err != nil {
		return nil, err
	}

	st := PaStream{
		OutputParameters: outParams,
		SampleRate:       sampleRate,
	}

	return &st, nil
}

func (s *PaStream) Open(framesPerBuffer int) error {

	// get device info
	di, err := GetDeviceInfo(s.OutputParameters.DeviceIndex)
	if err != nil {
		return err
	}

	outParams := &C.PaStreamParameters{
		device:           C.int(s.OutputParameters.DeviceIndex),
		channelCount:     C.int(s.OutputParameters.ChannelCount),
		sampleFormat:     C.PaSampleFormat(s.OutputParameters.SampleFormat),
		suggestedLatency: C.double(di.DefaultLowOutputLatency),
	}

	streamFlags := int(C.paNoFlag)

	errCode := C.Pa_OpenStream(&s.stream,
		nil,
		outParams,
		C.double(s.SampleRate),
		C.ulong(framesPerBuffer),
		C.PaStreamFlags(streamFlags),
		nil,
		nil)

	if errCode != C.paNoError {
		return &PaError{int(errCode)}
	}

	s.isOpen = true

	return nil
}

func (s *PaStream) Close() error {
	if !s.isOpen {
		return nil
	}

	errCode := C.Pa_CloseStream(s.stream)
	if errCode != C.paNoError {
		return &PaError{int(errCode)}
	}

	s.isOpen = false

	return nil
}

func (s *PaStream) StartStream() error {
	if !s.isOpen {
		return &PaError{int(C.paBadStreamPtr)}
	}

	errCode := C.Pa_StartStream(s.stream)
	if errCode != C.paNoError {
		return &PaError{int(errCode)}
	}

	return nil
}

func (s *PaStream) StopStream() error {
	if !s.isOpen {
		return &PaError{int(C.paBadStreamPtr)}
	}

	errCode := C.Pa_StopStream(s.stream)
	if errCode != C.paNoError {
		return &PaError{int(errCode)}
	}

	return nil
}

func (s *PaStream) Write(frames int, buf []byte) error {
	if !s.isOpen {
		return &PaError{int(C.paBadStreamPtr)}
	}

	// TODO: only interleaved frames supported

	buffer := unsafe.Pointer(&buf[0])

	errCode := C.Pa_WriteStream(s.stream, buffer, C.ulong(frames))
	if errCode != C.paNoError {
		return &PaError{int(errCode)}
	}

	return nil
}
