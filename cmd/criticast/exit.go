package main

import "errors"

type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

func exitCode(err error) int {
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	return 1
}

func exitErr(code int, msg string) error {
	return &exitError{code: code, msg: msg}
}
