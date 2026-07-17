package counter

import (
	"reflect"
	"testing"
)

func TestDeduplicatePreservesFirstOccurrence(t *testing.T) {
	got := Deduplicate([]string{"alarm-7", "alarm-7", "alarm-9"})
	want := []string{"alarm-7", "alarm-9"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Deduplicate() = %v, want %v", got, want)
	}
}
