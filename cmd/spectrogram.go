/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"slices"

	"github.com/spf13/cobra"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/palette/moreland"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"

	"github.com/drgolem/musiclab/audiosource"
	"github.com/drgolem/musiclab/dsp"
)

// spectrogramCmd represents the spectrogram command
var spectrogramCmd = &cobra.Command{
	Use:   "spectrogram",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: doSpectrogramCmd,
}

func init() {
	rootCmd.AddCommand(spectrogramCmd)

	spectrogramCmd.Flags().String("file", "", "file to analyze")
}

func doSpectrogramCmd(cmd *cobra.Command, args []string) {
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

	//nSamples := audioData.SampleRate * 4
	//audioSamples := audioData.Audio[:nSamples]

	audioSamples := audioData.Audio
	sampleRate := audioData.SampleRate

	audioSamplesCopy := slices.Clone(audioSamples)

	stft := dsp.New(
		int(float64(sampleRate)/100.0), // 0.01 sec
		2048,
	)

	stftRes := stft.STFT(audioSamplesCopy)
	spectrogram, _ := dsp.SplitSpectrogram(stftRes)

	maxFreq := getMaxFreq(spectrogram, sampleRate)

	sOut := printMatrixAsGnuplotFormat(spectrogram, sampleRate)

	os.WriteFile(inFileName+".dat", []byte(sOut), 0644)

	hd := hmData{
		mx:         spectrogram,
		sampleRate: sampleRate,
	}

	//pal := palette.Heat(12, 1)
	//pal := moreland.SmoothBlueRed().Palette(255)
	pal := moreland.SmoothBlueRed().Palette(32)
	//pal := palette.Heat(100, 1)
	h := plotter.NewHeatMap(&hd, pal)

	h.Rasterized = true

	p := plot.New()
	p.Title.Text = "Heat map"

	p.Add(h)

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

	p.X.Padding = 0
	p.Y.Padding = 0
	//p.X.Max = 1.5
	//p.Y.Max = 1.5

	p.Y.Max = maxFreq + 100.0
	//p.Y.Max = 8000.0
	//p.Y.Max = 1000.0

	img := vgimg.New(500, 500)
	dc := draw.New(img)
	l.Top = true
	// Calculate the width of the legend.
	r := l.Rectangle(dc)
	legendWidth := r.Max.X - r.Min.X
	l.YOffs = -p.Title.TextStyle.FontExtents().Height // Adjust the legend down a little.

	l.Draw(dc)
	dc = draw.Crop(dc, 0, -legendWidth-vg.Millimeter, 0, 0) // Make space for the legend.

	p.Draw(dc)

	w, err := os.Create(inFileName + ".png")
	if err != nil {
		log.Panic(err)
	}
	png := vgimg.PngCanvas{Canvas: img}
	if _, err = png.WriteTo(w); err != nil {
		log.Panic(err)
	}
}

type hmData struct {
	mx         [][]float64
	sampleRate int
}

func (hm *hmData) Dims() (c, r int) {
	c = len(hm.mx)
	r = len(hm.mx[0])
	return
}

// Z returns the value of a grid value at (c, r).
// It will panic if c or r are out of bounds for the grid.
func (hm *hmData) Z(c, r int) float64 {
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
func (hm *hmData) X(c int) float64 {
	return float64(c)
}

// Y returns the coordinate for the row at the index r.
// It will panic if r is out of bounds for the grid.
func (hm *hmData) Y(r int) float64 {
	return float64(r) * float64(hm.sampleRate) / (2 * 1024)
}

func printMatrixAsGnuplotFormat(matrix [][]float64, sampleRate int) string {
	var buffer bytes.Buffer

	buffer.WriteString(fmt.Sprintln("#", len(matrix[0]), len(matrix)/2))
	for i, vec := range matrix {
		for j, val := range vec[:1024] {
			freq := float64(j) * float64(sampleRate) / (2 * 1024)
			//buffer.WriteString(fmt.Sprintln(i, j, math.Log(val)))
			buffer.WriteString(fmt.Sprintf("%d %.6f %.6f\n", i, freq, math.Log(val)))
		}
		buffer.WriteString(fmt.Sprintln(""))
	}

	return buffer.String()
}

func getMaxFreq(spectrogram [][]float64, sampleRate int) float64 {
	var mx float64

	for _, frame := range spectrogram {
		sz := len(frame)
		for idx, val := range frame {
			if val < 1.0 {
				continue
			}
			freq := float64(idx) * float64(sampleRate) / (2 * float64(sz))
			if freq > mx {
				mx = freq
			}
		}
	}

	return mx
}
