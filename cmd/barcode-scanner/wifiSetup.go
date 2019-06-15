package main

import (
	"unicode"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/tty"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/wifi"
)

var wifiInfo struct {
	ssid, pw string
} = struct {
	ssid, pw string
}{}

// enterWifiSetup is only called by transitionState
func (a *app) enterWifiSetup() {
	a.state = wifiSetupSSID
	wifiInfo.ssid = ""
	wifiInfo.pw = ""
	// TODO prepare screen
}

// handleWifiSetupInput is only called by transitionState
func (a *app) handleWifiSetupInput(r rune) {
	switch r {
	case '\r':
		line := a.currentLine.String()
		switch a.state {
		case wifiSetupSSID:
			wifiInfo.ssid = line
		case wifiSetupPW:
			wifiInfo.pw = line
			err := wifi.Setup(wifiInfo.ssid, wifiInfo.pw)
			if err != nil {
				// TODO display something on screen
				logger.Criticalf("wifi setup error: %v", err)
			}
			a.doneWifiSetup()
		}
	case tty.KeyBackspace, tty.KeyDelete:
		if a.currentLine.Len() >= 1 {
			// https://stackoverflow.com/questions/39907667/how-to-remove-unicode-characters-from-byte-buffer-in-go
			a.currentLine.Truncate(a.currentLine.Len() - 1)
			// TODO display line again
			logger.Tracef("handleWifiSetupInput: backspace")
		}
	default:
		if unicode.IsPrint(r) {
			_, _ = a.currentLine.WriteRune(r)
		}
	}
}

// cancelWifiSetup is only called by transitionState
func (a *app) cancelWifiSetup() {
	a.state = readBarcode
	wifiInfo.ssid = ""
	wifiInfo.pw = ""
	a.enterReadBarcode()
}

// doneWifiSetup is called by handleWifiSetupInput (NOT transitionState)
func (a *app) doneWifiSetup() {
	logger.Debugf("State: wifiSetup -> readBarcode (doneWifiSetup)")
	a.cancelWifiSetup()
}
