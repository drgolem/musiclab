package decoders

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"os"

	"github.com/drgolem/go-ogg/ogg"
)

func GetOggFileStreamType(fileName string) (StreamType, error) {
	var streamType StreamType

	f, err := os.Open(fileName)
	if err != nil {
		return streamType, err
	}
	defer f.Close()

	reader := bufio.NewReader(f)

	oggReader, err := ogg.NewOggReader(reader)
	if err != nil {
		return streamType, err
	}

	if oggReader.Next() {
		p, err := oggReader.Scan()
		if err != nil {
			return streamType, err
		}

		bytesReader := bytes.NewReader(p)
		var ch VorbisCommonHeader
		err = binary.Read(bytesReader, binary.LittleEndian, &ch)
		if err == nil {
			if ch.PacketType == 1 && ch.VorbisPattern == VorbisPattern {
				streamType = StreamType_Vorbis
			}
		}

		if streamType == StreamType_Unknown {
			bytesReader = bytes.NewReader(p)
			var coh OpusCommonHeader
			err = binary.Read(bytesReader, binary.LittleEndian, &coh)
			if err == nil {
				if coh.OpusPattern == OpusHeadPattern {
					streamType = StreamType_Opus
				}
			}
		}
	}

	return streamType, nil
}
