package taskmaster

import "testing"

func TestValidateTask_RejectsEmpty(t *testing.T) {
	if err := ValidateTask(""); err == nil {
		t.Fatal("expected error for empty task, got nil")
	}
}

func TestValidateTask_RejectsWhitespaceOnly(t *testing.T) {
	if err := ValidateTask("   "); err == nil {
		t.Fatal("expected error for whitespace-only task, got nil")
	}
}

func TestValidateTask_AcceptsPlainTask(t *testing.T) {
	if err := ValidateTask("summarize my inbox"); err != nil {
		t.Fatalf("expected nil for valid task, got %v", err)
	}
}

func TestValidateTask_RejectsOversizeTask(t *testing.T) {
	big := make([]byte, 4097)
	for i := range big {
		big[i] = 'a'
	}
	if err := ValidateTask(string(big)); err == nil {
		t.Fatal("expected error for 4097-byte task, got nil")
	}
}

func TestValidateTask_AcceptsMaxSizeTask(t *testing.T) {
	ok := make([]byte, 4096)
	for i := range ok {
		ok[i] = 'a'
	}
	if err := ValidateTask(string(ok)); err != nil {
		t.Fatalf("expected nil for 4096-byte task, got %v", err)
	}
}

func TestValidateResult_RejectsEmpty(t *testing.T) {
	if err := ValidateResult(""); err == nil {
		t.Fatal("expected error for empty result, got nil")
	}
}

func TestValidateResult_AcceptsPlainResult(t *testing.T) {
	if err := ValidateResult("You have 3 emails. The urgent one is from Ana."); err != nil {
		t.Fatalf("expected nil for valid result, got %v", err)
	}
}

func TestValidateResult_RejectsOversizeResult(t *testing.T) {
	big := make([]byte, 8193)
	for i := range big {
		big[i] = 'a'
	}
	if err := ValidateResult(string(big)); err == nil {
		t.Fatal("expected error for 8193-byte result, got nil")
	}
}

func TestValidateResult_AcceptsMaxSizeResult(t *testing.T) {
	ok := make([]byte, 8192)
	for i := range ok {
		ok[i] = 'a'
	}
	if err := ValidateResult(string(ok)); err != nil {
		t.Fatalf("expected nil for 8192-byte result, got %v", err)
	}
}
