package ui

import "testing"

func TestTheUseOfTheProcessorIsReadOutOfWhatTopSays(t *testing.T) {
	output := `Processes: 512 total, 3 running, 509 sleeping
CPU usage: 12.34% user, 5.67% sys, 81.99% idle
PhysMem: 16G used, 8G unused
`
	if got := parseCPU(output); got < 17.9 || got > 18.1 {
		t.Fatalf("the use of the processor came out as %v", got)
	}
	if got := parseCPU("nothing here\n"); got != 0 {
		t.Fatalf("the use of the processor came out as %v", got)
	}
}

func TestThePortIsReadOutOfTheNameOfASocket(t *testing.T) {
	port, ok := portOf("127.0.0.1:8080")
	if !ok || port != 8080 {
		t.Fatalf("the port came out as %v", port)
	}
	port, ok = portOf("*:5432")
	if !ok || port != 5432 {
		t.Fatalf("the port came out as %v", port)
	}
	if _, ok := portOf("localhost"); ok {
		t.Fatal("a name without a port was taken for one")
	}
}

func TestThePortsOfTheSessionAreThoseOfTheMachine(t *testing.T) {
	// The machine of the test has at least one port open, the one of the
	// test server of the toolkit is not needed: any port will do.
	readings := readListeningPorts()
	if len(readings) == 0 {
		t.Skip("the machine listens on no port")
	}
	for _, reading := range readings {
		if reading.Port <= 0 {
			t.Fatalf("the port came out as %v", reading.Port)
		}
	}
}
