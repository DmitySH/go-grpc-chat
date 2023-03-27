package config

import (
	"github.com/spf13/viper"
	"log"
)

func MustLoadConfig(cfgPath string) {
	viper.SetConfigFile(cfgPath)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Fatal("can't load config:", err)
	}
}
