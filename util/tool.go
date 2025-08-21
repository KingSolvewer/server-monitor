package util

import (
	"context"
	"fmt"
	"math"
	"time"
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

// AlignTicker 每隔 interval 触发一次，但严格对齐到整点/整分/整秒
// 支持 context 取消，可选择立即触发一次
func AlignTicker(ctx context.Context, interval time.Duration, immediate bool) <-chan time.Time {
	ch := make(chan time.Time)

	go func() {
		defer close(ch)

		// 如果需要立即触发一次
		if immediate {
			select {
			case <-ctx.Done():
				return
			case ch <- time.Now():
			}
		}

		// 第一次先对齐到下一个 interval 的整数倍
		now := time.Now()
		next := now.Truncate(interval).Add(interval)

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
			ch <- next
		}

		timer := time.NewTimer(time.Minute)
		defer timer.Stop()

		// 后续循环
		for {
			next = time.Now().Truncate(interval).Add(interval)
			sleep := time.Until(next)
			if sleep < 0 {
				sleep = 0
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(sleep):
				ch <- next
			}
		}
	}()

	return ch
}

func LogPath(fileName string) string {
	return "/var/log/" + fileName + ".log"
	//return fileName + ".log"
}
