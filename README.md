
musiclab

Learning audio

### Build
```
go build
```

### Generate music scale
```
./musiclab doremi
```

File doremi.wav created

### Play audio file
```
./musiclab play --file=doremi.wav
```

### Spectrogram

Create audio file spectrogram
```
./musiclab spectrogram --file=doremi.wav
```
![](examples/doremi.spectr.png)

### Chromagram
Create audio file chromagram
```
 ./musiclab chromagram --file=doremi.wav
 ```
![](examples/doremi.chroma.png)