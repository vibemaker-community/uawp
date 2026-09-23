package transaction

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vibemaker-community/uawp/internal/plan"
)

func TestJournalCodecStrictRoundTrip(t *testing.T) {
	want := validJournal(t)
	encoded, err := EncodeJournal(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeJournal(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if got.TransactionID != want.TransactionID || got.Actions[0].Change.Path != want.Actions[0].Change.Path {
		t.Fatalf("decoded journal = %#v", got)
	}
}

func TestStrictCodecsRejectDuplicateUnknownTrailingAndOversizedJSON(t *testing.T) {
	valid := validJournal(t)
	encoded, err := EncodeJournal(valid)
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string][]byte{
		"duplicate": bytes.Replace(encoded, []byte(`"schemaVersion": "1"`), []byte(`"schemaVersion": "1", "schemaVersion": "1"`), 1),
		"unknown":   bytes.Replace(encoded, []byte(`"schemaVersion": "1"`), []byte(`"schemaVersion": "1", "unknown": true`), 1),
		"trailing":  append(append([]byte(nil), encoded...), []byte(` {}`)...),
		"oversized": []byte(`{"schemaVersion":"1","padding":"` + strings.Repeat("x", MaxRecordBytes) + `"}`),
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeJournal(bytes.NewReader(raw)); err == nil {
				t.Fatal("DecodeJournal accepted hostile JSON")
			}
		})
	}
}

func TestPointerReceiptAndPlanCodecsRoundTrip(t *testing.T) {
	journal := validJournal(t)
	pointer := LivePointer{SchemaVersion: PointerSchemaVersion, TransactionID: journal.TransactionID, Operation: journal.Operation, PlanID: journal.PlanID, Workspace: journal.Workspace, Phase: journal.Phase}
	rawPointer, err := EncodeLivePointer(pointer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeLivePointer(bytes.NewReader(rawPointer)); err != nil {
		t.Fatal(err)
	}
	receipt := Receipt{SchemaVersion: ReceiptSchemaVersion, TransactionID: "txn-001", Operation: journal.Operation, PlanID: journal.PlanID, CompletedAt: "2026-09-22T12:30:00Z", Result: ResultCompleted, Hashes: map[string]string{".uawp/CONTEXT.md": strings.Repeat("a", 64)}}
	rawReceipt, err := EncodeReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeReceipt(bytes.NewReader(rawReceipt)); err != nil {
		t.Fatal(err)
	}

	p := plan.NewForWorkspace("create", "/workspace", []plan.Change{plan.NewFile(".uawp/file.md", 0o600, plan.MissingSHA256, []byte("payload"))})
	rawPlan, err := EncodePlan(p.Persisted())
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodePlan(bytes.NewReader(rawPlan))
	if err != nil {
		t.Fatal(err)
	}
	restored, err := plan.Restore(decoded)
	if err != nil || restored.ID != p.ID {
		t.Fatalf("restored plan = %#v, %v", restored, err)
	}
}
