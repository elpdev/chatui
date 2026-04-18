package transport

import "errors"

var ErrUnauthorized = errors.New("relay unauthorized")

func IsUnauthorized(err error) bool {
	return errors.Is(err, ErrUnauthorized)
}
