# Guitar Tab Generation: Viterbi DP vs Greedy

Comparison of two approaches to MIDI-to-guitar-tab conversion using **Lágrima** by Francisco Tárrega as reference. This piece is a standard classical guitar study typically played in positions VII-IX (frets 7-12).

**Source MIDI:** `scores_midi/lagrima-tarrega.mid` (385 notes, range E2-E5, all playable on standard guitar)

## Algorithm Overview

| | Viterbi DP (musiclab) | Greedy (baseline) |
|---|---|---|
| **Strategy** | Enumerate all valid fingerings per time slot; minimize total cost (hand movement + stretch + high fret penalty) across the entire piece | For each note, pick the lowest available fret on the lowest available string |
| **Scope** | Global optimization | Local per-note |
| **Hand position** | Maintained across consecutive notes | Not considered |

## Output Comparison (first section)

### Viterbi DP (musiclab `tabs`)

```
e|-------5-----7-----------------------------5-----7--------|
B|-9----10----12-----7-----------------9----10----12-----7--|
G|13----------------11----------------13----------------11--|
D|----9-----9-----9-----9--7--9-----9-----9-----9-----9-----|
A|-------9----11-----------------------7-----9----11--------|
E|12----------------11-----------7----------------------11--|
```

Frets cluster around **7-13** — matches the known position VII-IX where Lágrima is played.

### Greedy (lowest fret first)

```
e|4----5----7---2-----------4----5----7---2-----------12-0-11-0-|
B|9-0-10-0-12-0-7-0---0---0-9-0-10-0-12-0-7-0---0---0--2----0---|
G|----------1-------2-----------------1-------2-----------------|
D|2----4--------1-----------2----4--------1---------------------|
A|----------------------2-------------------------2-------------|
E|--------------------------------------------------------------|
```

Frets scattered across **0-12** — notes placed at lowest possible position regardless of hand context. A guitarist would need to jump between open position and 12th fret within the same beat.

## Fret Distribution

```
              Viterbi DP          Greedy
              ----------          ------
Frets  0-4:   117 (30%)          274 (72%)
Frets  5-9:   168 (44%)           64 (17%)
Frets 10-14:   90 (23%)           36 (10%)
Frets 15-19:   10  (3%)            5  (1%)
              ----------          ------
Mean fret:       7.0                3.4
```

The Viterbi output centers around fret 7-9, matching the piece's natural position. The greedy approach pushes 72% of notes into frets 0-4, which is physically inconsistent with the upper-register melody.

## Why This Matters

A guitar fretboard spans ~45 cm. A classical guitarist's hand covers about 4-5 frets in a fixed position. When simultaneous notes (bass + melody) are spread across 10+ frets, they are **physically unplayable**.

The Viterbi approach treats tab generation as a shortest-path problem: each time slot has multiple valid fingerings (states), and transitions between states have costs based on hand movement. Dynamic programming finds the globally optimal path through all states.

### Generating tabs

```bash
# From MIDI file
musiclab tabs --midi scores_midi/lagrima-tarrega.mid

# With capo (shifts all frets down)
musiclab tabs --midi scores_midi/lagrima-tarrega.mid --capo 2

# Custom tuning (Drop D)
musiclab tabs --midi scores_midi/lagrima-tarrega.mid --tuning D2,A2,D3,G3,B3,E4
```

## References

- [tuttut](https://github.com/natecdr/tuttut) — HMM + Viterbi for guitar fingering (inspired our approach)
- [MidiToTabs](https://github.com/romanblack1/MidiToTabs) — MIDI to guitar tab (Python reference)
- Lágrima MIDI from [classicalguitarmidi.com](https://www.classicalguitarmidi.com/tarrega.html) (public domain, transcription by Dmitri Bachovich)
