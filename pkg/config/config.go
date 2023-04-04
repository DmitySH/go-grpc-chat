package config

import (
	"github.com/spf13/viper"
	"log"
)

func LoadEnvConfig(cfgPath string) {
	viper.SetConfigFile(cfgPath)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Println("can't load env config:", err)
		log.Println("using system envs")
	}
}
