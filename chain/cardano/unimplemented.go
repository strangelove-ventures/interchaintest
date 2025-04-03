package cardano

import (
	"errors"
	"runtime"
)

func errNotImplemented() error {
	pc, _, _, _ := runtime.Caller(1)
	return errors.New(runtime.FuncForPC(pc).Name() + " not implemented")
}
