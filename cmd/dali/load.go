package main

import (
	"errors"

	"github.com/salvadorsru/dali/model"
	"github.com/salvadorsru/dali/parser"
)

func load(path string) (*model.Document, error) {
	f, err := parser.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return model.Build(f.NodeChanges()), nil
}

func arg(args []string, i int, name string) (string, error) {
	if i >= len(args) {
		return "", errors.New("missing argument: " + name)
	}
	return args[i], nil
}
