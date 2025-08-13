package util

import (
	"fmt"
	"math"
)

func ToGbInt64(x uint64) uint64 {
	return x / 1024 / 1024 / 1024
}

func ToMbFloat(x uint64) float64 {
	return float64(x) / 1024 / 1024
}

func ToDouble(x float64) float64 {
	fmt.Println(x, x*100, math.Round(x*100))
	return math.Round(x*10000) / 10000
}
