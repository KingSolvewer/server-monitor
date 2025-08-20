package configuration

import (
	"fmt"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"strconv"
)

type Config struct {
	DbHost     string
	DbUsername string
	DbPassword string
	DbName     string
	DbPort     int
	WebNode    int
}

var (
	db     *gorm.DB
	config *Config
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

	config = &Config{
		DbHost:     viper.GetString("DB_HOST"),
		DbUsername: viper.GetString("DB_USERNAME"),
		DbPassword: viper.GetString("DB_PASSWORD"),
		DbName:     viper.GetString("DB_NAME"),
		DbPort:     viper.GetInt("DB_PORT"),
		WebNode:    viper.GetInt("WEB_NODE"),
	}
	if config.DbHost == "" {
		config.DbHost = "localhost"
	}
	if config.DbUsername == "" {
		config.DbUsername = "root"
	}
	if config.DbPort == 0 {
		config.DbPort = 3306
	}

	dsn := config.DbUsername + ":" + config.DbPassword + "@tcp(" + config.DbHost + ":" + strconv.Itoa(config.DbPort) + ")/" + config.DbName + "?charset=utf8mb4&parseTime=True&loc=Local"

	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{NamingStrategy: schema.NamingStrategy{IdentifierMaxLength: 64, SingularTable: true}})
	if err != nil {
		panic("Gorm init error: " + err.Error())
	}
}

func GetConfig() *Config {
	return config
}

func GetDb() *gorm.DB {
	return db
}
