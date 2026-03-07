/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"cmp"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"os"
	"slices"
	"time"

	"github.com/drgolem/musiclab/dsp"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/spf13/cobra"
)

// samplematchCmd represents the samplematch command
var samplematchCmd = &cobra.Command{
	Use:   "samplematch",
	Short: "Find where a sample occurs in a reference audio file",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: doSamplematchCmd,
}

func init() {
	rootCmd.AddCommand(samplematchCmd)

	samplematchCmd.Flags().String("in", "", "reference file")
	samplematchCmd.Flags().Bool("db", false, "use database for reference data")
	samplematchCmd.Flags().String("sample", "", "sample file to find match in reference file")
}

func doSamplematchCmd(cmd *cobra.Command, args []string) {

	refFile, err := cmd.Flags().GetString("in")
	if err != nil {
		slog.Error("failed to get in flag", "error", err)
		return
	}
	if _, err := os.Stat(refFile); os.IsNotExist(err) {
		slog.Error("path does not exist", "path", refFile)
		return
	}
	smplFile, err := cmd.Flags().GetString("sample")
	if err != nil {
		slog.Error("failed to get sample flag", "error", err)
		return
	}
	if _, err := os.Stat(smplFile); os.IsNotExist(err) {
		slog.Error("path does not exist", "path", smplFile)
		return
	}

	useDb, err := cmd.Flags().GetBool("db")
	if err != nil {
		slog.Error("failed to get db flag", "error", err)
		return
	}

	refSongHashes := make(map[uint64][]SongHashLocator)
	songs := make(map[uint64]string)

	frameShift := 441
	frameSamples := 2048

	if useDb {
		//opts := badger.DefaultOptions("/tmp/badger")
		opts := badger.DefaultOptions(refFile)
		opts.Logger = nil
		db, err := badger.Open(opts)
		if err != nil {
			slog.Error("failed to open database", "error", err)
			return
		}
		defer db.Close()

		err = db.View(func(txn *badger.Txn) error {
			opts := badger.DefaultIteratorOptions
			opts.PrefetchSize = 10
			it := txn.NewIterator(opts)
			defer it.Close()
			prefix := []byte("hash")
			for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
				item := it.Item()
				k := item.Key()
				err := item.Value(func(v []byte) error {
					//fmt.Printf("key=%s, value=%s\n", k, v)
					hash := binary.LittleEndian.Uint64(k[4:])
						sl, err := arrSongLocatorsFromBytes(v)
					if err != nil {
						return err
					}

					refSongHashes[hash] = sl
					return nil
				})
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			slog.Error("failed to read hashes from database", "error", err)
			return
		}

		err = db.View(func(txn *badger.Txn) error {
			opts := badger.DefaultIteratorOptions
			opts.PrefetchSize = 10
			it := txn.NewIterator(opts)
			defer it.Close()
			prefix := []byte("song")
			for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
				item := it.Item()
				k := item.Key()
				err := item.Value(func(v []byte) error {
					//fmt.Printf("key=%s, value=%s\n", k, v)
					idx := binary.LittleEndian.Uint64(k[4:])
					song := string(v)

					songs[idx] = song
					return nil
				})
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			slog.Error("failed to read songs from database", "error", err)
			return
		}

	} else {
		refFileSampleRate, refFileSpectr := getFileSpectrogram(refFile, frameShift, frameSamples)

		refSongIdx := int64(0)
		refSongHashes = spectrToSongHashes(refFileSampleRate, frameShift, refSongIdx, refFileSpectr)
	}

	smplFileSampleRate, smplFileSpectr := getFileSpectrogram(smplFile, frameShift, frameSamples)

	smplSongIdx := int64(1)
	smplSongLocators := spectrToSongLocators(smplFileSampleRate, frameShift, smplSongIdx, smplFileSpectr)

	//sampleRate := smplFileSampleRate

	matchingTimestamps := make([]MatchPosLocator, 0)

	for idx, sl := range smplSongLocators {

		// find hash in ref data
		if refSongLoc, ok := refSongHashes[sl.Hash]; !ok {
			continue
		} else {
			for _, shl := range refSongLoc {
				diffTs := math.Abs(float64(shl.Ts - sl.Ts))

				mpl := MatchPosLocator{
					CandidateTs:  shl.Ts,
					TsDiff:       diffTs,
					Hash:         sl.Hash,
					SamplePosIdx: idx,
					SongIdx:      shl.SongIdx,
				}

				matchingTimestamps = append(matchingTimestamps, mpl)
			}
		}
	}

	diffCount := make(map[int]int)
	diffAgg := make(map[int][]MatchPosLocator)

	for _, mt := range matchingTimestamps {
		tsDiff := int(mt.TsDiff)
		diffCount[tsDiff]++

		if da, ok := diffAgg[tsDiff]; !ok {
			diffAgg[tsDiff] = []MatchPosLocator{mt}
		} else {
			da = append(da, mt)
			diffAgg[tsDiff] = da
		}
	}

	// Create slice of key-value pairs
	pairs := make([][2]int, 0, len(diffCount))

	for k, v := range diffCount {
		pairs = append(pairs, [2]int{k, v})
	}

	// Sort slice based on values

	slices.SortFunc(pairs, func(a, b [2]int) int {
		return cmp.Compare(a[1], b[1])
	})

	slices.Reverse(pairs)

	// Extract sorted keys
	keys := make([]int, len(pairs))

	for i, p := range pairs {
		keys[i] = p[0]
	}

	// Print sorted map
	maxCandidates := 2

	for _, k := range keys {
		//fmt.Printf("%d: %d\n", k, diffCount[k])

		startTs := time.Duration(k) * time.Millisecond

		//fmt.Printf("%v\n", diffAgg[k])

		idx := uint64(diffAgg[k][0].SongIdx)

		fmt.Printf("candidate song: %s\n", songs[idx])
		fmt.Printf("candidate start: %v\n", startTs)

		maxCandidates--
		if maxCandidates == 0 {
			break
		}
	}
}

func spectrToSongHashes(sampleRate int, frameShift int, songIdx int64, spectrogram [][]float64) map[uint64][]SongHashLocator {

	songHashes := make(map[uint64][]SongHashLocator)

	for idx, sl := range spectrogram {
		peaks := dsp.OctaveBinPeaks(sampleRate, 1.0, sl)

		h := dsp.PeaksHash(peaks)
		if h == 0 {
			continue
		}

		timePt := time.Duration(idx*frameShift*1000/sampleRate) * time.Millisecond

		if sh, ok := songHashes[h]; ok {
			sh = append(sh, SongHashLocator{
				Ts:      timePt.Milliseconds(),
				SongIdx: songIdx,
			})
			songHashes[h] = sh
		} else {
			sl := make([]SongHashLocator, 1)

			sl[0] = SongHashLocator{
				Ts:      timePt.Milliseconds(),
				SongIdx: songIdx,
			}
			songHashes[h] = sl
		}
	}

	return songHashes
}

// ordered by timestamp
func spectrToSongLocators(sampleRate int, frameShift int, songIdx int64, spectrogram [][]float64) []SongHashLocator {

	songLocators := make([]SongHashLocator, 0)

	for idx, sl := range spectrogram {
		peaks := dsp.OctaveBinPeaks(sampleRate, 1.0, sl)

		h := dsp.PeaksHash(peaks)
		if h == 0 {
			continue
		}

		timePt := time.Duration(idx*frameShift*1000/sampleRate) * time.Millisecond

		sl := SongHashLocator{
			Ts:      timePt.Milliseconds(),
			SongIdx: songIdx,
			Hash:    h,
		}

		songLocators = append(songLocators, sl)
	}

	return songLocators
}
