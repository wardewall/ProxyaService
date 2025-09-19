package main

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"

	"ProxyaService/internal/auth"
	"ProxyaService/internal/bot"
	"ProxyaService/internal/config"
	"ProxyaService/internal/logger"
)

func main() {
	_ = godotenv.Load()

	conf := config.Load()
	log := logger.New(conf.LogLevel)

	if conf.BotToken == "" {
		log.Error("BOT token is not set. Define TOKEN or BOT_TOKEN in environment/.env")
		os.Exit(1)
	}

	a := auth.New(conf.AllowedUserIDs, conf.AuthTokens)
	b := bot.New(log, conf, a)

	if err := b.Start(); err != nil {
		log.Error("bot stopped with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
