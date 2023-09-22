package interchain

import (
	"fmt"
	"testing"
)

// This is a fake impl of testing.T so that ICtest is happy to use it for logging.

type FakeI interface {
	testing.T
}

type FakeTesting struct {
	FakeName string
}

// impl all of testing.T for FakeTesting
func (t FakeTesting) Name() string {
	return t.FakeName
}

func (t FakeTesting) Cleanup(func()) {
}

func (t FakeTesting) Skip(...any) {
}

func (t FakeTesting) Parallel() {
}

func (t FakeTesting) Failed() bool {
	return false
}

func (t FakeTesting) Skipped() bool {
	return false
}

func (t FakeTesting) Error(...any) {
}

func (t FakeTesting) Errorf(format string, args ...any) {
}

func (t FakeTesting) Fail() {
}

func (t FakeTesting) FailNow() {
}

func (t FakeTesting) Fatal(...any) {
}

func (t FakeTesting) Helper() {
}

func (t FakeTesting) Logf(format string, args ...any) {
	fmt.Printf(format, args...)
}
