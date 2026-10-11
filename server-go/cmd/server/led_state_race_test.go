package main

import (
	"buzzcontrol/internal/protocol"
	"fmt"
	"sync"
	"testing"
)

// The LED state map is written from several goroutines (handlers, comet restore
// timers, ACK retry): it must be safe under -race.
func TestSendLEDSet_ConcurrentWritersAndReadersAreRaceFree(t *testing.T) {
	app := newTestAppWithHub(t)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				mac := fmt.Sprintf("aa:bb:cc:dd:ee:%02d", (g+i)%7)
				app.sendLEDSet(mac, protocol.LEDSetPayload{Color: [3]int{g, i, 0}})
				app.resendLEDOnReconnect(mac)
			}
		}(g)
	}
	wg.Wait()
}
