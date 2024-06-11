package ogg

import (
	"encoding/binary"
	"errors"
	"io"
)

var capturePattern = [4]byte{'O', 'g', 'g', 'S'}

const (
	headerFlagNewPacket         = 0
	headerFlagContinuedPacket   = 1
	headerFlagBeginningOfStream = 2
	headerFlagEndOfStream       = 4
)

type pageHeader struct {
	CapturePattern          [4]byte
	StreamStructureVersion  uint8
	HeaderTypeFlag          byte
	AbsoluteGranulePosition int64
	StreamSerialNumber      uint32
	PageSequenceNumber      uint32
	PageChecksum            uint32
	PageSegments            uint8
}

type oggPacketReader struct {
	r io.Reader
	//header []byte
	ph           pageHeader
	segmentTable []byte
	totalSize    int
	packetSizes  []int
	packetIdx    int
}

func NewOggReader(r io.Reader) (*oggPacketReader, error) {
	rd := oggPacketReader{
		r: r,
		//header: make([]byte, 27),
	}
	return &rd, nil
}

func (r *oggPacketReader) Close() {

}

func (r *oggPacketReader) Next() bool {

	if len(r.packetSizes) > 0 && r.packetIdx < len(r.packetSizes) {
		return true
	}

	err := binary.Read(r.r, binary.LittleEndian, &r.ph)
	if err != nil {
		return false
	}
	if r.ph.CapturePattern != capturePattern {
		return false
	}
	if r.ph.StreamStructureVersion != 0 {
		return false
	}
	r.segmentTable = make([]byte, r.ph.PageSegments)
	_, err = io.ReadFull(r.r, r.segmentTable)
	if err != nil {
		return false
	}
	r.totalSize = 0
	r.packetSizes = make([]int, 0)
	r.packetIdx = 0
	size := 0
	for _, s := range r.segmentTable {
		size += int(s)
		r.totalSize += int(s)
		if s < 0xFF {
			r.packetSizes = append(r.packetSizes, size)
			size = 0
		}
	}
	return true
}

func (r *oggPacketReader) Scan() ([]byte, error) {
	if len(r.packetSizes) == 0 || r.packetIdx >= len(r.packetSizes) {
		return nil, errors.New("invalid packet")
	}
	packet := make([]byte, r.packetSizes[r.packetIdx])
	_, err := io.ReadFull(r.r, packet)
	if err != nil {
		return nil, errors.New("invalid packet")
	}

	r.packetIdx++
	return packet, nil
}
