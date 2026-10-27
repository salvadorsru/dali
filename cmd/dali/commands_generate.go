package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/salvadorsru/dali/internal/assets"
	"github.com/salvadorsru/dali/internal/codegen"
	"github.com/salvadorsru/dali/internal/codegen/ir"
	"github.com/salvadorsru/dali/internal/mcp"
	"github.com/salvadorsru/dali/internal/output"
)

func cmdGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	outputDir := fs.String("out", "./generated", "output directory")
	node := fs.String("id", "", "node ID to generate (default: all)")
	byName := fs.String("name", "", "node name substring to generate (default: all)")
	tokensFormat := fs.String("tokens", "css", "tokens format: css, style-dict, figma")
	styleMode := fs.String("style", "inline", "style mode: inline or tailwind")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *styleMode != "inline" && *styleMode != "tailwind" {
		return fmt.Errorf("unknown style mode %q: use inline or tailwind", *styleMode)
	}

	svc, err := open(figFile)
	if err != nil {
		return err
	}
	defer svc.Close()

	config := ir.DefaultConfig()
	config.OutputDir = *outputDir
	config.Style = *styleMode

	// A single node to generate, resolved by id or by name
	if *node != "" && *byName != "" {
		return fmt.Errorf("use either --id or --name, not both")
	}
	if *byName != "" {
		nodes, err := svc.Query(fmt.Sprintf("name~%q", *byName))
		if err != nil {
			return err
		}
		if len(nodes) == 0 {
			if sugs, serr := svc.SuggestNames(*byName, 8); serr == nil && suggestHint(sugs, *byName) {
				return nil
			}
			return fmt.Errorf("no node named %q", *byName)
		}
		if len(nodes) > 1 {
			for _, n := range nodes {
				fmt.Printf("%s  %s  %s  %s\n", output.ID(n.ID), output.Type(string(n.Type)), truncate(n.Name, 40), output.Dim(n.PageID))
			}
			return fmt.Errorf("%q matches %d nodes; pick one with --id", *byName, len(nodes))
		}
		return codegen.GenerateComponents(svc, []string{nodes[0].ID}, config)
	}

	// If id specified, generate just that one
	if *node != "" {
		return codegen.GenerateComponents(svc, []string{*node}, config)
	}

	// Generate tokens if requested
	if *tokensFormat != "" {
		if err := codegen.GenerateTokens(svc, config, *tokensFormat); err != nil {
			return fmt.Errorf("generate tokens: %w", err)
		}
	}

	// Generate all components
	return codegen.GenerateAllComponents(svc, config)
}

func cmdExtractAssets(args []string) error {
	svc, err := open(figFile)
	if err != nil {
		return err
	}
	defer svc.Close()

	refs, err := svc.Assets()
	if err != nil {
		return err
	}
	out, err := assets.Extract(figFile, refs)
	if err != nil {
		return err
	}
	if err := svc.UpdateAssetInfo(out); err != nil {
		return err
	}
	fmt.Printf("extracted %d assets to %s\n", len(out), assets.ImagesDir(figFile))
	return nil
}

func cmdMCP(args []string) error {
	mcp.Version = version
	fig, root := mcpMode(figFile)
	return mcp.Serve(context.Background(), fig, root)
}
