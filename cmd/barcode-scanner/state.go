package main

import (
	"strings"
	"unicode"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/tty"
)

type State int

const (
	readBarcode State = iota
	wifiSetupSSID
	wifiSetupPW
)

func (a *app) transitionState(r rune) {
	logger.Debugf("key pressed: %x %q", r, r)

	switch a.state {
	case wifiSetupSSID, wifiSetupPW:
		if r == tty.KeyEscape {
			logger.Debugf("State: wifiSetup (%v) -> readBarcode (escape pressed)", a.state)
			a.cancelWifiSetup()
			return
		}

		a.handleWifiSetupInput(r)
		return
	case readBarcode:
		switch r {
		case tty.KeyEscape:
			logger.Debugf("State: readBarcode -> wifiSetup (escape pressed)")
			a.enterWifiSetup()
			return
		case '\r', '\n':
		default:
			a.handleBarcodeInput(r)
		}
	}
}

// enterReadBarcode is called by cancelWifiSetup and doneWifiSetup
func (a *app) enterReadBarcode() {
	// clear and init screen
}

// handleBarcodeInput is only called by transitionState
// it appends the new rune to a.currentLine and displays it on the screen
func (a *app) handleBarcodeInput(r rune) {
	if r > unicode.MaxASCII || !unicode.IsPrint(r) {
		logger.Debugf("handleBarcodeInput: got invalid input: %x %q, ignoring", r, r)
		return
	}

	_, _ = a.currentLine.WriteRune(r)
	// TODO display input on screen
}

// handleBarcodeDone signals that a new barcode is available in a.currentLine
func (a *app) handleBarcodeDone() {
	line := strings.TrimSpace(a.currentLine.String())
	if len(line) == 0 {
		logger.Debugf("handleBarcodeInput: empty currentLine, skipping")
		return
	}

	// TODO assemble storage.Barcode and insert it, retrying as needed
	a.currentLine.Reset()
}
