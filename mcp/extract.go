package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/salvadorsru/dali/assets"
	"github.com/salvadorsru/dali/model"
)

type extractInput struct {
	File string `json:"file,omitempty" jsonschema:"fig file to query; defaults to the one dali was started with"`
	Name string `json:"name,omitempty" jsonschema:"optional asset name substring filter"`
	Hash string `json:"hash,omitempty" jsonschema:"optional single image hash; when set only that asset is extracted"`
}

type extractOutput struct {
	Dir     string           `json:"dir"`
	Skipped int              `json:"skipped"`
	Assets  []model.ImageRef `json:"assets"`
}

// extractAssets writes images to <base>.assets/images and reports the written
// paths, enriched with mime, size and dimensions. Unlike get_image it returns
// file paths instead of base64, so it fits large images without bloating the
// conversation.
func (h *handlers) extractAssets(ctx context.Context, req *mcp.CallToolRequest, in extractInput) (*mcp.CallToolResult, extractOutput, error) {
	svc, err := h.service(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, extractOutput{}, err
	}
	refs, err := svc.Assets()
	if err != nil {
		return nil, extractOutput{}, err
	}
	var selected []model.ImageRef
	for _, a := range refs {
		if in.Hash != "" && a.Hash != in.Hash {
			continue
		}
		if in.Name != "" && !containsFold(a.Name, in.Name) {
			continue
		}
		selected = append(selected, a)
	}
	if len(selected) == 0 {
		if in.Hash != "" || in.Name != "" {
			return nil, extractOutput{}, fmt.Errorf("no asset matches hash=%q name=%q", in.Hash, in.Name)
		}
		return nil, extractOutput{Assets: []model.ImageRef{}}, nil
	}
	abs, err := h.resolve(in.File)
	if err != nil {
		return nil, extractOutput{}, err
	}
	out, err := assets.Extract(abs, selected)
	if err != nil {
		return nil, extractOutput{}, err
	}
	if err := svc.UpdateAssetInfo(out); err != nil {
		return nil, extractOutput{}, err
	}
	res := extractOutput{
		Dir:     assets.ImagesDir(abs),
		Skipped: len(selected) - len(out),
		Assets:  out,
	}
	if res.Assets == nil {
		res.Assets = []model.ImageRef{}
	}
	return nil, res, nil
}
