package decoders

var VorbisPattern = [6]byte{'v', 'o', 'r', 'b', 'i', 's'}
var OpusHeadPattern = [8]byte{'O', 'p', 'u', 's', 'H', 'e', 'a', 'd'}
var PpusTagsPattern = [8]byte{'O', 'p', 'u', 's', 'T', 'a', 'g', 's'}

type StreamType int

const (
	StreamType_Unknown StreamType = iota
	StreamType_Vorbis
	StreamType_Opus
)

type oggReader interface {
	Next() bool
	Scan() ([]byte, error)
	Close()
}

type VorbisCommonHeader struct {
	PacketType    byte
	VorbisPattern [6]byte
}

type VorbisIdentificationHeader struct {
	Version         uint32
	AudioChannels   byte
	AudioSampleRate uint32
	BitrateMax      int32
	BitrateMin      int32
	BlockSize01     uint32
	FraminfFlag     byte
}

type OpusCommonHeader struct {
	OpusPattern [8]byte
}

type OpusIdentificationHeader struct {
	Version         byte
	AudioChannels   byte
	PreSkip         uint16
	AudioSampleRate uint32
	OutputGain      uint16
	MappingFamily   byte
}
