# How-To Guide

## 1. Scan a Music Library and Match Samples

### Build the spectral hash database

Scan a music folder and index all songs by their spectral hashes:

```bash
musiclab db --scan --music-root /path/to/music --db spectr.db
```

This walks the folder recursively, decodes each audio file, computes STFT spectrograms, extracts octave-bin peak frequencies per frame, and stores the hashes in a Badger database.

**Supported formats:** MP3, FLAC, WAV, OGG (Vorbis/Opus)

### Find a sample in the database

Given a short audio clip, find which song it belongs to and where:

```bash
musiclab samplematch --in spectr.db --db --sample clip.wav
```

This computes spectral hashes for the sample, looks them up in the database, and reports the top matching songs with estimated start positions.

### Match against a single reference file (no database)

```bash
musiclab samplematch --in reference.flac --sample clip.wav
```

## 2. Audio Fingerprinting with Chromaprint

### Generate a fingerprint

```bash
musiclab fingerprint song.mp3
```

Output: file path, duration, frame count, and base64-encoded Chromaprint fingerprint.

### Drop-in replacement for fpcalc

`musiclab fingerprint` produces output in the same formats as the reference `fpcalc` CLI:

```bash
# Default text output (same as fpcalc)
musiclab fingerprint song.mp3
# DURATION=210
# FINGERPRINT=AQADtNI...

# JSON output (matches fpcalc -json)
musiclab fingerprint song.mp3 --json
# {"duration": 210.00, "fingerprint": "AQADtNI..."}

# Raw subfingerprints (matches fpcalc -raw)
musiclab fingerprint song.mp3 --raw

# Signed int32 format
musiclab fingerprint song.mp3 --signed

# Limit analysis duration (like fpcalc -length)
musiclab fingerprint song.mp3 --length 30

# Plain base64 only (no header)
musiclab fingerprint song.mp3 --plain
```

### Compare with reference fpcalc

```bash
# Use external fpcalc binary for comparison
musiclab fingerprint song.mp3 --fpcalc
```

### Look up on AcoustID

Register for an API key at https://acoustid.org/new-application, then:

```bash
# Via environment variable
export ACOUSTID_API_KEY=your_key
musiclab fingerprint song.mp3 --lookup

# Via flag
musiclab fingerprint song.mp3 --lookup --api-key your_key

# JSON output with lookup
musiclab fingerprint song.mp3 --lookup --json
```

Returns MusicBrainz metadata: artist, title, album, and match score.

### Compare with reference fpcalc

To verify fingerprint compatibility with the reference C implementation:

```bash
# Our implementation
musiclab fingerprint song.flac --json

# Reference fpcalc
musiclab fingerprint song.flac --fpcalc --json

# Or directly
fpcalc song.flac
```

The native Go implementation produces ~99-100% matching fingerprints with fpcalc across all supported formats.

## 3. Other Commands

### FFT frequency analysis

```bash
musiclab fft --in song.wav
```

Analyzes dominant frequencies and maps them to musical notes.

### Chromagram visualization

```bash
musiclab chromagram --in song.wav --out chromagram.png
```

Generates a 12-bin chroma heatmap showing note intensity over time.

### Spectrogram visualization

```bash
musiclab spectrogram --in song.wav --out spectrogram.png
```

Generates a frequency-domain heatmap with octave-bin peak overlay.

### Synthesize audio from score

```bash
# List available scores
musiclab doremi --list

# Simple format CSV (columns: note, duration_ms)
musiclab doremi --score scores/greensleeves.csv --out greensleeves.wav

# Multi-track format CSV (columns: track, wave, note, start_ms, dur_ms)
musiclab doremi --score scores/canon_harmony.csv --out canon.wav

# Choose waveform: sine (default), square, triangle, sawtooth, guitar, pluck
musiclab doremi --score scores/greensleeves.csv --wave=guitar --out greensleeves.wav

# Pipe to player via stdout
musiclab doremi --score scores/greensleeves.csv --wave=guitar --stdout | musictools play -
```

> `musictools` is a companion CLI for audio playback and conversion: [github.com/drgolem/musictools](https://github.com/drgolem/musictools)

**Waveforms:**

| Name       | Sound                                            |
|------------|--------------------------------------------------|
| `sine`     | Pure tone, smooth                                |
| `square`   | Hollow, clarinet-like                            |
| `triangle` | Soft, recorder-like                              |
| `sawtooth` | Bright, buzzy                                    |
| `guitar`   | Plucked string (Karplus-Strong synthesis)        |
| `pluck`    | Same as guitar                                   |

**Score formats:**

Simple (sequential notes):
```csv
note,dur
C4,500
D4,500
E4,1000
```

Multi-track (overlapping instruments):
```csv
track,wave,note,start,dur
0,sine,C4,0,1000
1,triangle,E4,0,1000
0,sine,D4,1000,1000
```

**Available scores:** greensleeves, ode_to_joy, fur_elise, fur_elise_harmony, canon_in_d, canon_harmony, happy_birthday, mary_had_a_little_lamb, twinkle_twinkle, chord_demo

### Render audio with SoundFont instruments (MIDI)

Render score CSV or MIDI files using SoundFont (.sf2) instrument samples for realistic sound.

#### Getting a SoundFont

You need a `.sf2` file for MIDI rendering. Recommended free/redistributable SoundFonts:

| SoundFont | License | Size | Best for | Download |
|-----------|---------|------|----------|----------|
| **FluidR3_GM** | MIT-like | ~141 MB | General MIDI (all instruments) | [GitHub](https://github.com/FluidSynth/fluidr3) or `apt install fluid-soundfont-gm` |
| **GeneralUser GS** | Free to redistribute | ~30 MB | General MIDI, good quality/size ratio | [schristiancollins.com](https://www.schristiancollins.com/generaluser.php) |
| **MuseScore_General** | MIT | ~200 MB | Orchestra, piano, high quality | [MuseScore GitHub](https://github.com/musescore/MuseScore/tree/master/share/sound) |
| **Real Nylon Guitar** | Free | ~3 MB | Classical/nylon guitar | [Musical Artifacts](https://musical-artifacts.com/artifacts/3673) |

```bash
# macOS / Linux: install FluidR3 via package manager
# Ubuntu/Debian
sudo apt install fluid-soundfont-gm
# The file is at /usr/share/sounds/sf2/FluidR3_GM.sf2

# Or download GeneralUser GS manually
curl -LO https://www.schristiancollins.com/download/GeneralUser_GS_v1.471.zip
unzip GeneralUser_GS_v1.471.zip
```

#### Usage

```bash
# Render a score CSV with a SoundFont
musiclab midi --sf2 FluidR3_GM.sf2 --score scores/greensleeves.csv --out greensleeves.wav

# Render a MIDI file
musiclab midi --sf2 GeneralUser_GS.sf2 --midi song.mid --out song.wav

# Pipe to player
musiclab midi --sf2 FluidR3_GM.sf2 --score scores/greensleeves.csv --stdout | musictools play -

# Export score CSV as a standard MIDI file (no SF2 needed)
musiclab midi --score scores/greensleeves.csv --export-midi greensleeves.mid

# Set tempo (for score CSV), instrument program, and bank
musiclab midi --sf2 FluidR3_GM.sf2 --score scores/ode_to_joy.csv --tempo 100 --program 0 --bank 0 --out ode_piano.wav
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--sf2` | SoundFont (.sf2) file for instrument rendering |
| `--score` | Input score CSV file (note,dur format) |
| `--midi` | Input MIDI file (.mid) |
| `--out` | Output WAV file (auto-named if omitted) |
| `--stdout` | Write WAV to stdout for piping |
| `--export-midi` | Export score as standard MIDI file |
| `--tempo` | Tempo in BPM for score CSV (default: 120) |
| `--program` | MIDI program/instrument number 0-127 (-1 = SF2 default) |
| `--bank` | MIDI bank number (default: 0) |

**Sample MIDI files** in `scores_midi/` (public domain, sourced from [mfiles.co.uk](https://www.mfiles.co.uk/classical-midi.htm) and [classicalguitarmidi.com](https://www.classicalguitarmidi.com/)):

| File | Composition | Composer |
|------|-------------|----------|
| `asturias-albeniz.mid` | Asturias (Leyenda) | Albéniz |
| `fur-elise.mid` | Für Elise | Beethoven |
| `beethoven-symphony9-4-ode-to-joy-piano-solo.mid` | Ode to Joy | Beethoven |
| `moonlight-movement1.mid` | Moonlight Sonata (1st mvt) | Beethoven |
| `Eine-Kleine-Nachtmusik1.mid` | Eine kleine Nachtmusik | Mozart |
| `pachelbel-canon-in-d.mid` | Canon in D | Pachelbel |
| `the-blue-danube-piano.mid` | The Blue Danube | Strauss II |
| `lagrima-tarrega.mid` | Lágrima | Tárrega |
| `recuerdos-de-la-alhambra-tarrega.mid` | Recuerdos de la Alhambra | Tárrega |
| `romance-anonimo.mid` | Romance Anónimo (Romanza) | Traditional |
| `greensleeves.mid` | Greensleeves | Traditional |
| `greensleeves_guitar.mid` | Greensleeves (guitar arr.) | Traditional |

### Transcribe audio to score

Detect pitches in a monophonic audio file and output a note/duration CSV:

```bash
# Basic transcription
musiclab transcribe --file melody.wav --out score.csv

# Pipe to stdout
musiclab transcribe --file melody.wav --stdout

# Transcribe a specific section
musiclab transcribe --file song.flac --start 1m30s --duration 10s --out score.csv

# Bass voice mode (uses Harmonic Product Spectrum for fundamental detection)
musiclab transcribe --file bass_line.wav --voice bass --out bass_score.csv

# Tune detection parameters
musiclab transcribe --file recording.mp3 --silence 0.03 --frame 4096 --hop 2048
```

**Supported input formats:** WAV, FLAC, MP3, OGG Vorbis, Opus

**Voice modes:**
- `melody` (default) — strongest peak in 80–2000 Hz range
- `bass` — fundamental frequency via HPS in 30–500 Hz range

**Round-trip test:** synthesize → transcribe → re-synthesize
```bash
musiclab doremi --score scores/greensleeves.csv --out greensleeves.wav
musiclab transcribe --file greensleeves.wav --out greensleeves_roundtrip.csv
musiclab doremi --score greensleeves_roundtrip.csv --wave=guitar --stdout | musictools play -
```

### Generate guitar tabs from MIDI or score

Convert MIDI files or score CSV to guitar tablature using Viterbi DP for optimal fingering:

```bash
# From MIDI file
musiclab tabs --midi song.mid

# From score CSV
musiclab tabs --score scores/greensleeves.csv

# Save to file
musiclab tabs --midi song.mid --out song.tab

# With capo
musiclab tabs --midi song.mid --capo 2

# Custom tuning (Drop D)
musiclab tabs --midi song.mid --tuning D2,A2,D3,G3,B3,E4

# Set tab width
musiclab tabs --midi song.mid --width 60

# Limit max fret
musiclab tabs --midi song.mid --max-fret 12
```

The algorithm enumerates all valid fingerings per time slot and uses Viterbi dynamic programming to minimize total cost (hand movement + stretch + high fret penalty), producing playable fingerings that stay in consistent positions.

### Parse ASCII guitar tabs to CSV and MIDI

Convert ASCII guitar tab files (.tab) to polyphonic score CSV and standard MIDI:

```bash
# Convert tab to CSV
musiclab tab2csv --in scores_tabs/greensleeves.tab --out greensleeves.csv

# Export MIDI
musiclab tab2csv --in scores_tabs/greensleeves.tab --export-midi greensleeves.mid

# Set tempo and beats per column
musiclab tab2csv --in scores_tabs/romance-anonimo.tab --tempo 80 --bpc 0.5

# With capo
musiclab tab2csv --in song.tab --capo 2 --out song.csv
```

**Supported tab format features:**
- Standard 6-string guitar (auto-detects string labels: E/A/D/G/B/e)
- Double pipes (`||`) for section starts
- Multi-digit fret numbers (10, 12, etc.)
- Duration calculated from column spacing

**Sample tabs** in `scores_tabs/` (public domain, sourced from [classtab.org](https://www.classtab.org/)):

| File | Composition | Composer |
|------|-------------|----------|
| `asturias-albeniz.tab` | Asturias (Leyenda) | Albéniz |
| `lagrima-tarrega.tab` | Lágrima | Tárrega |
| `recuerdos-de-la-alhambra-tarrega.tab` | Recuerdos de la Alhambra | Tárrega |
| `romance-anonimo.tab` | Romance de Amor | Traditional |
| `greensleeves.tab` | Greensleeves | Traditional |
| `house_of_the_rising_sun.tab` | House of the Rising Sun | Traditional |
