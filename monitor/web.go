package monitor

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-ping/ping"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/solvewer/server-monitor/configuration"
	"github.com/solvewer/server-monitor/util"
	"gorm.io/gorm"
	"sync"
	"time"
)

type ServerMonitor struct {
	Pressure     float64   `gorm:"column:pressure"`
	CpuUsage     float64   `gorm:"column:cpu_usage"`
	LoadAvg      float64   `gorm:"column:load_avg"`
	MemUsage     float64   `gorm:"column:mem_usage"`
	MemTotal     uint64    `gorm:"column:mem_total"`
	MemUsed      uint64    `gorm:"column:mem_used"`
	SwapUsage    float64   `gorm:"column:swap_usage"`
	DiskUsage    float64   `gorm:"column:disk_usage"`
	DiskTotal    uint64    `gorm:"column:disk_total"`
	DiskUsed     uint64    `gorm:"column:disk_used"`
	SentSpeed    float64   `gorm:"column:sent_speed"`
	ReceiveSpeed float64   `gorm:"column:receive_speed"`
	AvgRtt       float64   `gorm:"column:avg_rtt"`
	PacketLoss   float64   `gorm:"column:packet_loss"`
	Node         int       `gorm:"column:node;primaryKey"`
	CreatedAt    time.Time `gorm:"column:created_at;primaryKey"`
}

var (
	db       *gorm.DB
	config   *configuration.Config
	wg       sync.WaitGroup
	lastRecv uint64
	lastSent uint64
)

func init() {
	db = configuration.GetDb()
	config = configuration.GetConfig()
}

func calc(t time.Time) *ServerMonitor {

	monitor := new(ServerMonitor)

	wg.Add(7)
	monitor.route()
	wg.Wait()

	monitor.CreatedAt = t.Truncate(time.Minute)
	fmt.Println("入表时间：", monitor.CreatedAt)
	monitor.Node = config.WebNode
	return monitor
}

func Start() {
	// Step 1: 计算距离下一个整分钟的时间
	now := time.Now()
	fmt.Println("程序开始执行时间：", now)
	next := now.Truncate(time.Minute).Add(time.Minute + time.Second*30)
	time.Sleep(time.Until(next)) // 等待直到下一个30秒时刻

	fmt.Println("开始统计时间：", time.Now())
	run(next)
	fmt.Println("统计结束时间：", time.Now())

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop() // 确保释放资源

	for now = range ticker.C {
		fmt.Println("统计开始时间：", now)
		run(now)
		fmt.Println("统计结束时间：", time.Now())
	}
}

func run(t time.Time) {
	calc(t)
	//monitor := calc(t)

	//err := monitor.save()
	//if err != nil {
	//	fmt.Println(err)
	//}
}

func (monitor *ServerMonitor) save() (err error) {
	ctx := context.Background()
	err = gorm.G[ServerMonitor](db).Table("server_monitor").Create(ctx, monitor)
	if err != nil {
		return errors.New("Gorm Insert Sql error: " + err.Error())
	}

	return
}

func (monitor *ServerMonitor) route() {
	//1. 压力（系统负载 / CPU 核数）= 1分钟平均负载 / CPU核数
	go func() {
		defer wg.Done()
		monitor.pressure()
	}()

	// 2. CPU 使用率
	go func() {
		defer wg.Done()
		monitor.cpu()
	}()

	// 3. 内存使用率
	go func() {
		defer wg.Done()
		monitor.mem()
	}()

	// 4. 交换分区使用率
	go func() {
		defer wg.Done()
		monitor.swap()
	}()

	// 5. 根分区使用率
	go func() {
		defer wg.Done()
		monitor.disk()
	}()

	// 6 网络使用量（当前瞬时的发送接收字节数）
	go func() {
		defer wg.Done()
		monitor.byte()
	}()

	// 7 网络流量
	go func() {
		defer wg.Done()
		monitor.net()
	}()
}

func (monitor *ServerMonitor) pressure() {
	loadAvg, _ := load.Avg()
	cpuCount, _ := cpu.Counts(true)
	monitor.Pressure = util.ToDouble(loadAvg.Load1 / float64(cpuCount))
	fmt.Println("系统压力", monitor.Pressure)

	// 3. 系统负载（Load1）
	monitor.LoadAvg = util.ToDouble(loadAvg.Load1)
	fmt.Println("系统负载", monitor.LoadAvg)
}

func (monitor *ServerMonitor) cpu() {
	cpuPercent, _ := cpu.Percent(time.Second, false) // 采样一秒
	monitor.CpuUsage = util.ToDouble(cpuPercent[0])
	fmt.Println("cpu使用率", monitor.CpuUsage)
}

func (monitor *ServerMonitor) mem() {
	vmem, _ := mem.VirtualMemory()
	monitor.MemUsage = util.ToDouble(vmem.UsedPercent)
	monitor.MemTotal = util.ToGbInt64(vmem.Total)
	monitor.MemUsed = util.ToGbInt64(vmem.Used)
	fmt.Println("内存使用情况：", vmem, monitor.MemUsage)
}

func (monitor *ServerMonitor) swap() {
	swap, _ := mem.SwapMemory()
	monitor.SwapUsage = util.ToDouble(swap.UsedPercent)
	fmt.Println("交换区使用情况：", swap, monitor.SwapUsage)
}

func (monitor *ServerMonitor) disk() {
	diskUsage, _ := disk.Usage("/")
	monitor.DiskUsage = util.ToDouble(diskUsage.UsedPercent)
	monitor.DiskTotal = util.ToGbInt64(diskUsage.Total)
	monitor.DiskUsed = util.ToGbInt64(diskUsage.Used)
	fmt.Println("磁盘使用情况：", diskUsage, monitor.DiskUsage)
}

func (monitor *ServerMonitor) byte() {

	ioStats, _ := net.IOCounters(false)
	currentRecv := ioStats[0].BytesRecv
	currentSent := ioStats[0].BytesSent
	if lastRecv == 0 {
		monitor.ReceiveSpeed = 0
	} else {
		monitor.ReceiveSpeed = util.ToDouble(util.ToMbFloat(currentRecv - lastRecv))
	}
	if lastSent == 0 {
		monitor.SentSpeed = 0
	} else {
		monitor.SentSpeed = util.ToDouble(util.ToMbFloat(currentSent - lastSent))
	}
	lastRecv = currentRecv
	lastSent = currentSent
}

func (monitor *ServerMonitor) net() {
	pinger, err := ping.NewPinger("8.8.8.8") // Google DNS
	if err != nil {
		fmt.Println("ping时报错：", err)
	}
	pinger.Count = 5                 // 一次发送 5 个 ping
	pinger.Interval = time.Second    // 每秒一个
	pinger.Timeout = 6 * time.Second // 最长运行时间
	pinger.SetPrivileged(true)       // 使用原始 socket（需要 root 权限）

	// 可选：注册回调
	pinger.OnRecv = func(pkt *ping.Packet) {
		fmt.Printf("Reply from %s: time=%v\n", pkt.IPAddr, pkt.Rtt)
	}
	pinger.OnFinish = func(stats *ping.Statistics) {
		//spew.Dump(stats)
		//os.Exit(1)
		fmt.Printf("\n--- %s ping statistics ---\n", stats.Addr)
		fmt.Printf("Packets: Sent = %d, Received = %d, Lost = %d (%.1f%% loss)\n",
			stats.PacketsSent, stats.PacketsRecv, stats.PacketsSent-stats.PacketsRecv, stats.PacketLoss)
		fmt.Printf("RTT: min = %v, avg = %v, max = %v, stddev = %v\n",
			stats.MinRtt, stats.AvgRtt, stats.MaxRtt, stats.StdDevRtt)

		monitor.AvgRtt = util.ToDouble(float64(int(stats.AvgRtt)) / 1000000)
		monitor.PacketLoss = util.ToDouble(stats.PacketLoss)
	}

	fmt.Println("PING 8.8.8.8:")
	err = pinger.Run() // 阻塞执行
	if err != nil {
		fmt.Println("ping时报错：", err)
	}
}
