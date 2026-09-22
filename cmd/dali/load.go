package main

import (
	"errors"

	"github.com/salvadorsru/dali/internal/model"
	"github.com/salvadorsru/dali/internal/parser"
)

func load(path string) (*model.Document, error) {
	f, err := parser.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return model.Build(f.NodeChanges(), f.Blobs()), nil
}

func arg(args []string, i int, name string) (string, error) {
	if i >= len(args) {
		return "", errors.New("missing argument: " + name)
	}
	return args[i], nil
}
