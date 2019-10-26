package main

import (
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/buzzer"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/gpio"
)

func (a *app) onBootup() {
	if a.cfg.HardwareVersion < 2 {
		if err := buzzer.StartupBeep(); err != nil {
			logger.Warningf("buzzer beep error: %v", err)
		}
	} else {
		gpio.RedLED.Disable()
		if err := gpio.GreenLED.Enable(); err != nil {
			logger.Criticalf("failed to switch green led on: %v", err)
		}
	}
}

func (a *app) onShutdown() {
	if a.cfg.HardwareVersion >= 2 {
		gpio.GreenLED.Disable()
		_ = gpio.RedLED.Enable()
	}
}

func (a *app) successFeedback() {
	if a.cfg.HardwareVersion < 2 {
		if err := buzzer.StartupBeep(); err != nil {
			logger.Warningf("buzzer beep error: %v", err)
		}
	} else {
		if err := gpio.Fail(a.ctx); err != nil {
			logger.Infof("gpio.Fail failed: %v", err)
		}
	}
}

func (a *app) failFeedback() {
	if a.cfg.HardwareVersion < 2 {
		if err := buzzer.FailBeep(); err != nil {
			logger.Infof("buzzer.FailBeep failed: %v", err)
		}
	} else {
		if err := gpio.Fail(a.ctx); err != nil {
			logger.Infof("gpio.Fail failed: %v", err)
		}
	}
}
