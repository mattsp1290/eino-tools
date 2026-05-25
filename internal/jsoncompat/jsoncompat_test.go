package jsoncompat

import "testing"

func TestRejectDuplicateTopLevelKeys(t *testing.T) {
	t.Parallel()

	if err := RejectDuplicateTopLevelKeys([]byte(`{"path":"a","path":"b"}`)); err == nil {
		t.Fatal("expected duplicate top-level key error")
	}
	if err := RejectDuplicateTopLevelKeys([]byte(`{"path":"a","nested":{"path":"b"}}`)); err != nil {
		t.Fatalf("nested duplicate-like key returned error: %v", err)
	}
	if err := RejectDuplicateTopLevelKeys([]byte(`{not-json`)); err != nil {
		t.Fatalf("malformed JSON returned helper error instead of deferring to unmarshal: %v", err)
	}
}
