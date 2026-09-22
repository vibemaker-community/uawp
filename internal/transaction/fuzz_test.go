package transaction

import (
	"bytes"
	"testing"
)

func FuzzJournal(f *testing.F) {
	for _, seed := range [][]byte{[]byte(`{}`), []byte(`{"schemaVersion":"2"}`), []byte(`{"a":1,"a":2}`), []byte("\x00"), bytes.Repeat([]byte("x"), 1024)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeJournal(bytes.NewReader(data))
		_, _ = DecodeLivePointer(bytes.NewReader(data))
		_, _ = DecodeReceipt(bytes.NewReader(data))
	})
}

func FuzzClassification(f *testing.F) {
	f.Add([]byte(`{}`), []byte(`{}`))
	f.Fuzz(func(t *testing.T, journal, planData []byte) {
		j, jErr := DecodeJournal(bytes.NewReader(journal))
		p, pErr := DecodePlan(bytes.NewReader(planData))
		if jErr != nil || pErr != nil {
			return
		}
		bundle := Bundle{Journal: j, Plan: p}
		_ = Classify(bundle, Observation{Paths: map[string]PathState{}, Inputs: map[string]string{}, Backups: map[int]BackupState{}})
	})
}
