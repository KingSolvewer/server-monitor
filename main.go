package main

import (
	"fmt"
	"github.com/solvewer/server-monitor/monitor"
	"time"
)

func main() {
	// Step 1: 计算距离下一个整分钟的时间
	now := time.Now()
	next := now.Truncate(time.Minute).Add(time.Minute)
	time.Sleep(time.Until(next)) // 等待直到下一个00秒

	fmt.Println("当前执行时间", time.Now().Format(time.DateTime))
	run()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop() // 确保释放资源

	for range ticker.C {
		fmt.Println("当前执行时间", time.Now().Format(time.DateTime))
		run()
	}

}

func run() {

	serverMonitor := monitor.Calc()

	err := monitor.Save(serverMonitor)
	if err != nil {
		fmt.Println(err)
	}
}
