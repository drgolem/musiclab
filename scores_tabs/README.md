# Sample Guitar Tabs

Classical guitar tablature sourced from [classtab.org](https://www.classtab.org/) — a curated collection of user-contributed ASCII tabs for public domain compositions.

All pieces included here are in the **public domain**:

| File | Composition | Composer | Died |
|------|-------------|----------|------|
| `asturias-albeniz.tab` | Asturias (Leyenda) | Isaac Albéniz | 1909 |
| `lagrima-tarrega.tab` | Lágrima | Francisco Tárrega | 1909 |
| `recuerdos-de-la-alhambra-tarrega.tab` | Recuerdos de la Alhambra | Francisco Tárrega | 1909 |
| `romance-anonimo.tab` | Romance de Amor (Spanish Romance) | Traditional/anonymous | — |
| `greensleeves.tab` | Greensleeves | Traditional English | — |
| `house_of_the_rising_sun.tab` | House of the Rising Sun | Traditional American folk | — |

Each `.tab` file can be converted to CSV and MIDI:

```bash
musiclab tab2csv --in scores_tabs/lagrima-tarrega.tab --export-midi lagrima.mid
```

Matching MIDI files for the classical pieces are available in `scores_midi/`.
