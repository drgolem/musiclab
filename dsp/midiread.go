package dsp

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// ReadMIDINotes parses a standard MIDI file and extracts note events.
func ReadMIDINotes(path string) ([]MidiNote, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	return ParseMIDINotes(f)
}

// ParseMIDINotes reads note-on/note-off events from a MIDI stream.
// Returns notes and tempo in BPM.
func ParseMIDINotes(r io.Reader) ([]MidiNote, int, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, 0, err
	}
	if len(data) < 14 {
		return nil, 0, fmt.Errorf("file too short for MIDI header")
	}

	// Parse header: MThd
	if string(data[0:4]) != "MThd" {
		return nil, 0, fmt.Errorf("not a MIDI file (missing MThd)")
	}
	headerLen := binary.BigEndian.Uint32(data[4:8])
	if headerLen < 6 {
		return nil, 0, fmt.Errorf("invalid MIDI header length")
	}
	numTracks := int(binary.BigEndian.Uint16(data[10:12]))
	division := binary.BigEndian.Uint16(data[12:14])

	ticksPerQuarter := int(division)
	if division&0x8000 != 0 {
		// SMPTE time division - not commonly used, approximate
		ticksPerQuarter = 480
	}

	tempo := 120 // default
	usPerQuarter := 500000

	type pendingNote struct {
		note    uint8
		vel     uint8
		channel uint8
		tickOn  int
	}

	var notes []MidiNote
	var pending []pendingNote
	pos := 8 + int(headerLen)

	for track := 0; track < numTracks && pos < len(data); track++ {
		if pos+8 > len(data) || string(data[pos:pos+4]) != "MTrk" {
			break
		}
		trackLen := int(binary.BigEndian.Uint32(data[pos+4 : pos+8]))
		trackEnd := pos + 8 + trackLen
		if trackEnd > len(data) {
			trackEnd = len(data)
		}
		p := pos + 8

		var absTick int
		var runningStatus byte

		for p < trackEnd {
			// Read variable-length delta
			delta, bytesRead := readVarLen(data[p:trackEnd])
			p += bytesRead
			absTick += delta

			if p >= trackEnd {
				break
			}

			status := data[p]
			if status < 0x80 {
				// Running status
				status = runningStatus
			} else {
				p++
				if status < 0xF0 {
					runningStatus = status
				}
			}

			switch {
			case status >= 0x80 && status <= 0x8F:
				// Note off
				if p+1 >= trackEnd {
					p = trackEnd
					break
				}
				note := data[p]
				p += 2 // note + velocity
				ch := status & 0x0F
				// Find and close pending note
				for i := len(pending) - 1; i >= 0; i-- {
					pn := pending[i]
					if pn.note == note && pn.channel == ch {
						durTick := absTick - pn.tickOn
						startMs := tickToMs(pn.tickOn, usPerQuarter, ticksPerQuarter)
						durMs := tickToMs(durTick, usPerQuarter, ticksPerQuarter)
						notes = append(notes, MidiNote{
							Note:     pn.note,
							Velocity: pn.vel,
							StartMs:  startMs,
							DurMs:    durMs,
							Channel:  pn.channel,
						})
						pending = append(pending[:i], pending[i+1:]...)
						break
					}
				}

			case status >= 0x90 && status <= 0x9F:
				// Note on (velocity 0 = note off)
				if p+1 >= trackEnd {
					p = trackEnd
					break
				}
				note := data[p]
				vel := data[p+1]
				p += 2
				ch := status & 0x0F
				if vel == 0 {
					// Treat as note off
					for i := len(pending) - 1; i >= 0; i-- {
						pn := pending[i]
						if pn.note == note && pn.channel == ch {
							durTick := absTick - pn.tickOn
							startMs := tickToMs(pn.tickOn, usPerQuarter, ticksPerQuarter)
							durMs := tickToMs(durTick, usPerQuarter, ticksPerQuarter)
							notes = append(notes, MidiNote{
								Note:     pn.note,
								Velocity: pn.vel,
								StartMs:  startMs,
								DurMs:    durMs,
								Channel:  pn.channel,
							})
							pending = append(pending[:i], pending[i+1:]...)
							break
						}
					}
				} else {
					pending = append(pending, pendingNote{
						note: note, vel: vel, channel: ch, tickOn: absTick,
					})
				}

			case status >= 0xA0 && status <= 0xAF:
				p += 2 // aftertouch
			case status >= 0xB0 && status <= 0xBF:
				p += 2 // control change
			case status >= 0xC0 && status <= 0xCF:
				p += 1 // program change
			case status >= 0xD0 && status <= 0xDF:
				p += 1 // channel pressure
			case status >= 0xE0 && status <= 0xEF:
				p += 2 // pitch bend

			case status == 0xFF:
				// Meta event
				if p >= trackEnd {
					break
				}
				metaType := data[p]
				p++
				metaLen, bytesRead := readVarLen(data[p:trackEnd])
				p += bytesRead
				if metaType == 0x51 && metaLen == 3 && p+3 <= trackEnd {
					// Tempo
					usPerQuarter = int(data[p])<<16 | int(data[p+1])<<8 | int(data[p+2])
					tempo = 60000000 / usPerQuarter
				}
				p += metaLen

			case status == 0xF0 || status == 0xF7:
				// SysEx
				sxLen, bytesRead := readVarLen(data[p:trackEnd])
				p += bytesRead + sxLen

			default:
				p++
			}
		}

		pos = trackEnd
	}

	// Sort notes by start time
	for i := 1; i < len(notes); i++ {
		for j := i; j > 0 && notes[j].StartMs < notes[j-1].StartMs; j-- {
			notes[j], notes[j-1] = notes[j-1], notes[j]
		}
	}

	return notes, tempo, nil
}

func readVarLen(data []byte) (int, int) {
	value := 0
	bytesRead := 0
	for i := 0; i < len(data) && i < 4; i++ {
		b := data[i]
		value = (value << 7) | int(b&0x7F)
		bytesRead++
		if b&0x80 == 0 {
			break
		}
	}
	return value, bytesRead
}

func tickToMs(ticks, usPerQuarter, ticksPerQuarter int) int {
	if ticksPerQuarter == 0 {
		return 0
	}
	return ticks * usPerQuarter / ticksPerQuarter / 1000
}
