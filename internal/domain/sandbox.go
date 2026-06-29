package domain

import (
	"errors"
	"strings"
)

var ErrSandboxSerial = errors.New("sandbox serial запрещён для регистрации")

func IsSandboxSerial(serial string) bool {
	s := strings.ToLower(strings.TrimSpace(serial))
	return s == "stub" || strings.HasPrefix(s, "e2e-")
}
