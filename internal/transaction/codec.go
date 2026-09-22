package transaction

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/uawp/uawp/internal/plan"
)

func EncodeJournal(value Journal) ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, err
	}
	return encode(value)
}

func DecodeJournal(reader io.Reader) (Journal, error) {
	var value Journal
	if err := decodeStrict(reader, &value); err != nil {
		return Journal{}, err
	}
	if err := value.Validate(); err != nil {
		return Journal{}, err
	}
	return value, nil
}

func EncodeLivePointer(value LivePointer) ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, err
	}
	return encode(value)
}

func DecodeLivePointer(reader io.Reader) (LivePointer, error) {
	var value LivePointer
	if err := decodeStrict(reader, &value); err != nil {
		return LivePointer{}, err
	}
	if err := value.Validate(); err != nil {
		return LivePointer{}, err
	}
	return value, nil
}

func EncodeReceipt(value Receipt) ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, err
	}
	return encode(value)
}

func DecodeReceipt(reader io.Reader) (Receipt, error) {
	var value Receipt
	if err := decodeStrict(reader, &value); err != nil {
		return Receipt{}, err
	}
	if err := value.Validate(); err != nil {
		return Receipt{}, err
	}
	return value, nil
}

func EncodePlan(value plan.PersistedPlan) ([]byte, error) {
	if _, err := plan.Restore(value); err != nil {
		return nil, err
	}
	return encode(value)
}

func DecodePlan(reader io.Reader) (plan.PersistedPlan, error) {
	var value plan.PersistedPlan
	if err := decodeStrict(reader, &value); err != nil {
		return plan.PersistedPlan{}, err
	}
	if _, err := plan.Restore(value); err != nil {
		return plan.PersistedPlan{}, err
	}
	return value, nil
}

func encode(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	if len(data)+1 > MaxRecordBytes {
		return nil, fmt.Errorf("transaction record exceeds %d bytes", MaxRecordBytes)
	}
	return append(data, '\n'), nil
}

func decodeStrict(reader io.Reader, target any) error {
	limited := io.LimitReader(reader, MaxRecordBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if len(data) > MaxRecordBytes {
		return fmt.Errorf("transaction record exceeds %d bytes", MaxRecordBytes)
	}
	if err := rejectDuplicateKeys(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if token, err := decoder.Token(); err != io.EOF {
		if err != nil {
			return err
		}
		return fmt.Errorf("unexpected trailing JSON token %v", token)
	}
	return nil
}

func rejectDuplicateKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := walkJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err != nil {
			return err
		}
		return fmt.Errorf("unexpected trailing JSON")
	}
	return nil
}

func walkJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			if seen[key] {
				return fmt.Errorf("duplicate JSON field %q", key)
			}
			seen[key] = true
			if err := walkJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return fmt.Errorf("unterminated JSON object")
		}
	case '[':
		for decoder.More() {
			if err := walkJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return fmt.Errorf("unterminated JSON array")
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
	return nil
}
