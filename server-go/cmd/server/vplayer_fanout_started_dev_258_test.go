package main

import (
	"bytes"
	"testing"
)

// #258: cost of the per-recipient UPDATE during an open-answers phase (STARTED) at
// 20 tablets, against the single shared frame the same phase used to cost.

func BenchmarkVPlayerFanout_Started_20_QCM(b *testing.B) {
	msg := buildFanoutBenchMessage(20, benchQCMQuestion())
	msg.Msg = bytes.Replace(msg.Msg, []byte(`"PHASE":"READY"`), []byte(`"PHASE":"STARTED"`), 1)
	msg.Msg = bytes.Replace(msg.Msg, []byte(`"PHASE":"PREPARE"`), []byte(`"PHASE":"STARTED"`), 1)
	recipients := buildFanoutBenchRecipients(20)
	if payloads, ok := buildVPlayerPayloads(msg, recipients); !ok || len(payloads) != 20 {
		b.Fatalf("STARTED must build 20 per-recipient frames (ok=%v, n=%d)", ok, len(payloads))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buildVPlayerPayloads(msg, recipients)
	}
}

// The pre-#258 cost of the same phase: ONE shared frame for every tablette.
func BenchmarkVPlayerFanout_Started_20_QCM_SharedFrameBaseline(b *testing.B) {
	msg := buildFanoutBenchMessage(20, benchQCMQuestion())
	msg.Msg = bytes.Replace(msg.Msg, []byte(`"PHASE":"READY"`), []byte(`"PHASE":"STARTED"`), 1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := msg.SerializeForVPlayerCommon(); err != nil {
			b.Fatal(err)
		}
	}
}
