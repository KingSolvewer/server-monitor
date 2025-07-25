package monitor

import (
	"context"
	"errors"
	"fmt"
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
	Node         int       `gorm:"column:node;primaryKey"`
	CreatedAt    time.Time `gorm:"column:created_at;primaryKey"`
}

var (
	config Config
	wg     sync.WaitGroup
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
}

func calc(t time.Time) *ServerMonitor {

	monitorTable := new(ServerMonitor)

	wg.Add(6)

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

	wg.Wait()

	monitorTable.CreatedAt = t.Truncate(time.Minute)
	fmt.Println("入表时间：", monitorTable.CreatedAt)
	monitorTable.Node = config.WebNode
	return monitorTable
}

func save(monitor *ServerMonitor) (err error) {
	dsn := config.DbUsername + ":" + config.DbPassword + "@tcp(" + config.DbHost + ":" + strconv.Itoa(config.DbPort) + ")/" + config.DbName + "?charset=utf8mb4&parseTime=True&loc=Local"

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{NamingStrategy: schema.NamingStrategy{IdentifierMaxLength: 64, SingularTable: true}})
	if err != nil {
		return errors.New("Gorm error: " + err.Error())
	}
	sqlDB, err := db.DB()
	if err != nil {
		return errors.New("DB error: " + err.Error())
	}
	err = sqlDB.Ping()
	if err != nil {
		return errors.New("Connect DB error: " + err.Error())
	}
	ctx := context.Background()
	err = gorm.G[ServerMonitor](db).Create(ctx, monitor)
	if err != nil {
		return errors.New("Gorm Insert Sql error: " + err.Error())
	}

	return
}

func Start() {
	// Step 1: 计算距离下一个整分钟的时间
	now := time.Now()
	next := now.Truncate(time.Minute).Add(time.Minute)
	time.Sleep(time.Until(next)) // 等待直到下一个00秒

	fmt.Println("当前开始执行时间", time.Now().Format(time.DateTime))
	fmt.Println("统计时间：", time.Now().Truncate(time.Minute).Format(time.DateTime))
	run(now)
	fmt.Println("当前结束执行时间", time.Now().Format(time.DateTime))

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop() // 确保释放资源

	for range ticker.C {
		now = time.Now()
		fmt.Println("当前开始执行时间", time.Now().Format(time.DateTime))
		fmt.Println("统计时间：", now.Truncate(time.Minute).Format(time.DateTime))
		run(now)
		fmt.Println("当前结束执行时间", time.Now().Format(time.DateTime))
	}
}

func run(t time.Time) {
	calc(t)

	//serverMonitor := calc(t)
	//
	//err := save(serverMonitor)
	//if err != nil {
	//	fmt.Println(err)
	//}
}
