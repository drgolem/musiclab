package cmd

import (
	"fmt"
	"log/slog"
	"math"
	"os"
	"strings"

	"github.com/drgolem/musiclab/dsp"
	"github.com/spf13/cobra"
)

// Standard guitar tuning: string 0 (low E) to string 5 (high E)
var standardTuning = [6]uint8{40, 45, 50, 55, 59, 64} // E2, A2, D3, G3, B3, E4
var stringNames = [6]string{"E", "A", "D", "G", "B", "e"}

var tabsCmd = &cobra.Command{
	Use:   "tabs",
	Short: "Convert MIDI or score CSV to guitar tablature",
	Long: `Convert MIDI files or score CSV files to ASCII guitar tablature.

Examples:
  musiclab tabs --midi song.mid
  musiclab tabs --score scores/stairway_to_heaven.csv
  musiclab tabs --midi song.mid --capo 2
  musiclab tabs --midi song.mid --tuning -1   # half-step down`,
	Run: doTabsCmd,
}

func init() {
	rootCmd.AddCommand(tabsCmd)

	tabsCmd.Flags().String("midi", "", "input MIDI file (.mid)")
	tabsCmd.Flags().String("score", "", "input score CSV file (note,dur format)")
	tabsCmd.Flags().String("out", "", "output file (default: stdout)")
	tabsCmd.Flags().Int("capo", 0, "capo fret position")
	tabsCmd.Flags().Int("tuning", 0, "tuning offset in semitones (e.g. -1 for half-step down)")
	tabsCmd.Flags().Int("width", 80, "tab line width before wrapping")
	tabsCmd.Flags().Int("max-fret", 22, "maximum fret number")
}

func doTabsCmd(cmd *cobra.Command, args []string) {
	scoreFile, _ := cmd.Flags().GetString("score")
	midiFile, _ := cmd.Flags().GetString("midi")
	outFile, _ := cmd.Flags().GetString("out")
	capo, _ := cmd.Flags().GetInt("capo")
	tuningOffset, _ := cmd.Flags().GetInt("tuning")
	width, _ := cmd.Flags().GetInt("width")
	maxFret, _ := cmd.Flags().GetInt("max-fret")

	if scoreFile == "" && midiFile == "" {
		slog.Error("--score or --midi is required")
		return
	}

	// Build tuning with offset
	var tuning [6]uint8
	for i, n := range standardTuning {
		tuning[i] = uint8(int(n) + tuningOffset)
	}

	var notes []dsp.MidiNote
	if midiFile != "" {
		n, _, err := dsp.ReadMIDINotes(midiFile)
		if err != nil {
			slog.Error("failed to read MIDI", "error", err)
			return
		}
		notes = n
		slog.Info("Read MIDI", "notes", len(notes))
	} else {
		n, err := scoreToMidiNotes(scoreFile)
		if err != nil {
			slog.Error("failed to parse score", "error", err)
			return
		}
		notes = n
	}

	if len(notes) == 0 {
		slog.Error("no notes found")
		return
	}

	tab := notesToTab(notes, tuning, capo, maxFret, width)

	if outFile != "" {
		if err := os.WriteFile(outFile, []byte(tab), 0644); err != nil {
			slog.Error("failed to write output", "error", err)
			return
		}
		slog.Info("Written", "path", outFile)
	} else {
		fmt.Print(tab)
	}
}

type tabSlot struct {
	frets [6]int // -1 = not played
}

// fingering represents one possible way to play a set of notes.
type fingering struct {
	frets    [6]int  // -1 = not played
	position float64 // average fret of played notes (hand position)
	stretch  int     // max fret - min fret among played frets
}

func notesToTab(notes []dsp.MidiNote, tuning [6]uint8, capo, maxFret, lineWidth int) string {
	// Group notes into time slots (notes within 30ms are simultaneous)
	const groupThresholdMs = 30
	var noteGroups [][]uint8

	i := 0
	for i < len(notes) {
		startMs := notes[i].StartMs
		var group []uint8
		for i < len(notes) && notes[i].StartMs-startMs < groupThresholdMs {
			group = append(group, notes[i].Note)
			i++
		}
		// Sort low to high
		for a := 1; a < len(group); a++ {
			for b := a; b > 0 && group[b] < group[b-1]; b-- {
				group[b], group[b-1] = group[b-1], group[b]
			}
		}
		noteGroups = append(noteGroups, group)
	}

	// Enumerate all valid fingerings for each time slot
	allFingerings := make([][]fingering, len(noteGroups))
	for t, group := range noteGroups {
		allFingerings[t] = enumerateFingerings(group, tuning, capo, maxFret)
		if len(allFingerings[t]) == 0 {
			// Fallback: at least one empty slot
			allFingerings[t] = []fingering{{frets: [6]int{-1, -1, -1, -1, -1, -1}}}
		}
	}

	// Viterbi DP: find minimum-cost path through fingerings
	slots := viterbiOptimalPath(allFingerings)

	// Render ASCII tab
	return renderTab(slots, lineWidth)
}

// enumerateFingerings generates all valid (string, fret) assignments for a group of notes.
// For single notes: one fingering per possible position.
// For chords: recursive assignment, pruned to max 50 candidates.
func enumerateFingerings(noteGroup []uint8, tuning [6]uint8, capo, maxFret int) []fingering {
	if len(noteGroup) == 0 {
		return nil
	}

	// For each note, find all playable (string, fret) pairs
	type strFret struct {
		str  int
		fret int
	}
	noteOptions := make([][]strFret, len(noteGroup))
	for i, note := range noteGroup {
		for s := 0; s < 6; s++ {
			fret := int(note) - int(tuning[s]) - capo
			if fret >= 0 && fret <= maxFret {
				noteOptions[i] = append(noteOptions[i], strFret{s, fret})
			}
		}
		if len(noteOptions[i]) == 0 {
			return nil // unplayable note
		}
	}

	// Recursively enumerate valid assignments (each string used at most once)
	var results []fingering
	var assign func(noteIdx int, frets [6]int, usedStrings uint8)
	assign = func(noteIdx int, frets [6]int, usedStrings uint8) {
		if len(results) >= 50 {
			return // cap to prevent explosion
		}
		if noteIdx == len(noteGroup) {
			f := makeFingering(frets)
			results = append(results, f)
			return
		}
		for _, opt := range noteOptions[noteIdx] {
			if usedStrings&(1<<opt.str) != 0 {
				continue
			}
			newFrets := frets
			newFrets[opt.str] = opt.fret
			assign(noteIdx+1, newFrets, usedStrings|(1<<opt.str))
		}
	}

	baseFrets := [6]int{-1, -1, -1, -1, -1, -1}
	assign(0, baseFrets, 0)
	return results
}

func makeFingering(frets [6]int) fingering {
	minF, maxF := 100, -1
	sum, count := 0.0, 0
	for _, f := range frets {
		if f < 0 {
			continue
		}
		if f > 0 { // open strings don't count for stretch
			if f < minF {
				minF = f
			}
			if f > maxF {
				maxF = f
			}
		}
		sum += float64(f)
		count++
	}
	pos := 0.0
	if count > 0 {
		pos = sum / float64(count)
	}
	stretch := 0
	if maxF > minF && minF < 100 {
		stretch = maxF - minF
	}
	return fingering{frets: frets, position: pos, stretch: stretch}
}

// viterbiOptimalPath finds the sequence of fingerings that minimizes total cost.
// Cost = stretch penalty + hand jump penalty + high fret penalty.
func viterbiOptimalPath(allFingerings [][]fingering) []tabSlot {
	n := len(allFingerings)
	if n == 0 {
		return nil
	}

	const (
		stretchWeight = 2.0  // penalty per fret of stretch
		jumpWeight    = 1.5  // penalty per fret of hand movement
		highFretWeight = 0.1 // slight preference for lower positions
	)

	fingeringCost := func(f fingering) float64 {
		return float64(f.stretch)*stretchWeight + f.position*highFretWeight
	}

	transitionCost := func(prev, curr fingering) float64 {
		return math.Abs(prev.position-curr.position) * jumpWeight
	}

	// DP tables
	costs := make([][]float64, n)
	backPtr := make([][]int, n)

	// Initialize first slot
	costs[0] = make([]float64, len(allFingerings[0]))
	backPtr[0] = make([]int, len(allFingerings[0]))
	for j, f := range allFingerings[0] {
		costs[0][j] = fingeringCost(f)
		backPtr[0][j] = -1
	}

	// Forward pass
	for t := 1; t < n; t++ {
		costs[t] = make([]float64, len(allFingerings[t]))
		backPtr[t] = make([]int, len(allFingerings[t]))
		for j, curr := range allFingerings[t] {
			bestCost := math.Inf(1)
			bestPrev := 0
			baseCost := fingeringCost(curr)
			for k, prev := range allFingerings[t-1] {
				c := costs[t-1][k] + baseCost + transitionCost(prev, curr)
				if c < bestCost {
					bestCost = c
					bestPrev = k
				}
			}
			costs[t][j] = bestCost
			backPtr[t][j] = bestPrev
		}
	}

	// Backtrace: find best final state
	bestFinal := 0
	bestFinalCost := math.Inf(1)
	for j, c := range costs[n-1] {
		if c < bestFinalCost {
			bestFinalCost = c
			bestFinal = j
		}
	}

	// Reconstruct path
	path := make([]int, n)
	path[n-1] = bestFinal
	for t := n - 2; t >= 0; t-- {
		path[t] = backPtr[t+1][path[t+1]]
	}

	// Convert to tabSlots
	slots := make([]tabSlot, n)
	for t, idx := range path {
		slots[t] = tabSlot{frets: allFingerings[t][idx].frets}
	}
	return slots
}

func renderTab(slots []tabSlot, lineWidth int) string {
	var result strings.Builder

	slotsPerLine := (lineWidth - 3) / 4
	if slotsPerLine < 1 {
		slotsPerLine = 1
	}

	for lineStart := 0; lineStart < len(slots); lineStart += slotsPerLine {
		lineEnd := lineStart + slotsPerLine
		if lineEnd > len(slots) {
			lineEnd = len(slots)
		}

		for s := 5; s >= 0; s-- {
			result.WriteString(stringNames[s])
			result.WriteString("|")
			for _, slot := range slots[lineStart:lineEnd] {
				if slot.frets[s] < 0 {
					result.WriteString("---")
				} else if slot.frets[s] >= 10 {
					result.WriteString(fmt.Sprintf("%d-", slot.frets[s]))
				} else {
					result.WriteString(fmt.Sprintf("-%d-", slot.frets[s]))
				}
			}
			result.WriteString("-|\n")
		}
		result.WriteString("\n")
	}

	return result.String()
}
