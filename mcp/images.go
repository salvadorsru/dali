package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/salvadorsru/dali/parser"
)

type imageInput struct {
	File string `json:"file,omitempty" jsonschema:"fig file to query; defaults to the one dali was started with"`
	Hash string `json:"hash,omitempty" jsonschema:"image hash from find_assets"`
	Name string `json:"name,omitempty" jsonschema:"asset name; used to resolve the hash when hash is empty"`
}

type imageOutput struct {
	Hash   string `json:"hash"`
	Name   string `json:"name,omitempty"`
	MIME   string `json:"mime"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	Data   string `json:"data,omitempty" jsonschema:"base64-encoded image data"`
}

func (h *handlers) getImage(ctx context.Context, req *mcp.CallToolRequest, in imageInput) (*mcp.CallToolResult, imageOutput, error) {
	note := newNotifier(ctx, req)
	svc, err := h.service(in.File, note)
	if err != nil {
		return nil, imageOutput{}, err
	}
	hash := in.Hash
	name := in.Name
	if hash == "" && name == "" {
		return nil, imageOutput{}, errors.New("pass either \"hash\" or \"name\"")
	}
	if hash == "" {
		assets, err := svc.Assets()
		if err != nil {
			return nil, imageOutput{}, err
		}
		for _, a := range assets {
			if a.Name == name {
				hash = a.Hash
				break
			}
		}
		if hash == "" {
			return nil, imageOutput{}, fmt.Errorf("no asset named %q", name)
		}
	}
	abs, err := h.resolve(in.File)
	if err != nil {
		return nil, imageOutput{}, err
	}
	f, err := parser.Open(abs)
	if err != nil {
		return nil, imageOutput{}, err
	}
	defer f.Close()
	data, err := f.ReadImage(hash)
	if err != nil {
		return nil, imageOutput{}, err
	}
	out := imageOutput{
		Hash: hash,
		MIME: http.DetectContentType(data),
		Data: base64.StdEncoding.EncodeToString(data),
	}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		out.Width, out.Height = cfg.Width, cfg.Height
	}
	if name != "" {
		out.Name = name
	}
	return nil, out, nil
}
