package main

import (
	"testing"
	"time"
)

func TestParsePingOutput(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want float64
		ok   bool
	}{
		{"linux iputils", "64 bytes from 192.168.1.42: icmp_seq=1 ttl=255 time=12.3 ms", 12.3, true},
		{"linux sub-ms", "64 bytes from 10.0.0.5: icmp_seq=1 ttl=64 time=0.412 ms", 0.412, true},
		{"windows en", "Reply from 192.168.1.42: bytes=32 time=7ms TTL=255", 7, true},
		{"windows en <1ms", "Reply from 192.168.1.1: bytes=32 time<1ms TTL=64", 0.5, true},
		{"windows fr", "Réponse de 192.168.1.42 : octets=32 temps=15 ms TTL=255", 15, true},
		{"windows fr <1ms", "Réponse de 192.168.1.1 : octets=32 temps<1ms TTL=64", 0.5, true},
		{"windows fr comma", "Réponse de 192.168.1.42 : octets=32 temps=3,5 ms TTL=255", 3.5, true},
		{"timeout linux", "PING 192.168.1.42 56(84) bytes of data.\n\n--- 192.168.1.42 ping statistics ---\n1 packets transmitted, 0 received, 100% packet loss, time 0ms", 0, false},
		{"timeout windows", "Request timed out.", 0, false},
		{"unreachable windows", "Reply from 192.168.1.10: Destination host unreachable.", 0, false},
		{"empty", "", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parsePingOutput(tt.out)
			if ok != tt.ok || (ok && got != tt.want) {
				t.Errorf("parsePingOutput = %v,%v want %v,%v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestPingArgs(t *testing.T) {
	win := pingArgs("windows", "10.0.0.1", 800*time.Millisecond)
	if len(win) != 5 || win[0] != "-n" || win[2] != "-w" || win[3] != "800" || win[4] != "10.0.0.1" {
		t.Errorf("windows args = %v", win)
	}
	lin := pingArgs("linux", "10.0.0.1", 800*time.Millisecond)
	if len(lin) != 5 || lin[0] != "-c" || lin[2] != "-W" || lin[3] != "1" || lin[4] != "10.0.0.1" {
		t.Errorf("linux args = %v (timeout must round up to >= 1 s)", lin)
	}
}

func TestClassifyLogLine(t *testing.T) {
	var c LogCounters
	lines := []struct{ level, msg string }{
		{"INFO", "AckManager: retry 1/3 for msgID=abc mac=AA:BB action=LED_SET"},
		{"WARN", "AckManager: EXPIRED after 3 retries for msgID=abc mac=AA:BB action=LED_SET"},
		{"DEBUG", "ACK received from AA:BB: action=LED_SET msgID=abc"},
		{"INFO", "Buzzer connected via WebSocket: x (MAC: AA:BB)"},
		{"INFO", "Buzzer disconnected from WebSocket: x (MAC: AA:BB)"},
		{"INFO", "BUTTON from AA:BB: A"},
		{"INFO", "BUTTON from AA:BB ignored (phase=STOPPED, not STARTED)"},
		{"ERROR", "something else"},
	}
	for _, l := range lines {
		classifyLogLine(l.level, l.msg, &c)
	}
	want := LogCounters{Entries: 8, AckReceived: 1, AckRetry: 1, AckExpired: 1, BuzzerConnected: 1, BuzzerDisconnected: 1, ButtonPress: 1, Warn: 1, Error: 1}
	if c != want {
		t.Errorf("counters = %+v, want %+v", c, want)
	}
}

func TestParseLogMessage(t *testing.T) {
	frame := []byte(`{"ACTION":"LOG_ENTRY","MSG":{"entry":{"timestamp":1,"level":"WARN","component":"App","message":"AckManager: EXPIRED after 3 retries"}}}`)
	level, msg, ok := parseLogMessage(frame)
	if !ok || level != "WARN" || msg != "AckManager: EXPIRED after 3 retries" {
		t.Errorf("parseLogMessage = %q %q %v", level, msg, ok)
	}
	if _, _, ok := parseLogMessage([]byte(`{"ACTION":"LOG_HISTORY","MSG":{}}`)); ok {
		t.Error("LOG_HISTORY must be ignored")
	}
	if _, _, ok := parseLogMessage([]byte(`not json`)); ok {
		t.Error("garbage must be ignored")
	}
}

func TestLogsURL(t *testing.T) {
	for in, want := range map[string]string{
		"http://192.168.1.10":      "ws://192.168.1.10/ws/logs",
		"http://192.168.1.10:8080": "ws://192.168.1.10:8080/ws/logs",
		"https://buzz.local/":      "wss://buzz.local/ws/logs",
	} {
		got, err := logsURL(in)
		if err != nil || got != want {
			t.Errorf("logsURL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := logsURL("ftp://x"); err == nil {
		t.Error("ftp scheme must fail")
	}
}

func TestStatsPercentiles(t *testing.T) {
	var s Samples
	for i := 1; i <= 100; i++ {
		s.AddMs(float64(i))
	}
	st := s.Stats()
	if st.Count != 100 || st.MinMs != 1 || st.MaxMs != 100 || st.P50Ms != 50 || st.P95Ms != 95 || st.P99Ms != 99 || st.MeanMs != 50.5 {
		t.Errorf("stats = %+v", st)
	}
	if (&Samples{}).Stats().Count != 0 {
		t.Error("empty samples must give zero stats")
	}
	if got := percentile([]float64{7}, 95); got != 7 {
		t.Errorf("single-sample percentile = %v", got)
	}
}

func TestProberBucketsByPhase(t *testing.T) {
	p := newProber([]string{"h1"})
	p.setPhase("baseline")
	p.record("h1", 5, true)
	p.record("h1", 0, false)
	p.setPhase("ble-idle")
	p.record("h1", 9, true)
	snap := p.snapshot()
	b := snap["baseline"]["h1"]
	if b.Sent != 2 || b.Lost != 1 || b.LossPct != 50 || b.RTT.Count != 1 || b.RTT.P50Ms != 5 {
		t.Errorf("baseline bucket = %+v", b)
	}
	if i := snap["ble-idle"]["h1"]; i.Sent != 1 || i.Lost != 0 || i.RTT.P50Ms != 9 {
		t.Errorf("ble-idle bucket = %+v", i)
	}
}
