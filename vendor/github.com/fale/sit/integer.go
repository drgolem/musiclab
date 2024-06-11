package sit

import (
	"fmt"
	"math"
	"time"

	"gonum.org/v1/plot"
)

type Ticker struct {
	Ticker plot.Ticker
	Format string
	Time   func(t float64) time.Time
}

func tickers(min float64, max float64) (int, int) {
	minorTicker := 1
	majorTicker := 5
	if max-min > 25 {
		minorTicker = 2
		majorTicker = 10
	}
	if max-min > 50 {
		minorTicker = 5
		majorTicker = 10
	}
	if max-min > 100 {
		minorTicker, majorTicker = tickers(min/10, max/10)
		minorTicker = minorTicker * 10
		majorTicker = majorTicker * 10
	}
	return minorTicker, majorTicker
}

func (t Ticker) Ticks(min float64, max float64) []plot.Tick {
	minorTicker, majorTicker := tickers(min, max)
	ticks := []plot.Tick{}
	i := math.Floor(float64(min)/float64(minorTicker)) * float64(minorTicker)
	for {
		t := plot.Tick{Value: i}
		if int(i)%majorTicker == 0 {
			t.Label = fmt.Sprintf("%.0f\n", i)
		}
		ticks = append(ticks, t)
		i = i + float64(minorTicker)
		if i > max {
			break
		}
	}
	return ticks
}
