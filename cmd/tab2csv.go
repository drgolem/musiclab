package cmd

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/drgolem/musiclab/dsp"
	"github.com/spf13/cobra"
)

var tab2csvCmd = &cobra.Command{
	Use:   "tab2csv",
	Short: "Convert ASCII guitar tab to score CSV or MIDI",
	Long: `Parse ASCII guitar tablature and convert to score CSV or MIDI file.

Reads standard 6-string guitar tab format:
  e|---0---5---7---|
  B|---1-----------|
  G|---2-----------|
  D|---3-----------|
  A|---3-----------|
  E|---0-----------|

Examples:
  musiclab tab2csv --in stairway.tab --out stairway.csv
  musiclab tab2csv --in stairway.tab --export-midi stairway.mid --tempo 72
  musiclab tab2csv --in stairway.tab --out stairway.csv --bpc 8 --tempo 72`,
	Run: doTab2csvCmd,
}

func init() {
	rootCmd.AddCommand(tab2csvCmd)

	tab2csvCmd.Flags().String("in", "", "input tab file (or - for stdin)")
	tab2csvCmd.Flags().String("out", "", "output CSV file (default: stdout)")
	tab2csvCmd.Flags().String("export-midi", "", "export as MIDI file")
	tab2csvCmd.Flags().Int("tempo", 120, "tempo in BPM")
	tab2csvCmd.Flags().Int("bpc", 8, "beats per tab column (subdivisions per character position)")
	tab2csvCmd.Flags().Int("tuning", 0, "tuning offset in semitones")
	tab2csvCmd.Flags().Int("capo", 0, "capo fret position")
}

func doTab2csvCmd(cmd *cobra.Command, args []string) {
	inFile, _ := cmd.Flags().GetString("in")
	outFile, _ := cmd.Flags().GetString("out")
	exportMidi, _ := cmd.Flags().GetString("export-midi")
	tempo, _ := cmd.Flags().GetInt("tempo")
	bpc, _ := cmd.Flags().GetInt("bpc")
	tuningOffset, _ := cmd.Flags().GetInt("tuning")
	capo, _ := cmd.Flags().GetInt("capo")

	if inFile == "" {
		slog.Error("--in is required")
		return
	}

	var lines []string
	if inFile == "-" {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
	} else {
		data, err := os.ReadFile(inFile)
		if err != nil {
			slog.Error("failed to read file", "error", err)
			return
		}
		lines = strings.Split(string(data), "\n")
	}

	// Build tuning with offset + capo
	var tuning [6]uint8
	for i, n := range standardTuning {
		tuning[i] = uint8(int(n) + tuningOffset + capo)
	}

	notes := parseTabLines(lines, tuning, tempo, bpc)
	slog.Info("Parsed tab", "notes", len(notes))

	if len(notes) == 0 {
		slog.Error("no notes found in tab")
		return
	}

	// Build CSV
	csv := buildCSV(notes)

	if outFile != "" {
		if err := os.WriteFile(outFile, []byte(csv), 0644); err != nil {
			slog.Error("failed to write CSV", "error", err)
			return
		}
		slog.Info("Written CSV", "path", outFile)
	} else if exportMidi == "" {
		fmt.Print(csv)
	}

	// Export MIDI
	if exportMidi != "" {
		f, err := os.Create(exportMidi)
		if err != nil {
			slog.Error("failed to create MIDI", "error", err)
			return
		}
		if err := dsp.WriteMIDIFile(f, notes, tempo); err != nil {
			f.Close()
			slog.Error("failed to write MIDI", "error", err)
			return
		}
		f.Close()
		slog.Info("Written MIDI", "path", exportMidi, "notes", len(notes))
	}
}

// parseTabLines extracts note events from ASCII guitar tab lines.
// It identifies groups of 6 consecutive string lines (e, B, G, D, A, E)
// and reads fret numbers at each column position.
func parseTabLines(lines []string, tuning [6]uint8, tempo, bpc int) []dsp.MidiNote {
	// Ms per column position: one column = one subdivision
	// At given tempo, quarter note = 60000/tempo ms
	// bpc controls how many columns make a beat
	msPerCol := 60000.0 / float64(tempo) / float64(bpc)

	var allNotes []dsp.MidiNote
	globalCol := 0 // running column offset across tab blocks

	// Find groups of 6 string lines
	tabBlocks := extractTabBlocks(lines)

	for _, block := range tabBlocks {
		blockNotes := parseTabBlock(block, tuning, msPerCol, globalCol)
		allNotes = append(allNotes, blockNotes...)

		// Advance global column by block width
		if len(block) > 0 {
			globalCol += len(block[0].data)
		}
	}

	// Sort by start time
	for i := 1; i < len(allNotes); i++ {
		for j := i; j > 0 && allNotes[j].StartMs < allNotes[j-1].StartMs; j-- {
			allNotes[j], allNotes[j-1] = allNotes[j-1], allNotes[j]
		}
	}

	return allNotes
}

type tabLine struct {
	stringIdx int    // 0=low E, 1=A, 2=D, 3=G, 4=B, 5=high e
	data      string // the fret data portion after the string label
}

// extractTabBlocks finds groups of 6 tab string lines in the input.
func extractTabBlocks(lines []string) [][]tabLine {
	var blocks [][]tabLine
	var currentBlock []tabLine

	for _, line := range lines {
		tl, ok := parseTabLineLabel(line)
		if ok {
			currentBlock = append(currentBlock, tl)
			if len(currentBlock) == 6 {
				// Verify we have all 6 strings
				if isCompleteBlock(currentBlock) {
					blocks = append(blocks, currentBlock)
				}
				currentBlock = nil
			}
		} else {
			// Non-tab line resets current block
			if len(currentBlock) > 0 && len(currentBlock) < 6 {
				currentBlock = nil
			}
		}
	}

	return blocks
}

// parseTabLineLabel identifies a tab line and extracts string index + data.
// Handles formats like: "e|---5---", "E||---5---", "B |---5---"
func parseTabLineLabel(line string) (tabLine, bool) {
	line = strings.TrimRight(line, " \t\r")
	if len(line) < 3 {
		return tabLine{}, false
	}

	// Map string labels to indices (0=low E to 5=high e)
	stringMap := map[string]int{
		"E": 0, "A": 1, "D": 2, "G": 3, "B": 4, "e": 5,
	}

	// Try to find string label at start
	label := ""
	pos := 0

	// Skip leading whitespace
	for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
		pos++
	}

	if pos >= len(line) {
		return tabLine{}, false
	}

	// Check for string label (single char)
	ch := line[pos : pos+1]
	if _, ok := stringMap[ch]; ok {
		label = ch
		pos++
	} else {
		return tabLine{}, false
	}

	// Skip optional space
	for pos < len(line) && line[pos] == ' ' {
		pos++
	}

	// Must have pipe separator
	if pos >= len(line) || line[pos] != '|' {
		return tabLine{}, false
	}
	pos++

	// Skip optional second pipe (e.g. "e||")
	if pos < len(line) && line[pos] == '|' {
		pos++
	}

	data := line[pos:]
	// Strip trailing pipe if present
	data = strings.TrimRight(data, "|")

	// Validate: data should contain mostly dashes and digits
	dashCount := 0
	for _, c := range data {
		if c == '-' {
			dashCount++
		}
	}
	if dashCount < len(data)/3 {
		return tabLine{}, false
	}

	idx, ok := stringMap[label]
	if !ok {
		return tabLine{}, false
	}

	return tabLine{stringIdx: idx, data: data}, true
}

func isCompleteBlock(block []tabLine) bool {
	seen := [6]bool{}
	for _, tl := range block {
		if tl.stringIdx >= 0 && tl.stringIdx < 6 {
			seen[tl.stringIdx] = true
		}
	}
	for _, s := range seen {
		if !s {
			return false
		}
	}
	return true
}

// parseTabBlock reads fret numbers from a block of 6 string lines.
func parseTabBlock(block []tabLine, tuning [6]uint8, msPerCol float64, globalColOffset int) []dsp.MidiNote {
	// Organize by string index
	var stringData [6]string
	maxLen := 0
	for _, tl := range block {
		stringData[tl.stringIdx] = tl.data
		if len(tl.data) > maxLen {
			maxLen = len(tl.data)
		}
	}

	// Pad all strings to same length
	for i := range stringData {
		for len(stringData[i]) < maxLen {
			stringData[i] += "-"
		}
	}

	var notes []dsp.MidiNote
	noteNames := [12]string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

	col := 0
	for col < maxLen {
		for s := 0; s < 6; s++ {
			if col >= len(stringData[s]) {
				continue
			}
			ch := stringData[s][col]
			if !unicode.IsDigit(rune(ch)) {
				continue
			}

			// Read full fret number (could be 2 digits like "12")
			fretStr := string(ch)
			if col+1 < len(stringData[s]) && unicode.IsDigit(rune(stringData[s][col+1])) {
				fretStr += string(stringData[s][col+1])
			}
			fret, _ := strconv.Atoi(fretStr)

			// Convert (string, fret) to MIDI note
			midiNote := int(tuning[s]) + fret
			if midiNote < 0 || midiNote > 127 {
				continue
			}

			// Determine note duration: count columns until next fret number or end
			durCols := 1
			for nc := col + len(fretStr); nc < len(stringData[s]); nc++ {
				if unicode.IsDigit(rune(stringData[s][nc])) {
					break
				}
				durCols++
			}

			startMs := int(float64(globalColOffset+col) * msPerCol)
			durMs := int(float64(durCols) * msPerCol)
			if durMs < 50 {
				durMs = 50
			}

			// Velocity based on octave
			velocity := autoVelocity(uint8(midiNote))

			// Note name for CSV
			semitone := midiNote % 12
			octave := midiNote/12 - 1
			_ = noteNames
			_ = semitone
			_ = octave

			notes = append(notes, dsp.MidiNote{
				Note:     uint8(midiNote),
				Velocity: velocity,
				StartMs:  startMs,
				DurMs:    durMs,
				Channel:  0,
			})
		}
		col++
	}

	return notes
}

// buildCSV generates a multi-note score CSV from MIDI notes.
// Uses note,start,dur format to support overlapping notes.
func buildCSV(notes []dsp.MidiNote) string {
	noteNames := [12]string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

	var sb strings.Builder
	sb.WriteString("note,start,dur\n")
	for _, n := range notes {
		semitone := int(n.Note) % 12
		octave := int(n.Note)/12 - 1
		name := fmt.Sprintf("%s%d", noteNames[semitone], octave)
		sb.WriteString(fmt.Sprintf("%s,%d,%d\n", name, n.StartMs, n.DurMs))
	}
	return sb.String()
}
