package domain

import "errors"

var (
	ErrInvalidSerial      = errors.New("serial телефона не указан")
	ErrObserverUnavailable = errors.New("observer недоступен")
	ErrRecoveryUnavailable = errors.New("recovery-engine недоступен")
	ErrRecoveryFailed      = errors.New("recovery не вернул решение")
	ErrExecutorUnavailable = errors.New("executor недоступен")
)
