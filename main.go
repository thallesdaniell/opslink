package main

import (
	"opslink/cmd"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

func main() {
	_ = godotenv.Load()

	logrus.WithField("cmd", "main").Info("[OPSLINK-CLI]")
	cmd.Execute()
}
