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
	"github.com/solvewer/server-monitor/util"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"strconv"
	"sync"
	"time"
)

type Config struct {
	DbHost     string
	DbUsername string
	DbPassword string
	DbName     string
	DbPort     int
	WebNode    int
	LastRecv   uint64
	LastSent   uint64
}

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
	db     *gorm.DB
	config Config
	wg     sync.WaitGroup
	pinger *ping.Pinger
)

func init() {
	SetConfig()
}

func SetConfig() {

	viper.AddConfigPath(".")
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	config.DbHost = viper.GetString("DB_HOST")
	if config.DbHost == "" {
		config.DbHost = "localhost"
	}
	config.DbPassword = viper.GetString("DB_PASSWORD")
	config.DbName = viper.GetString("DB_NAME")
	config.DbUsername = viper.GetString("DB_USERNAME")
	if config.DbUsername == "" {
		config.DbUsername = "root"
	}
	config.DbPort = viper.GetInt("DB_PORT")
	if config.DbPort == 0 {
		config.DbPort = 3306
	}
	config.WebNode = viper.GetInt("WEB_NODE")

	dsn := config.DbUsername + ":" + config.DbPassword + "@tcp(" + config.DbHost + ":" + strconv.Itoa(config.DbPort) + ")/" + config.DbName + "?charset=utf8mb4&parseTime=True&loc=Local"

	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{NamingStrategy: schema.NamingStrategy{IdentifierMaxLength: 64, SingularTable: true}})
	if err != nil {
		panic("Gorm error: " + err.Error())
	}

	pinger, err = ping.NewPinger("8.8.8.8") // Google DNS
	if err != nil {
		panic("Ping error: " + err.Error())
	}
}

func calc(t time.Time) *ServerMonitor {

	monitorTable := new(ServerMonitor)

	wg.Add(7)

	//1. 压力（系统负载 / CPU 核数）= 1分钟平均负载 / CPU核数
	go func() {
		defer wg.Done()
		loadAvg, _ := load.Avg()
		cpuCount, _ := cpu.Counts(true)
		monitorTable.Pressure = util.ToDouble(loadAvg.Load1 / float64(cpuCount))
		fmt.Println("系统压力", monitorTable.Pressure)

		// 3. 系统负载（Load1）
		monitorTable.LoadAvg = util.ToDouble(loadAvg.Load1)
		fmt.Println("系统负载", monitorTable.LoadAvg)
	}()

	// 2. CPU 使用率
	go func() {
		defer wg.Done()
		cpuPercent, _ := cpu.Percent(time.Second, false) // 采样一秒
		monitorTable.CpuUsage = util.ToDouble(cpuPercent[0])
		fmt.Println("cpu使用率", monitorTable.CpuUsage)
	}()

	// 4. 内存使用率
	go func() {
		defer wg.Done()
		vmem, _ := mem.VirtualMemory()
		monitorTable.MemUsage = util.ToDouble(vmem.UsedPercent)
		monitorTable.MemTotal = util.ToGbInt64(vmem.Total)
		monitorTable.MemUsed = util.ToGbInt64(vmem.Used)
		fmt.Println("内存使用情况：", vmem, monitorTable.MemUsage)
	}()

	// 5. 交换分区使用率
	go func() {
		defer wg.Done()
		swap, _ := mem.SwapMemory()
		monitorTable.SwapUsage = util.ToDouble(swap.UsedPercent)
		fmt.Println("交换区使用情况：", swap, monitorTable.SwapUsage)
	}()

	// 6. 根分区使用率
	go func() {
		defer wg.Done()
		diskUsage, _ := disk.Usage("/")
		monitorTable.DiskUsage = util.ToDouble(diskUsage.UsedPercent)
		monitorTable.DiskTotal = util.ToGbInt64(diskUsage.Total)
		monitorTable.DiskUsed = util.ToGbInt64(diskUsage.Used)
		fmt.Println("磁盘使用情况：", diskUsage, monitorTable.DiskUsage)
	}()

	// 网络使用量（当前瞬时的发送接收字节数）
	go func() {
		defer wg.Done()
		ioStats, _ := net.IOCounters(false)
		currentRecv := ioStats[0].BytesRecv
		currentSent := ioStats[0].BytesSent
		if config.LastRecv == 0 {
			monitorTable.ReceiveSpeed = 0
		} else {
			monitorTable.ReceiveSpeed = util.ToDouble(util.ToMbFloat(currentRecv - config.LastRecv))
		}
		if config.LastSent == 0 {
			monitorTable.SentSpeed = 0
		} else {
			monitorTable.SentSpeed = util.ToDouble(util.ToMbFloat(currentSent - config.LastSent))
		}
		config.LastRecv = currentRecv
		config.LastSent = currentSent
	}()

	go func() {
		defer wg.Done()
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

			monitorTable.AvgRtt = util.ToDouble(float64(int(stats.AvgRtt)) / 1000000)
			monitorTable.PacketLoss = util.ToDouble(stats.PacketLoss)
		}

		fmt.Println("PING 8.8.8.8:")
		err := pinger.Run() // 阻塞执行
		if err != nil {
			fmt.Println(err)
		}
	}()

	wg.Wait()

	monitorTable.CreatedAt = t.Truncate(time.Minute)
	fmt.Println("入表时间：", monitorTable.CreatedAt)
	monitorTable.Node = config.WebNode
	return monitorTable
}

func save(monitor *ServerMonitor) (err error) {
	ctx := context.Background()
	err = gorm.G[ServerMonitor](db).Table("server_monitor").Create(ctx, monitor)
	if err != nil {
		return errors.New("Gorm Insert Sql error: " + err.Error())
	}

	return
}

func Start() {
	// Step 1: 计算距离下一个整分钟的时间
	now := time.Now()
	fmt.Println("程序开始执行时间：", time.Now().Format(time.DateTime))
	next := now.Truncate(time.Minute).Add(time.Minute)
	time.Sleep(time.Until(next)) // 等待直到下一个00秒

	fmt.Println("开始统计时间：", next.Format(time.DateTime))
	run(now)
	fmt.Println("统计结束时间：", time.Now().Format(time.DateTime))

	ticker := time.NewTicker(time.Minute + time.Second)
	defer ticker.Stop() // 确保释放资源

	for now = range ticker.C {
		fmt.Println("统计开始时间：", now.Format(time.DateTime))
		run(now)
		fmt.Println("统计结束时间：", time.Now().Format(time.DateTime))
	}
}

func run(t time.Time) {
	serverMonitor := calc(t)

	err := save(serverMonitor)
	if err != nil {
		fmt.Println(err)
	}
}
