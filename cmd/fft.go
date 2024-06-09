/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"math"
	"math/cmplx"
	"os"
	"slices"

	"github.com/drgolem/musiclab/audiosource"

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
	Short: "A brief description of your command",
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
		panic(err)
	}

	p.Y.Tick.Marker = sit.Ticker{}
	p.Y.Min = sit.Min(p.Y.Min, p.Y.Max)
	p.Y.Max = sit.Max(p.Y.Min, p.Y.Max)

	p.X.Tick.Marker = sit.Ticker{}
	p.X.Min = sit.Min(p.X.Min, p.X.Max)
	p.X.Max = sit.Max(p.X.Min, p.X.Max)

	// Save the plot to a PNG file.
	if err := p.Save(6*vg.Inch, 6*vg.Inch, "points.png"); err != nil {
		panic(err)
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
		log.Fatalf("Failed to initialize peak detector.\nError: %s", err)
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

		note := freqToNote(freq)
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

		note := freqToNote(freq)
		spectrNotes[note] += m
	}
	fmt.Printf("%v\n", spectrNotes)

	// collect peak frequency in octave separated bins

	binPeakFreq := octaveBinPeaks(sampleRate, 1.0, spectr)
	h := peaksHash(binPeakFreq)

	fmt.Printf("%v\n", binPeakFreq)

	fmt.Printf("hash: %d\n", h)
}

func peaksHash(peaks []float64) uint64 {
	h := uint64(0)
	p := uint64(1)
	idx := len(peaks) - 1
	for idx >= 0 {
		// round by clearing last bit
		v := uint64(int64(peaks[idx]) & ^1)
		h *= 1000
		h += v
		p = 1000 * p
		idx--
	}

	return h
}

func octaveBinPeaks(sampleRate int, minAmpl float64, spectr []float64) []float64 {
	binNotesNames := []string{
		"C2", "C3", "C4", "C5", "C6", "C8",
	}

	binNotes := make([]float64, 0)

	for _, nt := range binNotesNames {
		freq := noteToFrequency(nt)
		binNotes = append(binNotes, freq)
	}

	/*
		for idx, nt := range binNotesNames {
			fmt.Printf("%s - %.2f\n", nt, binNotes[idx])
		}
	*/

	freqToBinIndex := func(freq float64) int {
		idx := 0
		for idx < len(binNotes) && binNotes[idx] < freq {
			idx++
		}
		return idx
	}

	binFreqAmpl := make([]float64, len(binNotes)+1)
	binPeakFreq := make([]float64, len(binNotes)+1)

	for i, m := range spectr {
		freq := float64(i) * float64(sampleRate) / (2 * float64(len(spectr)))

		if freq < 30.0 || freq > 5000.0 {
			continue
		}

		if m < minAmpl {
			continue
		}

		idx := freqToBinIndex(freq)
		if binFreqAmpl[idx] < m {
			binFreqAmpl[idx] = m
			binPeakFreq[idx] = freq
		}
	}
	//fmt.Printf("%v\n", binFreqAmpl)
	return binPeakFreq
}

func exampleFft() {
	const (
		//mC      = 261.625565 // Hz
		//mC      = 415.3 // Hz
		sampleRate = 44100
		mC         = 440.0 // Hz
		samples    = sampleRate * 2
		//samples = 32768
		Ampl = 0.7
	)
	tone := make([]float64, samples)
	for i := range tone {
		//tone[i] = Ampl * math.Sin(mC*2*math.Pi*float64(i)/samples)
		tone[i] = Ampl * math.Sin(mC*2*math.Pi*float64(i)/float64(sampleRate))
	}

	fft2 := fourier.NewFFT(samples)
	coeff2 := fft2.Coefficients(nil, tone)

	var maxFreq2, magnitude2, mean2 float64
	for i, c := range coeff2 {
		m := cmplx.Abs(c)
		mean2 += m
		if m > magnitude2 {
			magnitude2 = m
			maxFreq2 = fft2.Freq(i) * float64(sampleRate)
		}
	}
	fmt.Printf("freq=%v Hz, magnitude=%.0f, mean=%.4f\n",
		maxFreq2, magnitude2, mean2/samples)
}
