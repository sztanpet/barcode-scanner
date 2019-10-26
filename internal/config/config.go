package config

import (
	"bytes"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/juju/loggo"
)

var logger = loggo.GetLogger("main.config")

type Config struct {
	StatePath         string
	UpdateBaseURL     string
	DatabaseDSN       string
	TelegramToken     string
	TelegramChannelID int64
	MachineID         string
	HardwareVersion   int64
}

func Get() *Config {
	StatePath := os.Getenv("STATE_PATH")
	if StatePath == "" {
		logger.Criticalf("Empty STATE_PATH env var!")
		os.Exit(1)
	}

	UpdateBaseURL := os.Getenv("UPDATE_BASEURL")
	if UpdateBaseURL == "" {
		logger.Criticalf("Empty UPDATE_BASEURL env var!")
		os.Exit(1)
	}

	DatabaseDSN := os.Getenv("DATABASE_DSN")
	if DatabaseDSN == "" {
		logger.Criticalf("Empty DATABASE_DSN env var!")
		os.Exit(1)
	}

	TelegramToken := os.Getenv("TELEGRAM_TOKEN")
	if TelegramToken == "" {
		logger.Criticalf("Empty TELEGRAM_TOKEN env var!")
		os.Exit(1)
	}

	cid := os.Getenv("TELEGRAM_CHANNELID")
	if cid == "" {
		logger.Criticalf("Empty TELEGRAM_CHANNELID env var!")
		os.Exit(1)
	}

	TelegramChannelID, err := strconv.ParseInt(cid, 10, 64)
	if err != nil {
		logger.Criticalf("Failed parsing TELEGRAM_CHANNELID env var!")
		os.Exit(1)
	}

	hwv := os.Getenv("HARDWARE_VERSION")
	if hwv == "" {
		hwv = "1"
	}

	HardwareVersion, err := strconv.ParseInt(hwv, 10, 64)
	if err != nil {
		logger.Criticalf("Failed parsing HARDWARE_VERSION env var!")
		os.Exit(1)
	}

	return &Config{
		StatePath:         StatePath,
		UpdateBaseURL:     UpdateBaseURL,
		DatabaseDSN:       DatabaseDSN,
		TelegramToken:     TelegramToken,
		TelegramChannelID: TelegramChannelID,
		MachineID:         machineID(),
		HardwareVersion:   HardwareVersion,
	}
}

func machineID() string {
	mid, err := ioutil.ReadFile("/etc/machine-id")
	if err != nil {
		panic("failed reading /etc/machine-id: " + err.Error())
	}

	mid = bytes.TrimSpace(mid)
	if len(mid) != 32 {
		panic("invalid contents of /etc/machine-id: " + string(mid))
	}

	return string(mid)
}
