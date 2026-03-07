/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"cmp"

	"fmt"
	"log/slog"
	"math/cmplx"
	"os"
	"slices"

	"github.com/drgolem/musiclab/audiosource"
	"github.com/drgolem/musiclab/dsp"

	"github.com/fale/sit"
	"github.com/spf13/cobra"
	"gonum.org/v1/gonum/dsp/fourier"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"

	"github.com/MicahParks/peakdetect"
)

// fftCmd represents the spectrogram command
var fftCmd = &cobra.Command{
	Use:   "fft",
	Short: "Analyze dominant frequencies and musical notes in an audio file",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: doFftCmd,
}

func init() {
	rootCmd.AddCommand(fftCmd)

	fftCmd.Flags().String("file", "", "file to analyze")
}

func doFftCmd(cmd *cobra.Command, args []string) {
	inFileName, err := cmd.Flags().GetString("file")
	if err != nil {
		slog.Error("failed to get file flag", "error", err)
		return
	}
	if _, err := os.Stat(inFileName); os.IsNotExist(err) {
		slog.Error("path does not exist", "path", inFileName)
		return
	}


	audioData, err := audiosource.AudioSamplesFromFile(inFileName)
	if err != nil {
		slog.Error("failed to read audio samples", "error", err)
		return
	}

	fmt.Printf("Spectrogram: %s\n", inFileName)
	fmt.Printf("Encoding: Signed 16bit\n")
	fmt.Printf("Sample Rate: %d\n", audioData.SampleRate)

	audioSamples := audioData.Audio
	sampleRate := audioData.SampleRate

	nSamples := len(audioSamples)
	//nSamples := 44100 * 4
	//audioSamples = audioSamples[:nSamples]

	// Initialize an FFT and perform the analysis.
	fft := fourier.NewFFT(nSamples)
	coeff := fft.Coefficients(nil, audioSamples)

	spectr := make([]float64, 0)

	var maxFreq, magnitude, mean float64
	for i, c := range coeff {
		m := cmplx.Abs(c)
		spectr = append(spectr, m)
		mean += m
		if m > magnitude {
			magnitude = m
			//maxFreq = fft.Freq(i) * float64(sampleRate) / float64(nSamples)
			//maxFreq = float64(i) * float64(sampleRate) / float64(nSamples)
			maxFreq = fft.Freq(i) * float64(sampleRate)
		}
	}
	fmt.Printf("freq=%v Hz, magnitude=%.0f, mean=%.4f\n",
		maxFreq, magnitude, mean/float64(nSamples))

	type freqMagn struct {
		Freq int
		Magn float64
	}

	pts := make(plotter.XYs, 0)
	for i, m := range spectr {
		//fq := fft.Freq(i) * float64(sampleRate)
		fq := float64(i) * float64(sampleRate) / (2 * float64(len(spectr)))

		if fq < 200.0 || fq > 600.0 {
			continue
		}

		pt := plotter.XY{
			X: fq,
			Y: m,
		}
		pts = append(pts, pt)
	}

	p := plot.New()

	p.Title.Text = "FFT example"
	p.X.Label.Text = "X"
	p.Y.Label.Text = "Y"

	err = plotutil.AddLinePoints(p,
		"FFT", pts,
	)
	if err != nil {
		slog.Error("failed to add line points", "error", err)
		return
	}

	p.Y.Tick.Marker = sit.Ticker{}
	p.Y.Min = sit.Min(p.Y.Min, p.Y.Max)
	p.Y.Max = sit.Max(p.Y.Min, p.Y.Max)

	p.X.Tick.Marker = sit.Ticker{}
	p.X.Min = sit.Min(p.X.Min, p.X.Max)
	p.X.Max = sit.Max(p.X.Min, p.X.Max)

	// Save the plot to a PNG file.
	if err := p.Save(6*vg.Inch, 6*vg.Inch, "points.png"); err != nil {
		slog.Error("failed to save plot", "error", err)
		return
	}

	freqBin := make(map[int]freqMagn, 0)
	for i, m := range spectr {
		fq := float64(i) * float64(sampleRate) / (2 * float64(len(spectr)))

		fqInt := int(fq)

		if fb, ok := freqBin[fqInt]; !ok {
			freqBin[fqInt] = freqMagn{
				Freq: int(fq),
				Magn: m,
			}
		} else {
			fb.Magn += m
			freqBin[fqInt] = fb
		}
	}

	freqArr := make([]freqMagn, 0)
	for _, v := range freqBin {
		freqArr = append(freqArr, v)
	}

	slices.SortFunc(freqArr, func(a freqMagn, b freqMagn) int {
		return cmp.Compare(a.Magn, b.Magn)
	})

	slices.Reverse(freqArr)

	for idx := 0; idx < 20; idx++ {
		fmt.Printf("%d - %v\n", idx, freqArr[idx])
	}

	// Algorithm configuration from example.
	const (
		lag       = 20
		threshold = 6
		influence = 0
	)

	data := spectr

	// Create then initialize the peak detector.
	detector := peakdetect.NewPeakDetector()
	err = detector.Initialize(influence, threshold, data[:lag]) // The length of the initial values is the lag.
	if err != nil {
		slog.Error("failed to initialize peak detector", "error", err)
		return
	}

	spectrNotes := make(map[string]float64)

	// Start processing new data points and determine what signal, if any they produce.
	//
	// This method, .Next(), is best for when data are being processed in a stream, but this simply iterates over a
	// slice.
	nextDataPoints := data[lag:]
	for i, newPoint := range nextDataPoints {
		signal := detector.Next(newPoint)
		var signalType string
		switch signal {
		case peakdetect.SignalNegative:
			signalType = "negative"
		case peakdetect.SignalNeutral:
			signalType = "neutral"
			continue
		case peakdetect.SignalPositive:
			signalType = "positive"
		}

		val := spectr[i+lag]
		if val <= 1.0 {
			continue
		}

		freq := float64(i+lag) * float64(sampleRate) / (2 * float64(len(spectr)))

		note := dsp.FreqToNote(freq)
		spectrNotes[note] += val

		println(fmt.Sprintf("Data point at index %d (%.2f) has the signal: %s, val: %.4f",
			i+lag, freq, signalType, val))
	}

	fmt.Printf("%v\n", spectrNotes)

	spectrNotes = make(map[string]float64)
	for i, m := range spectr {
		freq := float64(i) * float64(sampleRate) / (2 * float64(len(spectr)))

		if freq < 30.0 || freq > 5000.0 {
			continue
		}

		note := dsp.FreqToNote(freq)
		spectrNotes[note] += m
	}
	fmt.Printf("%v\n", spectrNotes)

	// collect peak frequency in octave separated bins

	binPeakFreq := dsp.OctaveBinPeaks(sampleRate, 1.0, spectr)
	h := dsp.PeaksHash(binPeakFreq)

	fmt.Printf("%v\n", binPeakFreq)

	fmt.Printf("hash: %d\n", h)
}


