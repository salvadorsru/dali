package codegen

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/salvadorsru/dali/codegen/html"
	"github.com/salvadorsru/dali/codegen/ir"
	"github.com/salvadorsru/dali/codegen/tokens"
	"github.com/salvadorsru/dali/codegen/util"
	"github.com/salvadorsru/dali/query"
)

// GenerateComponents generates code for a list of component IDs.
func GenerateComponents(svc *query.Service, componentIDs []string, config ir.GeneratorConfig) error {
	gen := html.HTMLGenerator{}

	// Collect tokens once
	tokenSet, err := tokens.CollectTokens(svc)
	if err != nil {
		return fmt.Errorf("collect tokens: %w", err)
	}

	// Ensure output directories
	if err := os.MkdirAll(config.OutputDir, 0o755); err != nil {
		return err
	}
	componentDir := filepath.Join(config.OutputDir, config.ComponentDir)
	if err := os.MkdirAll(componentDir, 0o755); err != nil {
		return err
	}
	assetsDir := filepath.Join(config.OutputDir, config.AssetsDir)
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		return err
	}

	// Generate each component
	var components []ir.ComponentSpec
	for _, id := range componentIDs {
		spec, err := util.BuildComponentSpec(svc, id)
		if err != nil {
			return fmt.Errorf("build component %s: %w", id, err)
		}
		components = append(components, spec)

		code, err := gen.GenerateComponent(spec, tokenSet, config)
		if err != nil {
			return fmt.Errorf("generate component %s: %w", id, err)
		}

		filename := util.SanitizeComponentName(spec.Name)
		path := filepath.Join(componentDir, filename+".html")
		if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
			return err
		}
	}

	// Generate tokens file
	tg := tokens.TokenGenerator{}
	tokenCode, err := tg.GenerateCSSVariables(tokenSet, config)
	if err != nil {
		return fmt.Errorf("generate tokens: %w", err)
	}
	tokenPath := filepath.Join(config.OutputDir, config.TokensFile)
	if err := os.WriteFile(tokenPath, []byte(tokenCode), 0o644); err != nil {
		return err
	}

	// Generate index
	if config.GenerateIndex {
		indexCode, err := gen.GenerateIndex(components, config)
		if err != nil {
			return fmt.Errorf("generate index: %w", err)
		}
		indexPath := filepath.Join(componentDir, "index.html")
		if err := os.WriteFile(indexPath, []byte(indexCode), 0o644); err != nil {
			return err
		}
	}

	return nil
}

// GenerateAllComponents generates code for all components in the document.
func GenerateAllComponents(svc *query.Service, config ir.GeneratorConfig) error {
	components, err := svc.Components()
	if err != nil {
		return err
	}
	var ids []string
	for _, c := range components {
		if c.Type.IsComponent() {
			ids = append(ids, c.ID)
		}
	}
	return GenerateComponents(svc, ids, config)
}

// GenerateTokens generates token files only.
func GenerateTokens(svc *query.Service, config ir.GeneratorConfig, format string) error {
	tokenSet, err := tokens.CollectTokens(svc)
	if err != nil {
		return err
	}

	var code string
	tg := tokens.TokenGenerator{}
	switch format {
	case "css", "css-vars":
		code, err = tg.GenerateCSSVariables(tokenSet, config)
	case "style-dict", "styledictionary":
		code, err = tg.GenerateStyleDictionary(tokenSet, config)
	case "figma", "figma-tokens":
		code, err = tg.GenerateFigmaTokens(tokenSet, config)
	default:
		return fmt.Errorf("unknown token format: %s", format)
	}
	if err != nil {
		return err
	}

	if err := os.MkdirAll(config.OutputDir, 0o755); err != nil {
		return err
	}

	ext := map[string]string{
		"css":        ".css",
		"style-dict": ".json",
		"figma":      ".json",
	}[format]
	if ext == "" {
		ext = ".txt"
	}

	path := filepath.Join(config.OutputDir, "tokens"+ext)
	return os.WriteFile(path, []byte(code), 0o644)
}
