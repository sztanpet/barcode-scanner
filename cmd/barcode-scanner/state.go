package main

type State int

const (
	readBarcode State = iota
	wifiSetupSSID
	wifiSetupPW
	wifiSetupDone
)

func (s State) String() string {
	switch s {
	case readBarcode:
		return "readBarcode"
	case wifiSetupSSID:
		return "wifiSetupSSID"
	case wifiSetupPW:
		return "wifiSetupPW"
	case wifiSetupDone:
		return "wifiSetupDone"
	default:
		panic("unknown state " + string(rune(s+'0')))
	}
}

/*
default: readBarcode state

readBarcode:
  - on escape -> wifiSetup
  - on enter -> readBarcodeDone
  - on invalid char -> ignore
  - on valid char -> append to currentLine
readBarcodeDone:
  - handle special barcode, not inserted into db
  - handle insertion into db
  - when done -> readBarcode

wifiSetup (wifiSetupSSID, wifiSetupPW, wifiSetupDone), default: wifiSetupSSID
wifiSetupSSID:
  - on escape -> readBarcode
  - on invalid char -> ignore
  - on valid char -> append to currentLine, display on screen
  - on backspace/delete -> delete last char from currentLine, display on screen
  - on enter -> save currentLine as SSID, transition to wifiSetupPW
wifiSetupPW:
  - on escape -> readBarcode
  - on invalid char -> ignore
  - on valid char -> append to currentLine, display on screen
  - on backspace/delete -> delete last char from currentLine, display on screen
  - on enter -> save currentLine as PW, transition to wifiSetupDone
wifiSetupDone:
  - display pre-setup message on screen
  - do setup (might take time)
  - show result on screen
  - wait 2 seconds so user can read it
  - transition back to readBarcode
*/
func (a *app) transitionState(r rune) {
	//logger.Tracef("key pressed: %x %q", r, r)

	switch a.state {
	case wifiSetupSSID, wifiSetupPW, wifiSetupDone:
		a.handleWifiSetupInput(r)

	case readBarcode:
		a.handleReadBarcode(r)
	}
}
