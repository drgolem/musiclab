/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"math/cmplx"
	"os"
	"slices"

	"github.com/drgolem/musiclab/audiosource"
	"github.com/spf13/cobra"
	"gonum.org/v1/gonum/dsp/fourier"

	"github.com/fale/sit"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
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

	//nSamples := len(audioSamples)

	nSamples := 44100 * 4
	audioSamples = audioSamples[:nSamples]

	// Initialize an FFT and perform the analysis.
	fft := fourier.NewFFT(nSamples)
	coeff := fft.Coefficients(nil, audioSamples)

	var maxFreq, magnitude, mean float64
	for i, c := range coeff {
		m := cmplx.Abs(c)
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
	for i, c := range coeff {
		m := cmplx.Abs(c)
		fq := fft.Freq(i) * float64(sampleRate)

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

	p.Title.Text = "Plotutil example"
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
	for i, c := range coeff {
		m := cmplx.Abs(c)
		fq := fft.Freq(i) * float64(sampleRate)

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

	const (
		//mC      = 261.625565 // Hz
		//mC      = 415.3 // Hz
		mC      = 440.0 // Hz
		samples = 44100 * 2
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
