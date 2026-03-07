# musiclab Design

Audio analysis and fingerprinting toolkit written in Go.

## Architecture Overview

```mermaid
graph TD
    subgraph CLI["CLI Commands"]
        FP[fingerprint]
        FFT[fft]
        CHR[chromagram]
        SPE[spectrogram]
        DB[db]
        SM[samplematch]
        DRM[doremi]
        TRS[transcribe]
        MIDI[midi]
        TABS[tabs]
        T2C[tab2csv]
    end

    subgraph Core["Core Packages"]
        DSP[dsp/]
        AS[audiosource/]
        AID[acoustid/]
        SCN[scan/]
        DEC[internal/decoders/]
    end

    subgraph External["External Dependencies"]
        AK[audiokit<br/>decoders: MP3, FLAC, WAV, OGG]
        MT[musictags<br/>metadata: ID3, Vorbis Comments]
        GN[gonum<br/>FFT, plotting]
        BG[badger<br/>KV store]
        MS[go-meltysynth<br/>SoundFont MIDI synth]
    end

    FP --> DSP
    FP --> AS
    FP --> AID
    FFT --> AS
    FFT --> DSP
    CHR --> AS
    CHR --> DSP
    SPE --> AS
    SPE --> DSP
    DB --> AS
    DB --> DSP
    DB --> SCN
    DB --> BG
    SM --> DSP
    SM --> BG
    DRM --> DSP
    TRS --> AS
    TRS --> DSP
    MIDI --> DSP
    MIDI --> MS
    TABS --> DSP
    T2C --> DSP

    AS --> DEC
    SCN --> MT
    DEC --> AK
    DSP --> GN
    CHR --> GN
    SPE --> GN
```

## Data Flow Pipelines

### Chromaprint Fingerprinting

```mermaid
flowchart LR
    A[Audio File] --> B[Decode<br/>MP3/FLAC/WAV/OGG]
    B --> C[Mono Mix<br/>int32 accumulation]
    C --> D[Resample<br/>to 11025 Hz]
    D --> E[Hamming Window<br/>+ FFT]
    E --> F[Energy Spectrum]
    F --> G[Chroma Features<br/>12 bins]
    G --> H[ChromaFilter<br/>coeffs: 0.25 0.75 1.0 0.75 0.25]
    H --> I[Normalize<br/>Euclidean norm]
    I --> J[Integral Image]
    J --> K[16 Classifiers<br/>algorithm 2]
    K --> L[Subfingerprints<br/>32-bit each]
    L --> M[Base64 Encode<br/>with compression]
```

**Key parameters:**
- Sample rate: 11025 Hz
- FFT frame size: 4096 samples
- Hop size: 1365 samples (frame_size / 3)
- Max duration: 120 seconds
- Resampler: polyphase Kaiser-windowed sinc (beta=9, cutoff=0.8, filter_length=16)

### Spectral Hash Database (db + samplematch)

```mermaid
flowchart TD
    subgraph Build["db --scan"]
        A1[Music Folder] --> A2[Walk Files<br/>concurrent]
        A2 --> A3[Decode + STFT<br/>per file]
        A3 --> A4[OctaveBinPeaks<br/>per frame]
        A4 --> A5[PeaksHash]
        A5 --> A6[Badger DB<br/>hash -> song,timestamp]
    end

    subgraph Match["samplematch"]
        B1[Sample File] --> B2[STFT + Hash]
        B2 --> B3[Lookup hashes<br/>in DB]
        B3 --> B4[Timestamp diff<br/>histogram]
        B4 --> B5[Top candidates<br/>by match count]
    end

    A6 -.-> B3
```

**Spectral hash scheme:**
- STFT: 2048-sample frames, 441-sample hop
- 8 octave bins: below C2, C2-C3, C3-C4, C4-C5, C5-C6, C6-C7, C7-C8, above C8
- Peak frequency per bin (largest amplitude wins)
- Hash: concatenated peak frequencies with LSB cleared, base-1000 encoding

### MIDI / SoundFont Rendering

```mermaid
flowchart LR
    A[Score CSV<br/>note,dur] --> B[scoreToMidiNotes<br/>sequential events]
    C[MIDI File<br/>.mid] --> D[MidiFileSequencer]

    B --> E[NoteOn/NoteOff<br/>event scheduling]
    D --> F[Sequencer.Render]

    E --> G[meltysynth<br/>Synthesizer]
    F --> G

    G --> H[float32 stereo<br/>L + R buffers]
    H --> I[int16 WAV<br/>output]

    B --> J[WriteMIDIFile<br/>export .mid]
```

**Pipeline:**
1. Score CSV is parsed into sequential `MidiNote` events (note, velocity, startMs, durMs)
2. Events are sorted by sample position and scheduled as NoteOn/NoteOff calls
3. go-meltysynth synthesizer renders audio between events using SoundFont instrument samples
4. Float32 stereo output is converted to 16-bit interleaved WAV

**MIDI export:** Score CSV can also be exported as standard MIDI files (Format 0, 480 ticks/quarter) via `dsp.WriteMIDIFile`, independent of SoundFont rendering.

### Audio Decoding

```mermaid
flowchart LR
    F[Audio File] --> R[Registry<br/>by extension]
    R --> MP3[MP3<br/>mpg123]
    R --> FLAC[FLAC<br/>libflac]
    R --> WAV[WAV<br/>go-wav]
    R --> OGG[OGG<br/>auto-detect]
    OGG --> VB[Vorbis]
    OGG --> OP[Opus]

    MP3 --> M[Mono float64<br/>normalized]
    FLAC --> M
    WAV --> M
    VB --> M
    OP --> M
```

Supports 16-bit, 24-bit, and 32-bit audio. Multi-channel audio is mixed to mono using int32/int64 accumulation (avoids int16 overflow).

## Package Responsibilities

| Package | Purpose |
|---|---|
| `dsp/` | Signal processing: FFT, STFT, chroma, fingerprinting, resampling, synthesis (Karplus-Strong), MIDI file writing, note utilities |
| `audiosource/` | Decode audio files to mono float64 samples |
| `acoustid/` | AcoustID REST API client for online fingerprint lookup |
| `scan/` | Concurrent music directory walker with metadata extraction |
| `internal/decoders/` | Decoder registry wrapping audiokit codec implementations |

## Project Ecosystem

```mermaid
graph TD
    AK[audiokit<br/>shared decoders, types, player] --> ML[musiclab<br/>DSP, fingerprinting, analysis]
    AK --> MT_TOOL[musictools<br/>player, transform, samplecut]
    AK --> MT_TAG[musictags<br/>tags, tagger, lyrics, coverart]
    MT_TAG --> ML

    style ML fill:#4a9,stroke:#333,stroke-width:2px
```

**musiclab** focuses on audio analysis and identification. It depends on:
- **audiokit** for codec abstraction (decoders, types)
- **musictags** for metadata extraction during library scanning

Audio playback, file conversion, and segment cutting live in **musictools**.
Metadata tagging, lyrics, and cover art live in **musictags**.

## References and Inspiration

Projects and algorithms that influenced the implementation:

| Feature | Reference | What we used |
|---|---|---|
| SoundFont MIDI rendering | [go-meltysynth](https://github.com/sinshu/go-meltysynth) | SF2 loading, MIDI sequencer, real-time synthesizer |
| Guitar sound synthesis | [Karplus-Strong algorithm](https://en.wikipedia.org/wiki/Karplus%E2%80%93Strong_string_synthesis) | Plucked string waveform in doremi `--wave=guitar` |
| Guitar tab conversion | [tuttut](https://github.com/natecdr/tuttut) — HMM + Viterbi for optimal fingering | Inspired our Viterbi DP approach for `tabs` command string/fret assignment. See [comparison](TABS_COMPARISON.md) |
| Guitar tab conversion | [MidiToTabs](https://github.com/romanblack1/MidiToTabs) — MIDI to guitar tab (Python) | Reference for MIDI-to-fretboard mapping, tuning offsets, capo support |
| Audio-to-tab pipeline | [gtrsnipe](https://github.com/scottvr/gtrsnipe) — audio separation + pitch detection + tab | Reference for full audio-to-tab pipeline design (Demucs + Basic-Pitch) |
| Chromaprint fingerprinting | [Chromaprint](https://acoustid.org/chromaprint) (C reference implementation) | Ported algorithm to Go: chroma features, classifier, base64 encoding. Drop-in `fpcalc` replacement (text, JSON, raw output formats) |
| Audio fingerprinting | [AcoustID](https://acoustid.org/) | Online lookup API for fingerprint-to-metadata matching |
| Tab-to-MIDI reference | [Sarath18/tab2midi](https://github.com/Sarath18/GuitarTabsToMidi) (Python) | Validated our tab2csv output against reference implementation (100% note match) |
