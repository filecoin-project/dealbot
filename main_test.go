package main

import (
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestFoo(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/scripts",
	})
}
