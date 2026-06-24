package domain

import "errors"

var (
	ErrInvalidSerial       = errors.New("serial телефона не указан")
	ErrPhoneNotFound       = errors.New("телефон не найден")
	ErrPhoneAlreadyExists  = errors.New("телефон уже зарегистрирован")
	ErrObserverUnavailable = errors.New("observer недоступен")
	ErrRecoveryUnavailable = errors.New("recovery-engine недоступен")
	ErrRecoveryFailed      = errors.New("recovery не вернул решение")
	ErrExecutorUnavailable = errors.New("executor недоступен")
	ErrConnectorUnavailable = errors.New("connector недоступен")
	ErrStoreUnavailable    = errors.New("хранилище недоступно")
	ErrLockNotAcquired     = errors.New("телефон обрабатывается другой репликой")
)
