package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/drgolem/musiclab/acoustid"
	"github.com/drgolem/musiclab/audiosource"
	"github.com/drgolem/musiclab/dsp"
)

var fingerprintCmd = &cobra.Command{
	Use:   "fingerprint [file]",
	Short: "Generate audio fingerprint and optionally look up on AcoustID",
	Args:  cobra.ExactArgs(1),
	Run:   doFingerprintCmd,
}

func init() {
	rootCmd.AddCommand(fingerprintCmd)

	fingerprintCmd.Flags().Bool("raw", false, "output raw subfingerprints (unsigned decimal, fpcalc-compatible)")
	fingerprintCmd.Flags().Bool("signed", false, "output raw subfingerprints as signed integers (for pg_acoustid)")
	fingerprintCmd.Flags().Bool("lookup", false, "submit to AcoustID and print matches")
	fingerprintCmd.Flags().String("api-key", "", "AcoustID API key (or ACOUSTID_API_KEY env)")
	fingerprintCmd.Flags().Bool("json", false, "output as JSON (fpcalc-compatible format)")
	fingerprintCmd.Flags().Bool("text", false, "output as DURATION=N / FINGERPRINT=... text (fpcalc default)")
	fingerprintCmd.Flags().Bool("plain", false, "output just the fingerprint string")
	fingerprintCmd.Flags().Int("length", 120, "max duration to process in seconds")
	fingerprintCmd.Flags().Bool("fpcalc", false, "use external fpcalc binary instead of native implementation")
}

func doFingerprintCmd(cmd *cobra.Command, args []string) {
	inFileName := args[0]
	if _, err := os.Stat(inFileName); os.IsNotExist(err) {
		slog.Error("file does not exist", "path", inFileName)
		return
	}

	raw, _ := cmd.Flags().GetBool("raw")
	signed, _ := cmd.Flags().GetBool("signed")
	doLookup, _ := cmd.Flags().GetBool("lookup")
	apiKey, _ := cmd.Flags().GetString("api-key")
	jsonOutput, _ := cmd.Flags().GetBool("json")
	textOutput, _ := cmd.Flags().GetBool("text")
	plainOutput, _ := cmd.Flags().GetBool("plain")
	maxLength, _ := cmd.Flags().GetInt("length")
	useFpcalc, _ := cmd.Flags().GetBool("fpcalc")

	if apiKey == "" {
		apiKey = os.Getenv("ACOUSTID_API_KEY")
	}

	if useFpcalc {
		doFingerprintWithFpcalc(inFileName, raw, doLookup, apiKey, jsonOutput)
		return
	}

	audioData, err := audiosource.AudioSamplesFromFile(inFileName)
	if err != nil {
		slog.Error("failed to read audio file", "error", err)
		return
	}

	// Truncate audio to maxLength seconds
	maxSamples := maxLength * audioData.SampleRate
	if len(audioData.Audio) > maxSamples {
		audioData.Audio = audioData.Audio[:maxSamples]
	}

	fp := dsp.NewFingerprinter(audioData.SampleRate)
	subfingerprints, err := fp.Fingerprint(audioData.Audio)
	if err != nil {
		slog.Error("fingerprint generation failed", "error", err)
		return
	}

	duration := len(audioData.Audio) / audioData.SampleRate

	if raw || signed {
		// fpcalc-compatible raw output: DURATION=N\nFINGERPRINT=num,num,...
		fmt.Printf("DURATION=%d\n", duration)
		fmt.Print("FINGERPRINT=")
		for i, sfp := range subfingerprints {
			if i > 0 {
				fmt.Print(",")
			}
			if signed {
				fmt.Print(int32(sfp))
			} else {
				fmt.Print(sfp)
			}
		}
		fmt.Println()
		return
	}

	encoded, err := dsp.EncodeFingerprint(subfingerprints)
	if err != nil {
		slog.Error("fingerprint encoding failed", "error", err)
		return
	}

	if plainOutput {
		fmt.Println(encoded)
		return
	}

	if jsonOutput {
		// fpcalc-compatible JSON: {"duration": float, "fingerprint": "..."}
		jsonStr := fmt.Sprintf("{\"duration\": %.2f, \"fingerprint\": \"%s\"}\n",
			float64(len(audioData.Audio))/float64(audioData.SampleRate), encoded)
		fmt.Print(jsonStr)
		return
	}

	if textOutput {
		// fpcalc default text format
		fmt.Printf("DURATION=%d\n", duration)
		fmt.Printf("FINGERPRINT=%s\n", encoded)
	} else {
		// Our extended default (shows file and frame count)
		fmt.Printf("DURATION=%d\n", duration)
		fmt.Printf("FINGERPRINT=%s\n", encoded)
	}

	if doLookup {
		if apiKey == "" {
			slog.Error("AcoustID API key required for lookup (use --api-key or ACOUSTID_API_KEY env)")
			return
		}
		lookupFingerprint(apiKey, encoded, duration, jsonOutput)
	}
}

// doFingerprintWithFpcalc uses the reference fpcalc binary for AcoustID-compatible fingerprints.
func doFingerprintWithFpcalc(inFileName string, raw, doLookup bool, apiKey string, jsonOutput bool) {
	fpcalcPath, err := exec.LookPath("fpcalc")
	if err != nil {
		slog.Error("fpcalc not found in PATH (install chromaprint: brew install chromaprint)")
		return
	}

	fpcalcArgs := []string{inFileName}
	if raw {
		fpcalcArgs = append(fpcalcArgs, "-raw")
	}

	out, err := exec.Command(fpcalcPath, fpcalcArgs...).Output()
	if err != nil {
		slog.Error("fpcalc failed", "error", err)
		return
	}

	// Parse fpcalc output: DURATION=N\nFINGERPRINT=...\n
	var duration int
	var fingerprint string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "DURATION=") {
			duration, _ = strconv.Atoi(strings.TrimPrefix(line, "DURATION="))
		} else if strings.HasPrefix(line, "FINGERPRINT=") {
			fingerprint = strings.TrimPrefix(line, "FINGERPRINT=")
		}
	}

	if raw {
		fmt.Print(string(out))
		return
	}

	if jsonOutput {
		outMap := map[string]interface{}{
			"file":        inFileName,
			"duration":    duration,
			"fingerprint": fingerprint,
			"backend":     "fpcalc",
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(outMap); err != nil {
			slog.Error("JSON encoding failed", "error", err)
		}
	} else {
		fmt.Printf("File: %s\n", inFileName)
		fmt.Printf("Duration: %ds\n", duration)
		fmt.Printf("Fingerprint (fpcalc): %s\n", fingerprint)
	}

	if doLookup {
		if apiKey == "" {
			slog.Error("AcoustID API key required for lookup (use --api-key or ACOUSTID_API_KEY env)")
			return
		}
		lookupFingerprint(apiKey, fingerprint, duration, jsonOutput)
	}
}

func lookupFingerprint(apiKey, fingerprint string, duration int, jsonOutput bool) {
	client := acoustid.NewClient(apiKey)
	results, err := client.Lookup(context.Background(), fingerprint, duration)
	if err != nil {
		slog.Error("AcoustID lookup failed", "error", err)
		return
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(results); err != nil {
			slog.Error("JSON encoding failed", "error", err)
		}
		return
	}

	if len(results) == 0 {
		fmt.Println("No matches found.")
		return
	}

	for _, r := range results {
		fmt.Printf("\nMatch (score: %.2f, ID: %s)\n", r.Score, r.ID)
		for _, rec := range r.Recordings {
			artists := ""
			for i, a := range rec.Artists {
				if i > 0 {
					artists += ", "
				}
				artists += a.Name
			}
			fmt.Printf("  Recording: %s - %s\n", artists, rec.Title)
			for _, rel := range rec.Releases {
				fmt.Printf("    Release: %s\n", rel.Title)
			}
		}
	}
}
