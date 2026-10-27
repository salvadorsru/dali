package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/salvadorsru/dali/internal/query"
)

// vectorView is query.Vector with the assembled SVG attached. The svg field is
// what an agent pastes; path, view_box and paints stay on the wire so a caller
// that wants a different size or colour does not have to reparse markup.
type vectorView struct {
	query.Vector
	SVG string `json:"svg"`
}

type vectorOutput struct {
	Vectors []vectorView `json:"vectors"`
}

// getVector returns the exact vector resources under a node — an instance, a
// component or a single vector — as paste-ready SVG plus the paints each
// placement overrode. Geometry is resolved against the component-side node, so
// the result is the vector as authored even when the placement is scaled.
func (h *handlers) getVector(ctx context.Context, req *mcp.CallToolRequest, in idInput) (*mcp.CallToolResult, vectorOutput, error) {
	svc, err := h.service(in.File, newNotifier(ctx, req))
	if err != nil {
		return nil, vectorOutput{}, err
	}
	node, err := svc.Node(in.ID)
	if err != nil {
		return nil, vectorOutput{}, err
	}
	if node == nil {
		return nil, vectorOutput{}, fmt.Errorf("node %q not found", in.ID)
	}
	vectors, err := svc.Vectors(in.ID)
	if err != nil {
		return nil, vectorOutput{}, err
	}
	out := vectorOutput{Vectors: make([]vectorView, 0, len(vectors))}
	for _, v := range vectors {
		out.Vectors = append(out.Vectors, vectorView{Vector: v, SVG: v.SVG()})
	}
	return nil, out, nil
}
