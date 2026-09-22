package main

import "github.com/salvadorsru/dali/query"

func open(path string) (*query.Service, error) {
	return query.Open(path)
}

func openWithID(args []string) (string, *query.Service, error) {
	id, err := arg(args, 0, "id")
	if err != nil {
		return "", nil, err
	}
	svc, err := open(figFile)
	if err != nil {
		return "", nil, err
	}
	return id, svc, nil
}