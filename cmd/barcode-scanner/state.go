package main

import (
	"strings"
	"time"
	"unicode"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/storage"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/tty"
)

type State int

const (
	readBarcode State = iota
	wifiSetupSSID
	wifiSetupPW
	wifiSetupDone
)

func (a *app) transitionState(r rune) {
	logger.Tracef("key pressed: %x %q", r, r)

	switch a.state {
	case wifiSetupSSID, wifiSetupPW, wifiSetupDone:
		if r == tty.KeyEscape {
			logger.Debugf("State: wifiSetup (%v) -> readBarcode (escape pressed)", a.state)
			a.cancelWifiSetup()
			return
		}

		a.handleWifiSetupInput(r)

	case readBarcode:
		switch r {
		case tty.KeyEscape:
			logger.Debugf("State: readBarcode -> wifiSetup (escape pressed)")
			a.enterWifiSetup()
			return

		case '\r', '\n':
			a.handleBarcodeDone()
		default:
			a.handleBarcodeInput(r)
		}
	}
}

// enterReadBarcode is called by cancelWifiSetup and doneWifiSetup
func (a *app) enterReadBarcode() {
	a.state = readBarcode
	a.currentLine.Reset()

	// clear and init screen
	_ = a.screen.Clear()
	_ = a.screen.WriteTitle("SCANNER")
	_ = a.screen.WriteLine(1, "Barcode data:")
	_ = a.screen.WriteHelp("waiting for scan")
}

// handleBarcodeInput is only called by transitionState
// it appends the new rune to a.currentLine and displays it on the screen
func (a *app) handleBarcodeInput(r rune) {
	if r > unicode.MaxASCII || !unicode.IsPrint(r) {
		logger.Debugf("handleBarcodeInput: got invalid input: %x %q, ignoring", r, r)
		return
	}

	_, _ = a.currentLine.WriteRune(r)
}

// handleBarcodeDone signals that a new barcode is available in a.currentLine
func (a *app) handleBarcodeDone() {
	bc := a.currentLine.String()
	a.currentLine.Reset()

	line := strings.TrimSpace(bc)
	if len(line) == 0 {
		logger.Debugf("handleBarcodeInput: empty currentLine, skipping")
		return
	}

	_ = a.screen.WriteLine(2, bc)
	if a.handleSpecialBarcode(bc) {
		return
	}

	a.storage.Insert(storage.Barcode{
		Barcode:        bc,
		Direction:      a.dir.String(),
		CurrierService: a.currier,
		CreatedAt:      time.Now(),
	})
}

func (a *app) handleSpecialBarcode(bc string) bool {
	switch bc {
	case "TODO1":
		a.dir = EGRESS
	case "TODO2":
		a.dir = INGRESS
	default:
		return false
	}

	a.persistSettings()
	return true
}
