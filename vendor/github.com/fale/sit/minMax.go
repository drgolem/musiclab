package sit

import "math"

func Min(min float64, max float64) float64 {
	_, majorTicker := tickers(min, max)
	return math.Floor(float64(min)/float64(majorTicker)) * float64(majorTicker)
}

func Max(min float64, max float64) float64 {
	_, majorTicker := tickers(min, max)
	return math.Ceil(float64(max)/float64(majorTicker)) * float64(majorTicker)
}
