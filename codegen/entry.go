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
	contents, err := Generate(svc, componentIDs, "css", config)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(config.OutputDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(config.OutputDir, config.ComponentDir), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(config.OutputDir, config.AssetsDir), 0o755); err != nil {
		return err
	}
	for rel, code := range contents {
		if err := os.WriteFile(filepath.Join(config.OutputDir, rel), []byte(code), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Generate renders components, index and tokens into an in-memory map keyed
// by their path relative to OutputDir, without touching disk. tokensFormat is
// "", "css", "style-dict" or "figma"; empty omits the tokens file.
func Generate(svc *query.Service, componentIDs []string, tokensFormat string, config ir.GeneratorConfig) (map[string]string, error) {
	gen := html.HTMLGenerator{}
	tokenSet, err := tokens.CollectTokens(svc)
	if err != nil {
		return nil, fmt.Errorf("collect tokens: %w", err)
	}

	out := make(map[string]string)

	var components []ir.ComponentSpec
	for _, id := range componentIDs {
		spec, err := util.BuildComponentSpec(svc, id)
		if err != nil {
			return nil, fmt.Errorf("build component %s: %w", id, err)
		}
		components = append(components, spec)

		code, err := gen.GenerateComponent(spec, tokenSet, config)
		if err != nil {
			return nil, fmt.Errorf("generate component %s: %w", id, err)
		}
		out[filepath.Join(config.ComponentDir, util.SanitizeComponentName(spec.Name)+".html")] = code
	}

	if config.GenerateIndex && len(components) > 0 {
		indexCode, err := gen.GenerateIndex(components, config)
		if err != nil {
			return nil, fmt.Errorf("generate index: %w", err)
		}
		out[filepath.Join(config.ComponentDir, "index.html")] = indexCode
	}

	if tokensFormat != "" {
		tg := tokens.TokenGenerator{}
		var code, name string
		switch tokensFormat {
		case "css", "css-vars":
			code, err = tg.GenerateCSSVariables(tokenSet, config)
			name = config.TokensFile
		case "style-dict", "styledictionary":
			code, err = tg.GenerateStyleDictionary(tokenSet, config)
			name = "tokens.json"
		case "figma", "figma-tokens":
			code, err = tg.GenerateFigmaTokens(tokenSet, config)
			name = "tokens.json"
		default:
			return nil, fmt.Errorf("unknown token format: %s", tokensFormat)
		}
		if err != nil {
			return nil, fmt.Errorf("generate tokens: %w", err)
		}
		out[name] = code
	}

	return out, nil
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
	contents, err := Generate(svc, nil, format, config)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(config.OutputDir, 0o755); err != nil {
		return err
	}
	for rel, code := range contents {
		if err := os.WriteFile(filepath.Join(config.OutputDir, rel), []byte(code), 0o644); err != nil {
			return err
		}
	}
	return nil
}
