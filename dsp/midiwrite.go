package dsp

import (
	"encoding/binary"
	"io"
)

// MidiNote represents a single note event for MIDI file generation.
type MidiNote struct {
	Note     uint8 // MIDI note number (60 = C4)
	Velocity uint8 // 0-127
	StartMs  int   // start time in milliseconds
	DurMs    int   // duration in milliseconds
	Channel  uint8 // MIDI channel 0-15
}

// NoteNameToMIDI converts a note name (e.g. "C4", "A#3", "Eb5") to MIDI note number.
func NoteNameToMIDI(name string) uint8 {
	if len(name) < 2 {
		return 60 // default C4
	}

	noteMap := map[string]int{
		"C": 0, "D": 2, "E": 4, "F": 5, "G": 7, "A": 9, "B": 11,
	}

	base, ok := noteMap[name[0:1]]
	if !ok {
		return 60
	}

	offset := 1
	if len(name) > 2 {
		switch name[1] {
		case '#':
			base++
			offset = 2
		case 'b':
			base--
			offset = 2
		}
	}

	octave := int(name[offset] - '0')
	// MIDI: C4 = 60, so C0 = 12
	midi := (octave+1)*12 + base
	if midi < 0 {
		return 0
	}
	if midi > 127 {
		return 127
	}
	return uint8(midi)
}

// WriteMIDIFile writes a standard MIDI file (format 0, single track) to w.
// tempo is in BPM. Notes are placed according to their StartMs/DurMs.
func WriteMIDIFile(w io.Writer, notes []MidiNote, tempo int) error {
	if tempo <= 0 {
		tempo = 120
	}

	ticksPerQuarter := uint16(480)
	msPerTick := 60000.0 / float64(tempo) / float64(ticksPerQuarter)

	// Build track events sorted by time
	type midiEvent struct {
		tick    int
		status  uint8
		data1   uint8
		data2   uint8
	}

	var events []midiEvent
	for _, n := range notes {
		if n.Note == 0 {
			continue
		}
		startTick := int(float64(n.StartMs) / msPerTick)
		endTick := int(float64(n.StartMs+n.DurMs) / msPerTick)
		vel := n.Velocity
		if vel == 0 {
			vel = 100
		}
		ch := n.Channel & 0x0F
		events = append(events, midiEvent{tick: startTick, status: 0x90 | ch, data1: n.Note, data2: vel})
		events = append(events, midiEvent{tick: endTick, status: 0x80 | ch, data1: n.Note, data2: 0})
	}

	// Sort by tick (stable sort to keep note-off before note-on at same tick)
	for i := 1; i < len(events); i++ {
		for j := i; j > 0 && events[j].tick < events[j-1].tick; j-- {
			events[j], events[j-1] = events[j-1], events[j]
		}
	}

	// Add tempo event and end-of-track
	// Build track data bytes
	var trackData []byte

	// Tempo meta event: FF 51 03 tt tt tt (microseconds per quarter)
	usPerQuarter := 60000000 / tempo
	trackData = append(trackData, 0x00) // delta = 0
	trackData = append(trackData, 0xFF, 0x51, 0x03)
	trackData = append(trackData, byte(usPerQuarter>>16), byte(usPerQuarter>>8), byte(usPerQuarter))

	// Program change for channel 0: acoustic nylon guitar (preset 24)
	trackData = append(trackData, 0x00)       // delta = 0
	trackData = append(trackData, 0xC0, 0x18) // channel 0, program 24

	prevTick := 0
	for _, ev := range events {
		delta := ev.tick - prevTick
		if delta < 0 {
			delta = 0
		}
		trackData = append(trackData, encodeVarLen(delta)...)
		trackData = append(trackData, ev.status, ev.data1, ev.data2)
		prevTick = ev.tick
	}

	// End of track
	trackData = append(trackData, 0x00, 0xFF, 0x2F, 0x00)

	// Write MIDI file
	// Header: MThd
	if _, err := w.Write([]byte("MThd")); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint32(6)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint16(0)); err != nil { // format 0
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint16(1)); err != nil { // 1 track
		return err
	}
	if err := binary.Write(w, binary.BigEndian, ticksPerQuarter); err != nil {
		return err
	}

	// Track: MTrk
	if _, err := w.Write([]byte("MTrk")); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint32(len(trackData))); err != nil {
		return err
	}
	_, err := w.Write(trackData)
	return err
}

func encodeVarLen(value int) []byte {
	if value < 0 {
		value = 0
	}
	if value < 0x80 {
		return []byte{byte(value)}
	}
	var buf []byte
	buf = append(buf, byte(value&0x7F))
	value >>= 7
	for value > 0 {
		buf = append(buf, byte(value&0x7F)|0x80)
		value >>= 7
	}
	// Reverse
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return buf
}
