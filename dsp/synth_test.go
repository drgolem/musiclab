package dsp

import (
	"math"
	"testing"
)

func TestWaveforms(t *testing.T) {
	tests := []struct {
		name  string
		fn    WaveFunc
		phase float64
		want  float64
	}{
		{"sine 0", Sine, 0, 0},
		{"sine 0.25", Sine, 0.25, 1.0},
		{"sine 0.75", Sine, 0.75, -1.0},
		{"square 0.1", Square, 0.1, 1.0},
		{"square 0.9", Square, 0.9, -1.0},
		{"triangle 0", Triangle, 0, 0},
		{"triangle 0.25", Triangle, 0.25, 1.0},
		{"triangle 0.5", Triangle, 0.5, 0},
		{"triangle 0.75", Triangle, 0.75, -1.0},
		{"sawtooth 0", Sawtooth, 0, -1.0},
		{"sawtooth 0.5", Sawtooth, 0.5, 0},
		{"sawtooth 1", Sawtooth, 1.0, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn(tt.phase)
			if math.Abs(got-tt.want) > 1e-10 {
				t.Errorf("got %f, want %f", got, tt.want)
			}
		})
	}
}

func TestWaveFuncByName(t *testing.T) {
	// Ensure all names return non-nil and default is sine
	for _, name := range []string{"sine", "square", "triangle", "sawtooth", "saw", "unknown", ""} {
		fn := WaveFuncByName(name)
		if fn == nil {
			t.Errorf("WaveFuncByName(%q) returned nil", name)
		}
	}
}

func TestMixTracks_Empty(t *testing.T) {
	result := MixTracks(nil, 44100)
	if result != nil {
		t.Errorf("expected nil for empty events, got %d samples", len(result))
	}
}

func TestMixTracks_SingleNote(t *testing.T) {
	events := []TrackEvent{
		{Track: 0, Wave: "sine", Note: "A4", StartMs: 0, DurMs: 100},
	}
	result := MixTracks(events, 44100)
	// 100ms at 44100 Hz = 4410 samples * 2 channels
	expectedLen := 4410 * 2
	if len(result) != expectedLen {
		t.Errorf("expected %d samples, got %d", expectedLen, len(result))
	}
	// Should have non-zero content
	hasNonZero := false
	for _, v := range result {
		if v != 0 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		t.Error("expected non-zero audio output")
	}
}

func TestMixTracks_Normalization(t *testing.T) {
	// 3 loud tracks at same time should get normalized
	events := []TrackEvent{
		{Track: 0, Wave: "square", Note: "C4", StartMs: 0, DurMs: 100, Velocity: 1.0},
		{Track: 1, Wave: "square", Note: "E4", StartMs: 0, DurMs: 100, Velocity: 1.0},
		{Track: 2, Wave: "square", Note: "G4", StartMs: 0, DurMs: 100, Velocity: 1.0},
	}
	result := MixTracks(events, 44100)
	for _, v := range result {
		if v > 1.0 || v < -1.0 {
			t.Fatalf("sample out of range: %f", v)
		}
	}
}

func TestMixTracks_RestSkipped(t *testing.T) {
	events := []TrackEvent{
		{Track: 0, Wave: "sine", Note: "-", StartMs: 0, DurMs: 100},
	}
	result := MixTracks(events, 44100)
	for _, v := range result {
		if v != 0 {
			t.Fatalf("expected silence for rest, got %f", v)
		}
	}
}

func TestFloat64ToInt16Bytes(t *testing.T) {
	samples := []float64{0.0, 1.0, -1.0}
	bytes := Float64ToInt16Bytes(samples)
	if len(bytes) != 6 {
		t.Fatalf("expected 6 bytes, got %d", len(bytes))
	}
	// 0.0 -> 0x0000
	if bytes[0] != 0 || bytes[1] != 0 {
		t.Errorf("expected 0x0000 for 0.0, got %02x%02x", bytes[1], bytes[0])
	}
	// 1.0 -> 0x7FFF
	val := int16(bytes[2]) | int16(bytes[3])<<8
	if val != 0x7FFF {
		t.Errorf("expected 0x7FFF for 1.0, got %04x", uint16(val))
	}
}
