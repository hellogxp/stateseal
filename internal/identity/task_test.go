package identity

import "testing"

func TestTaskIDValidationRejectsPathTraversal(t *testing.T) {
	for _, value := range []string{"", "../escape", "nested/task", ".hidden", "space task"} {
		if err := ValidateTaskID(value); err == nil {
			t.Fatalf("accepted unsafe task ID %q", value)
		}
	}
	for _, value := range []string{"payment-idempotency", "StateSeal.Core_1"} {
		if err := ValidateTaskID(value); err != nil {
			t.Fatalf("rejected task ID %q: %v", value, err)
		}
	}
}

func TestNormalizeTaskIDProducesValidIdentifier(t *testing.T) {
	for input, want := range map[string]string{
		"Payment Service": "Payment-Service",
		"../repo":         "repo",
		"测试仓库":            "stateseal-task",
	} {
		got := NormalizeTaskID(input)
		if got != want {
			t.Fatalf("NormalizeTaskID(%q) = %q, want %q", input, got, want)
		}
		if err := ValidateTaskID(got); err != nil {
			t.Fatalf("normalized ID is invalid: %v", err)
		}
	}
}
