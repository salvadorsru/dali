package ir

import (
	"github.com/salvadorsru/dali/model"
	"github.com/salvadorsru/dali/query"
)

// ComponentSpec is the complete specification for generating a component.
type ComponentSpec struct {
	ID           string
	Name         string
	Type         string
	Variants     []ComponentVariant
	Props        []ComponentProp
	Children     []NodeSpec
	DefaultProps map[string]any
	Layout       LayoutSpec
	Styles       StyleSpec
	Assets       []AssetSpec
	TextClasses  []TextClassSpec
}

// ComponentVariant represents a variant in a component set.
type ComponentVariant struct {
	ID       string
	Name     string
	Props    map[string]any
	Children []NodeSpec
	Layout   LayoutSpec
	Styles   StyleSpec
}

// ComponentProp is a property that can be overridden on instances.
type ComponentProp struct {
	ID       string
	Name     string
	Type     string
	Default  any
	Values   []any
	Required bool
}

// NodeSpec represents a node in the component tree.
type NodeSpec struct {
	ID          string
	Type        string
	Name        string
	ComponentID string
	IsInstance  bool
	Props       []ComponentProp
	Children    []NodeSpec
	Layout      LayoutSpec
	Styles      StyleSpec
	Text        TextSpec
	Vector      VectorSpec
	Images      []AssetSpec
}

// TextSpec holds text-specific properties.
type TextSpec struct {
	Content       string
	Runs          []TextRunSpec
	FontFamily    string
	FontStyle     string
	FontSize      float64
	LineHeight    float64
	LetterSpacing float64
	TextAlign     string
	Decoration    string
	Color         string
}

// TextRunSpec represents a styled text run.
type TextRunSpec struct {
	Start         int
	Length        int
	Text          string
	FontFamily    string
	FontStyle     string
	FontSize      float64
	Color         string
	Decoration    string
	LineHeight    float64
	LetterSpacing float64
}

// VectorSpec holds vector path data.
type VectorSpec struct {
	Path string
}

// LayoutSpec translates Figma auto-layout to CSS flexbox/grid.
type LayoutSpec struct {
	Mode              string // NONE, HORIZONTAL, VERTICAL, GRID
	Direction         string // row, column, row-reverse, column-reverse
	Gap               float64
	Padding           PaddingSpec
	AlignItems        string
	JustifyContent    string
	AlignContent      string
	PrimaryAxisSizing string // FIXED, HUG, FILL
	CounterAxisSizing string
	CornerRadius      float64
	CornerRadii       [4]float64
	StrokeWeight      float64
	StrokeColor       string
	StrokeAlign       string
	Constraints       ConstraintsSpec
	Width             float64 // absolute rendered width (FIXED primary-axis sizing)
	Height            float64 // absolute rendered height (FIXED counter-axis sizing)
}

// PaddingSpec represents padding on all sides.
type PaddingSpec struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
}

// ConstraintsSpec represents min/max width/height constraints.
type ConstraintsSpec struct {
	MinWidth  float64
	MaxWidth  float64
	MinHeight float64
	MaxHeight float64
}

// StyleSpec aggregates visual styles.
type StyleSpec struct {
	Fills       []FillSpec
	Strokes     []StrokeSpec
	Effects     []EffectSpec
	Opacity     float64
	Rotation    float64
	ClipContent bool
	Background  FillSpec
	BlendMode   string
	Visible     bool
}

// FillSpec represents a fill paint.
type FillSpec struct {
	Type      string // SOLID, GRADIENT_LINEAR, GRADIENT_RADIAL, GRADIENT_ANGULAR, IMAGE
	Color     string
	Opacity   float64
	Gradient  *GradientSpec
	Image     *AssetSpec
	BlendMode string
	Visible   bool
}

// GradientSpec represents gradient parameters.
type GradientSpec struct {
	Stops       []GradientStop
	StartPoint  Point
	EndPoint    Point
	Angle       float64
	InnerRadius float64
}

// GradientStop is a color stop in a gradient.
type GradientStop struct {
	Position float64
	Color    string
	Opacity  float64
}

// Point represents a 2D point.
type Point struct {
	X float64
	Y float64
}

// StrokeSpec represents a stroke paint.
type StrokeSpec struct {
	Type   string
	Color  string
	Width  float64
	Align  string // INSIDE, CENTER, OUTSIDE
	Cap    string
	Join   string
	Dashes []float64
}

// EffectSpec represents a visual effect.
type EffectSpec struct {
	Type      string // DROP_SHADOW, INNER_SHADOW, LAYER_BLUR, BACKGROUND_BLUR
	Color     string
	Offset    Point
	Radius    float64
	Spread    float64
	Visible   bool
	BlendMode string
}

// AssetSpec represents an image asset.
type AssetSpec struct {
	Hash   string
	Name   string
	MIME   string
	Width  int
	Height int
	Data   []byte
	Path   string // local path if extracted
}

// TextClassSpec represents a text style class.
type TextClassSpec struct {
	Family string
	Style  string
	Size   float64
	Usage  int
}

// TokenSet aggregates all design tokens for a document.
type TokenSet struct {
	Colors      []query.SolidColor
	ColorVars   []model.Variable
	Fonts       []model.Font
	TextClasses []query.TextClass
	Layouts     []query.TokenLayout
}

// GeneratorConfig controls code generation.
type GeneratorConfig struct {
	OutputDir        string
	ComponentDir     string
	AssetsDir        string
	TokensFile       string
	GenerateIndex    bool
	ResponsiveImages bool
	ImageSizes       []int
	CSSVarPrefix     string
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() GeneratorConfig {
	return GeneratorConfig{
		ComponentDir:     "components",
		AssetsDir:        "assets",
		TokensFile:       "tokens.css",
		GenerateIndex:    true,
		ResponsiveImages: false,
		ImageSizes:       []int{1, 2},
		CSSVarPrefix:     "dali",
	}
}
