package taskmaster

import (
	"errors"
	"strings"
)

const (
	MaxTaskBytes   = 4096
	MaxResultBytes = 8192
)

func ValidateTask(task string) error {
	if strings.TrimSpace(task) == "" {
		return errors.New("task must not be empty")
	}
	if len(task) > MaxTaskBytes {
		return errors.New("task exceeds size cap")
	}
	return nil
}

func ValidateResult(result string) error {
	if strings.TrimSpace(result) == "" {
		return errors.New("result must not be empty")
	}
	if len(result) > MaxResultBytes {
		return errors.New("result exceeds size cap")
	}
	return nil
}
