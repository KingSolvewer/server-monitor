package monitor

import (
	"context"
	"fmt"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/solvewer/server-monitor/util"
	"gorm.io/gorm"
	"time"
)

type MysqlMonitor struct {
	ThreadsConnected int       `gorm:"column:threads_connected"`
	ThreadsRunning   int       `gorm:"column:threads_running"`
	Qps              int       `gorm:"column:qps"`
	SlowQueries      int       `gorm:"column:slow_queries"`
	BufferHitRate    float64   `gorm:"column:buffer_hit_rate"`
	WriteSpeed       float64   `gorm:"column:write_speed"`
	ReadSpeed        float64   `gorm:"column:read_speed"`
	CreatedAt        time.Time `gorm:"column:created_at"`
}

var (
	lastQueries     int
	lastSlowQueries int
	prevIO          map[string]disk.IOCountersStat
)

func StartMysql() {
	if lastQueries == 0 {
		lastQueries, _ = GetStatus(db, "Queries")
	}
	if lastSlowQueries == 0 {
		lastSlowQueries, _ = GetStatus(db, "Slow_queries")
	}

	prevIO, _ = disk.IOCounters()

	// Step 1: 计算距离下一个整分钟的时间
	now := time.Now()
	fmt.Println("程序开始执行时间：", now)

	next := now.Truncate(time.Minute).Add(time.Minute + time.Second*30)
	time.Sleep(time.Until(next)) // 等待直到下一个30秒时刻

	fmt.Println("开始统计时间：", next)
	mysqlRun(next)
	fmt.Println("统计结束时间：", time.Now())

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for now = range ticker.C {
		fmt.Println("开始统计时间：", now)
		mysqlRun(now)
		fmt.Println("统计结束时间：", time.Now())
	}

}

type StatusResult struct {
	VariableName string `gorm:"column:Variable_name"`
	Value        int    `gorm:"column:Value"`
}

func GetStatus(db *gorm.DB, item string) (int, error) {
	var result StatusResult
	// 执行查询并映射结果
	err := db.Raw("SHOW GLOBAL STATUS LIKE '" + item + "'").Scan(&result).Error
	if err != nil {
		return 0, err
	}
	return result.Value, nil
}

func mysqlRun(t time.Time) {
	mysqlMonitor := mysqlCalc(t)

	ctx := context.Background()
	err := gorm.G[MysqlMonitor](db).Table("server_monitor_mysql").Create(ctx, mysqlMonitor)
	if err != nil {
		fmt.Println("Gorm Insert Sql error: " + err.Error())
	}
}

func mysqlCalc(t time.Time) *MysqlMonitor {
	mysqlMonitor := new(MysqlMonitor)

	// 当前连接数
	mysqlMonitor.ThreadsConnected, _ = GetStatus(db, "Threads_connected")

	// 活跃连接数
	mysqlMonitor.ThreadsRunning, _ = GetStatus(db, "Threads_running")

	// QPS
	queries, _ := GetStatus(db, "Queries")
	mysqlMonitor.Qps = (queries - lastQueries) / 60
	lastQueries = queries

	// 慢查询数量
	slowQueries, _ := GetStatus(db, "Slow_queries")
	mysqlMonitor.SlowQueries = slowQueries - lastSlowQueries
	lastSlowQueries = slowQueries

	// 请求缓存池数
	readReq, _ := GetStatus(db, "Innodb_buffer_pool_read_requests")
	// 读取缓存池
	reads, _ := GetStatus(db, "Innodb_buffer_pool_reads")
	if readReq > 0 {
		mysqlMonitor.BufferHitRate = util.ToDouble(float64(readReq-reads) * 100 / float64(readReq))
	}

	// 磁盘IO
	currIO, _ := disk.IOCounters()
	for device, curr := range currIO {
		prev, ok := prevIO[device]
		if !ok {
			continue
		}

		readBytes := curr.ReadBytes - prev.ReadBytes
		writeBytes := curr.WriteBytes - prev.WriteBytes

		mysqlMonitor.WriteSpeed = util.ToDouble(util.ToMbFloat(readBytes / 60))
		mysqlMonitor.ReadSpeed = util.ToDouble(util.ToMbFloat(writeBytes / 60))
	}
	prevIO = currIO

	mysqlMonitor.CreatedAt = t.Truncate(time.Minute)
	return mysqlMonitor
}
