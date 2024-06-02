/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"

	"github.com/drgolem/musiclab/audiosource"
	"github.com/drgolem/musiclab/dsp"
	"github.com/spf13/cobra"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/palette/moreland"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"
)

// chromagramCmd represents the chromagram command
var chromagramCmd = &cobra.Command{
	Use:   "chromagram",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: doChromagramCmd,
}

func init() {
	rootCmd.AddCommand(chromagramCmd)

	chromagramCmd.Flags().String("file", "", "file to analyze")
}

type noteInterval struct {
	Note   string
	MinIdx int
	MaxIdx int
}

func doChromagramCmd(cmd *cobra.Command, args []string) {
	inFileName, err := cmd.Flags().GetString("file")
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	if _, err := os.Stat(inFileName); os.IsNotExist(err) {
		fmt.Printf("path [%s] does not exist\n", inFileName)
		return
	}

	ctx := context.Background()
	audioData, err := audiosource.AudioSamplesFromFile(ctx, inFileName)
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}

	fmt.Printf("Spectrogram: %s\n", inFileName)
	fmt.Printf("Encoding: Signed 16bit\n")
	fmt.Printf("Sample Rate: %d\n", audioData.SampleRate)

	audioSamples := audioData.Audio
	sampleRate := audioData.SampleRate

	//wndSamplesLen := 2048 * 2
	wndSamplesLen := 2048 * 2

	stff := dsp.New(
		int(float64(sampleRate)/100.0), // 0.01 sec
		wndSamplesLen,
	)

	stffRes2 := stff.STFT(audioSamples)
	spectrogram, _ := dsp.SplitSpectrogram(stffRes2)

	noteIntervals := make([]noteInterval, 0)

	mx := make([][]float64, 0)

	for frameIdx, freqFrame := range spectrogram {
		freqMap := make(map[string]float64)
		for idx, sf := range freqFrame {
			if sf < 1.0 {
				continue
			}
			freq := float64(idx) * float64(sampleRate) / float64(wndSamplesLen)
			if math.Abs(freq) < 0.1 {
				continue
			}
			note := freqToNote(freq)
			freqMap[note] += sf
		}
		if len(freqMap) == 0 {
			continue
		}

		maxNote := getMaxNote(freqMap)
		//fmt.Printf("%d - %v\n", frameIdx, maxNote)

		freqArray := freqMapToArray(freqMap)
		mx = append(mx, freqArray)

		if len(noteIntervals) == 0 {
			noteIntervals = append(noteIntervals, noteInterval{
				Note:   maxNote,
				MinIdx: frameIdx,
				MaxIdx: frameIdx,
			})
		} else {
			if noteIntervals[len(noteIntervals)-1].Note == maxNote {
				noteIntervals[len(noteIntervals)-1].MaxIdx = frameIdx
			} else {
				noteIntervals = append(noteIntervals, noteInterval{
					Note:   maxNote,
					MinIdx: frameIdx,
					MaxIdx: frameIdx,
				})
			}
		}

		//if frameIdx >= 450 {
		//	break
		//}
	}

	for _, ntInt := range noteIntervals {
		fmt.Printf("%s - [%d:%d]\n", ntInt.Note, ntInt.MinIdx, ntInt.MaxIdx)
	}

	hd := chromaData{
		mx:         mx,
		sampleRate: sampleRate,
	}

	//pal := palette.Heat(12, 1)
	//pal := moreland.SmoothBlueRed().Palette(255)
	pal := moreland.SmoothBlueRed().Palette(32)
	//pal := palette.Heat(100, 1)
	h := plotter.NewHeatMap(&hd, pal)

	h.Rasterized = true

	p := plot.New()
	p.Title.Text = "Chromagram " + inFileName

	p.Add(h)

	/*
		// Create a legend.
		l := plot.NewLegend()
		thumbs := plotter.PaletteThumbnailers(pal)
		for i := len(thumbs) - 1; i >= 0; i-- {
			t := thumbs[i]
			if i != 0 && i != len(thumbs)-1 {
				l.Add("", t)
				continue
			}
			var val float64
			switch i {
			case 0:
				val = h.Min
			case len(thumbs) - 1:
				val = h.Max
			}
			l.Add(fmt.Sprintf("%.2f", val), t)
		}
	*/

	p.X.Padding = 0
	p.Y.Padding = 0
	//p.X.Max = 12
	//p.Y.Max = 12

	tickerFunc := func(min, max float64) []plot.Tick {
		ticks := []plot.Tick{
			{
				Label: "C",
				Value: 0,
			},
			{
				Label: "C#",
				Value: 1,
			},
			{
				Label: "D",
				Value: 2,
			},
			{
				Label: "D#",
				Value: 3,
			},
			{
				Label: "E",
				Value: 4,
			},
			{
				Label: "F",
				Value: 5,
			},
			{
				Label: "F#",
				Value: 6,
			},
			{
				Label: "G",
				Value: 7,
			},
			{
				Label: "G#",
				Value: 8,
			},
			{
				Label: "A",
				Value: 9,
			},
			{
				Label: "A#",
				Value: 10,
			},
			{
				Label: "B",
				Value: 11,
			},
		}
		return ticks
	}

	p.Y.Tick.Marker = plot.TickerFunc(tickerFunc)

	//p.Y.Max = 10000.0 / 2
	//p.Y.Max = 1000.0

	img := vgimg.New(500, 500)
	dc := draw.New(img)
	//l.Top = true
	// Calculate the width of the legend.
	//r := l.Rectangle(dc)
	//legendWidth := r.Max.X - r.Min.X
	//l.YOffs = -p.Title.TextStyle.FontExtents().Height // Adjust the legend down a little.

	//l.Draw(dc)
	//dc = draw.Crop(dc, 0, -legendWidth-vg.Millimeter, 0, 0) // Make space for the legend.

	p.Draw(dc)

	w, err := os.Create(inFileName + ".chroma.png")
	if err != nil {
		log.Panic(err)
	}
	png := vgimg.PngCanvas{Canvas: img}
	if _, err = png.WriteTo(w); err != nil {
		log.Panic(err)
	}
}

func freqMapToArray(freq map[string]float64) []float64 {
	notes := []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

	out := make([]float64, 0)

	for _, nt := range notes {
		val := 0.0
		if v, ok := freq[nt]; ok {
			val = v
		}
		out = append(out, val)
	}

	return out
}

func getMaxNote(mp map[string]float64) string {
	mx := -math.MaxFloat64
	note := ""
	for k, v := range mp {
		if v > mx {
			mx = v
			note = k
		}
	}
	return note
}

func freqToNote(freq float64) string {

	notes := []string{"B3", "C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B", "C5"}

	c4 := noteToFrequency("C4")
	b4 := noteToFrequency("B4")

	prec := 15.55

	for {
		if freq <= b4+prec {
			break
		}
		freq = freq / 2
	}

	for {
		if freq >= c4-prec {
			break
		}
		freq = freq * 2
	}

	prec /= 2
	idx := 0
	for idx < len(notes)-1 {
		nl := noteToFrequency(notes[idx])
		nr := noteToFrequency(notes[idx+1])
		if freq >= nl && freq < nr {
			diff := math.Abs(nr-nl) / 2
			if freq < nl+diff {
				return notes[idx]
			} else {
				return notes[idx+1]
			}
			/*
				closeL := math.Abs(freq - nl)
				closeR := math.Abs(freq - nr)

				if closeL < closeR {
					return notes[idx]
				} else {
					return notes[idx+1]
				}
			*/
		}
		idx++
	}

	return "C4"
}

type chromaData struct {
	mx         [][]float64
	sampleRate int
}

func (hm *chromaData) Dims() (c, r int) {
	c = len(hm.mx)
	r = len(hm.mx[0])
	return
}

// Z returns the value of a grid value at (c, r).
// It will panic if c or r are out of bounds for the grid.
func (hm *chromaData) Z(c, r int) float64 {
	val := hm.mx[c][r]
	/*
		if val < 1.0 {
			val = 0
		}
	*/
	return val
}

// X returns the coordinate for the column at the index c.
// It will panic if c is out of bounds for the grid.
func (hm *chromaData) X(c int) float64 {
	return float64(c)
}

// Y returns the coordinate for the row at the index r.
// It will panic if r is out of bounds for the grid.
func (hm *chromaData) Y(r int) float64 {
	//return float64(r) * float64(hm.sampleRate) / (2 * 1024)
	return float64(r)
}
