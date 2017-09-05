package hidgadget
import (
	 "github.com/jteeuwen/evdev"
	 "os"
	 "fmt"
)

/*
 * mapping
 *
 * Maps scancodes to USB event IDs
 *
 * This is based on the linux kernel file drivers/input/hid-input.c
 * by inverting the mapping defined in hid_keyboard[]
 */
var mapping []byte = []byte{3, 41, 30, 31, 32, 33, 34, 35, 36, 37, 38,
	39, 45, 46, 42, 43, 20, 26, 8, 21, 23, 28, 24, 12, 18, 19,
	47, 48, 40, 224, 4, 22, 7, 9, 10, 11, 13, 14, 15, 51, 52,
	53, 225, 50, 29, 27, 6, 25, 5, 17, 16, 54, 55, 56, 229, 85,
	226, 44, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 67, 83,
	71, 95, 96, 97, 86, 92, 93, 94, 87, 89, 90, 91, 98, 99, 0,
	148, 100, 68, 69, 135, 146, 147, 138, 136, 139, 140, 88, 228,
	84, 70, 230, 0, 74, 82, 75, 80, 79, 77, 81, 78, 73, 76, 0,
	239, 238, 237, 102, 103, 0, 72, 0, 133, 144, 145, 137, 227,
	231, 101, 243, 121, 118, 122, 119, 124, 116, 125, 244, 123,
	117, 0, 251, 0, 248, 0, 0, 0, 0, 0, 0, 0, 240, 0,
	249, 0, 0, 0, 0, 0, 241, 242, 0, 236, 0, 235, 232, 234,
	233, 0, 0, 0, 0, 0, 0, 250, 0, 0, 247, 245, 246, 182,
	183, 0, 0, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113,
	114}

/*
 * Map modifier events to the modifier bitcode
 */
var kb_mod map[uint16]byte = map[uint16]byte {
	29: 0x01,	// --left-ctrl
	97: 0x10,	// --right-ctrl
	42: 0x02,	// --left-shift
	54: 0x20,	// --right-shift
	56: 0x04,	// --left-alt
	100: 0x40,	// --right-alt
	125: 0x08,	// --left-meta
	126: 0x80,	// --right-meta
}

/*
 * USBHid structure for maintainig state of a USB Hid instance
 */
type UsbHid struct {
	ev	chan evdev.Event
	exit	chan bool
	file	*os.File
	report  [8]byte
	keys	int
}

func Test() {
	path := "/dev/hidg0"

	testEvents := []evdev.Event{
		evdev.Event{Type: evdev.EvKeys, Code: evdev.KeyH, Value: 1},
		evdev.Event{Type: evdev.EvKeys, Code: evdev.KeyH, Value: 0},
		evdev.Event{Type: evdev.EvKeys, Code: evdev.KeyLeftShift, Value: 1},
		evdev.Event{Type: evdev.EvKeys, Code: evdev.KeyE, Value: 1},
		evdev.Event{Type: evdev.EvKeys, Code: evdev.KeyE, Value: 0},
		evdev.Event{Type: evdev.EvKeys, Code: evdev.KeyL, Value: 1},
		evdev.Event{Type: evdev.EvKeys, Code: evdev.KeyL, Value: 0},
		evdev.Event{Type: evdev.EvKeys, Code: evdev.KeyLeftShift, Value: 0},
		evdev.Event{Type: evdev.EvKeys, Code: evdev.KeyL, Value: 1},
		evdev.Event{Type: evdev.EvKeys, Code: evdev.KeyL, Value: 0},
		evdev.Event{Type: evdev.EvKeys, Code: evdev.KeyO, Value: 1},
		evdev.Event{Type: evdev.EvKeys, Code: evdev.KeyO, Value: 0},
		evdev.Event{Type: evdev.EvKeys, Code: evdev.KeyA, Value: 1},
		evdev.Event{Type: evdev.EvKeys, Code: evdev.KeyB, Value: 1},
		evdev.Event{Type: evdev.EvKeys, Code: evdev.KeyC, Value: 1},
		evdev.Event{Type: evdev.EvKeys, Code: evdev.KeyA, Value: 0},
		evdev.Event{Type: evdev.EvKeys, Code: evdev.KeyB, Value: 0},
		evdev.Event{Type: evdev.EvKeys, Code: evdev.KeyC, Value: 0},
	}

	hid, err := Open(path)
	if err != nil {
		fmt.Println("ERROR: Could not open ", path)
		return
	}
	defer hid.Close()

	for _, ev := range testEvents {
		hid.ForwardEvent(ev)
	}
}

func (hid *UsbHid) ForwardEvent(ev evdev.Event) {
	hid.ev <- ev
}

func Open(path string) (*UsbHid, error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	hid := new(UsbHid)
	hid.ev = make(chan evdev.Event)
	hid.exit = make(chan bool)
	hid.file = file

	go eventWriter(hid)

	return hid, nil
}

func (hid *UsbHid) Close() {
	hid.exit <- true
}

func (hid *UsbHid) updateReport(ev evdev.Event) {

	if ev.Type == evdev.EvKeys {

		if kb_mod[ev.Code] != 0 {
			// This code is a modifier
			if ev.Value != 0 {
				hid.report[0] |= kb_mod[ev.Code]
			} else {
				hid.report[0] &^= kb_mod[ev.Code]
			}

		} else {
			// This code is a normal key
			if mapping[ev.Code] == 0 {
				fmt.Printf("Warning: No mapping for event code: %d\n", ev.Code)
				return
			}

			keyPos := -1
			for i, c := range hid.report[2:2+hid.keys] {
				if c == mapping[ev.Code] {
					keyPos = i
					break
				}
			}

			if keyPos != -1 {
				if ev.Value == 0 {
					// When removing a key from the middle of the byte
					if hid.keys > keyPos {
						hid.report[keyPos+2] = hid.report[hid.keys+1]
					}
					hid.report[hid.keys+1] = 0
					hid.keys--
				}
			} else {
				if hid.keys < len(hid.report) -2 {
					hid.report[2+hid.keys] = mapping[ev.Code]
					hid.keys++
				} else {
					hid.report[len(hid.report)-1] = mapping[ev.Code]
				}
			}
		}
	}
}

func eventWriter(hid *UsbHid) {

	defer hid.file.Close()

	for {
		select {
		case ev := <-hid.ev:
			fmt.Printf("Got event: Code = %d, Value = %d, mapping = %d\n", ev.Code, ev.Value, mapping[ev.Code])
			hid.updateReport(ev)
			n, _ := hid.file.Write(hid.report[:])
			if n != len(hid.report) {
				fmt.Println("ERROR: Write failed")
				return
			}

			hid.file.Sync()
		case <-hid.exit:
			return
		}
	}
}

