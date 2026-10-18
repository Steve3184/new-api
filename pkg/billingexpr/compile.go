package billingexpr

import (
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/parser"
	"github.com/expr-lang/expr/vm"
)

const maxCacheSize = 256

// DefaultExprVersion is used when an expression string has no version prefix.
const DefaultExprVersion = 1

const (
	requestRuleTraceFunction    = "_trace"
	requestRuleTraceIntFunction = "_trace_int"
)

// ParseExprVersion extracts the version tag and body from an expression string.
// Format: "v1:tier(...)" → version=1, body="tier(...)".
// No prefix defaults to DefaultExprVersion.
func ParseExprVersion(exprStr string) (version int, body string) {
	if strings.HasPrefix(exprStr, "v1:") {
		return 1, exprStr[3:]
	}
	return DefaultExprVersion, exprStr
}

// requestRulePatcher adds trace side effects to existing request multipliers
// without changing the stored expression or its numeric result.
type requestRulePatcher struct {
	requestRules         []RequestRuleTrace
	restrictedIdentifier string
}

func (p *requestRulePatcher) Visit(node *ast.Node) {
	if identifier, ok := (*node).(*ast.IdentifierNode); ok {
		switch identifier.Value {
		case requestRuleTraceFunction, requestRuleTraceIntFunction:
			p.restrictedIdentifier = identifier.Value
		}
		return
	}

	conditional, ok := (*node).(*ast.ConditionalNode)
	if !ok || !conditional.Ternary || !usesRequestProbe(conditional.Cond) {
		return
	}
	multiplier, ok := requestRuleNumber(conditional.Exp1)
	fallback, fallbackOK := requestRuleNumber(conditional.Exp2)
	if !ok || !fallbackOK || fallback != 1 {
		return
	}

	ruleIndex := len(p.requestRules)
	p.requestRules = append(p.requestRules, RequestRuleTrace{
		Cond:       conditional.Cond.String(),
		Multiplier: multiplier,
	})

	traceFunction := requestRuleTraceFunction
	var multiplierNode ast.Node = &ast.FloatNode{Value: multiplier}
	if _, multiplierIsInt := conditional.Exp1.(*ast.IntegerNode); multiplierIsInt {
		if _, fallbackIsInt := conditional.Exp2.(*ast.IntegerNode); fallbackIsInt {
			traceFunction = requestRuleTraceIntFunction
			multiplierNode = conditional.Exp1
		}
	}

	ast.Patch(node, &ast.CallNode{
		Callee: &ast.IdentifierNode{Value: traceFunction},
		Arguments: []ast.Node{
			&ast.IntegerNode{Value: ruleIndex},
			conditional.Cond,
			multiplierNode,
		},
	})
}

func requestRuleNumber(node ast.Node) (float64, bool) {
	switch value := node.(type) {
	case *ast.IntegerNode:
		return float64(value.Value), true
	case *ast.FloatNode:
		return value.Value, true
	default:
		return 0, false
	}
}

func usesRequestProbe(node ast.Node) bool {
	return ast.Find(node, func(node ast.Node) bool {
		identifier, ok := node.(*ast.IdentifierNode)
		if !ok {
			return false
		}
		switch identifier.Value {
		case "group", "param", "header", "image_resolution", "hour", "minute", "weekday", "month", "day":
			return true
		default:
			return false
		}
	}) != nil
}

type cachedEntry struct {
	prog          *vm.Program
	usedVars      map[string]bool
	usedUsageKeys map[string]bool
	requestRules  []RequestRuleTrace
	version       int
	fixedPricing  bool
}

var (
	cacheMu sync.RWMutex
	cache   = make(map[string]*cachedEntry, 64)
)

// compileEnvPrototypeV1 is the v1 type-checking prototype used at compile time.
var compileEnvPrototypeV1 = map[string]any{
	"group":            "",
	"image_count":      float64(1),
	"p":                float64(0),
	"c":                float64(0),
	"len":              float64(0),
	"cr":               float64(0),
	"cc":               float64(0),
	"cc1h":             float64(0),
	"img":              float64(0),
	"img_cr":           float64(0),
	"img_o":            float64(0),
	"ai":               float64(0),
	"ao":               float64(0),
	"req":              float64(0),
	"tier":             func(string, float64) float64 { return 0 },
	"fixed":            func(float64) float64 { return 0 },
	"_trace":           func(int, bool, float64) float64 { return 1 },
	"_trace_int":       func(int, bool, int) int { return 1 },
	"header":           func(string) string { return "" },
	"param":            func(string) any { return nil },
	"u":                func(string) any { return nil },
	"image_resolution": func() string { return "" },
	"has":              func(any, string) bool { return false },
	"hour":             func(string) int { return 0 },
	"minute":           func(string) int { return 0 },
	"weekday":          func(string) int { return 0 },
	"month":            func(string) int { return 0 },
	"day":              func(string) int { return 0 },
	"max":              math.Max,
	"min":              math.Min,
	"abs":              math.Abs,
	"ceil":             math.Ceil,
	"floor":            math.Floor,
}

func getCompileEnv(version int) map[string]any {
	switch version {
	default:
		return compileEnvPrototypeV1
	}
}

// CompileFromCache compiles an expression string, using a cached program when
// available. The cache is keyed by the SHA-256 hex digest of the expression.
func CompileFromCache(exprStr string) (*vm.Program, error) {
	return compileFromCacheByHash(exprStr, ExprHashString(exprStr))
}

// CompileFromCacheByHash is like CompileFromCache but accepts a pre-computed
// hash, useful when the caller already has the BillingSnapshot.ExprHash.
func CompileFromCacheByHash(exprStr, hash string) (*vm.Program, error) {
	return compileFromCacheByHash(exprStr, hash)
}

func compileFromCacheByHash(exprStr, hash string) (*vm.Program, error) {
	entry, err := compileEntryFromCacheByHash(exprStr, hash)
	if err != nil {
		return nil, err
	}
	return entry.prog, nil
}

func compileEntryFromCacheByHash(exprStr, hash string, group ...string) (*cachedEntry, error) {
	if len(group) > 0 {
		hash += ":group:" + ExprHashString(group[0])
	}
	cacheMu.RLock()
	if entry, ok := cache[hash]; ok {
		cacheMu.RUnlock()
		return entry, nil
	}
	cacheMu.RUnlock()

	version, body := ParseExprVersion(exprStr)
	// Validate fixed-price leaves before expression optimization so an
	// unreachable invalid branch cannot bypass billing safety checks.
	tree, err := parser.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("expr compile error: %w", err)
	}
	fixedPricing := ast.Find(tree.Node, func(node ast.Node) bool {
		identifier, ok := node.(*ast.IdentifierNode)
		return ok && identifier.Value == "fixed"
	}) != nil
	if fixedPricing {
		if err := validateFixedPricingTree(tree.Node); err != nil {
			return nil, fmt.Errorf("expr compile error: %w", err)
		}
	}
	patcher := &requestRulePatcher{}
	options := []expr.Option{expr.Env(getCompileEnv(version)), expr.Patch(patcher), expr.AsFloat64()}
	if len(group) > 0 {
		options = append(options, expr.Patch(groupPatcher{group[0]}))
	}
	prog, err := expr.Compile(body, options...)
	if patcher.restrictedIdentifier != "" {
		return nil, fmt.Errorf("expr compile error: identifier %q is reserved for internal use", patcher.restrictedIdentifier)
	}
	if err != nil {
		return nil, fmt.Errorf("expr compile error: %w", err)
	}

	entry := &cachedEntry{
		prog:          prog,
		usedVars:      extractUsedVars(prog),
		usedUsageKeys: extractUsedUsageKeys(prog),
		requestRules:  patcher.requestRules,
		version:       version,
		fixedPricing:  fixedPricing,
	}
	cacheMu.Lock()
	if len(cache) >= maxCacheSize {
		cache = make(map[string]*cachedEntry, 64)
	}
	cache[hash] = entry
	cacheMu.Unlock()

	return entry, nil
}

// Specializing the trusted routing group before optimization excludes other
// groups' cache/audio categories from this group's token normalization.
type groupPatcher struct{ group string }

func (p groupPatcher) Visit(node *ast.Node) {
	if identifier, ok := (*node).(*ast.IdentifierNode); ok && identifier.Value == "group" {
		ast.Patch(node, &ast.StringNode{Value: p.group})
	}
	if binary, ok := (*node).(*ast.BinaryNode); ok && (binary.Operator == "&&" || binary.Operator == "||" || binary.Operator == "and" || binary.Operator == "or") {
		for _, pair := range [][2]ast.Node{{binary.Left, binary.Right}, {binary.Right, binary.Left}} {
			if literal, ok := pair[0].(*ast.BoolNode); ok {
				and := binary.Operator == "&&" || binary.Operator == "and"
				if literal.Value == and {
					ast.Patch(node, pair[1])
				} else {
					ast.Patch(node, literal)
				}
				break
			}
		}
	}
	if conditional, ok := (*node).(*ast.ConditionalNode); ok {
		if condition, ok := conditional.Cond.(*ast.BoolNode); ok {
			branch := conditional.Exp2
			if condition.Value {
				branch = conditional.Exp1
			}
			ast.Patch(node, branch)
		}
		return
	}
	if ast.Find(*node, func(part ast.Node) bool {
		_, identifier := part.(*ast.IdentifierNode)
		return identifier
	}) != nil {
		return
	}
	program, err := expr.Compile((*node).String(), expr.AsBool())
	if err != nil {
		return
	}
	value, err := expr.Run(program, nil)
	if err != nil {
		return
	}
	if matches, ok := value.(bool); ok {
		ast.Patch(node, &ast.BoolNode{Value: matches})
	}
}

func compileEntryForGroup(expression, hash, group string) (*cachedEntry, error) {
	entry, err := compileEntryFromCacheByHash(expression, hash)
	if err != nil || !entry.usedVars["group"] {
		return entry, err
	}
	return compileEntryFromCacheByHash(expression, hash, group)
}

// UsedVarsForGroup reports billable categories after resolving group branches.
func UsedVarsForGroup(expression, hash, group string) map[string]bool {
	entry, err := compileEntryForGroup(expression, hash, group)
	if err != nil {
		return nil
	}
	return entry.usedVars
}

// ReferencedGroups supplies literal group boundaries for save-time validation.
func ReferencedGroups(expression string) []string {
	_, body := ParseExprVersion(expression)
	tree, err := parser.Parse(body)
	if err != nil {
		return nil
	}
	groups := []string{""}
	seen := map[string]bool{"": true}
	ast.Find(tree.Node, func(node ast.Node) bool {
		comparison, ok := node.(*ast.BinaryNode)
		if !ok {
			return false
		}
		for _, pair := range [][2]ast.Node{{comparison.Left, comparison.Right}, {comparison.Right, comparison.Left}} {
			id, ok := pair[0].(*ast.IdentifierNode)
			literal, literalOK := pair[1].(*ast.StringNode)
			if ok && id.Value == "group" && literalOK && !seen[literal.Value] {
				groups = append(groups, literal.Value)
				seen[literal.Value] = true
			}
		}
		return false
	})
	fallback := "__billing_fallback__"
	for seen[fallback] {
		fallback += "_"
	}
	groups = append(groups, fallback)
	return groups
}

// ExprVersion returns the version of a cached expression. Returns DefaultExprVersion
// if the expression hasn't been compiled yet or is empty.
func ExprVersion(exprStr string) int {
	if exprStr == "" {
		return DefaultExprVersion
	}
	hash := ExprHashString(exprStr)
	cacheMu.RLock()
	if entry, ok := cache[hash]; ok {
		cacheMu.RUnlock()
		return entry.version
	}
	cacheMu.RUnlock()
	v, _ := ParseExprVersion(exprStr)
	return v
}

func extractUsedVars(prog *vm.Program) map[string]bool {
	vars := make(map[string]bool)
	node := prog.Node()
	ast.Find(node, func(n ast.Node) bool {
		if id, ok := n.(*ast.IdentifierNode); ok {
			switch id.Value {
			case requestRuleTraceFunction, requestRuleTraceIntFunction:
				return false
			}
			vars[id.Value] = true
		}
		return false
	})
	return vars
}

func extractUsedUsageKeys(prog *vm.Program) map[string]bool {
	keys := make(map[string]bool)
	ast.Find(prog.Node(), func(node ast.Node) bool {
		call, ok := node.(*ast.CallNode)
		if !ok || len(call.Arguments) != 1 {
			return false
		}
		callee, ok := call.Callee.(*ast.IdentifierNode)
		if !ok || callee.Value != "u" {
			return false
		}
		literal, ok := call.Arguments[0].(*ast.StringNode)
		if !ok {
			return false
		}
		keys[strings.TrimSpace(literal.Value)] = true
		return false
	})
	return keys
}

// UsedVars returns the set of identifier names referenced by an expression.
// The result is cached alongside the compiled program. Returns nil for empty input.
func UsedVars(exprStr string) map[string]bool {
	return UsedVarsByHash(exprStr, ExprHashString(exprStr))
}

// UsedVarsByHash reuses the digest captured by the host's billing snapshot.
func UsedVarsByHash(exprStr, hash string) map[string]bool {
	if exprStr == "" {
		return nil
	}
	cacheMu.RLock()
	if entry, ok := cache[hash]; ok {
		cacheMu.RUnlock()
		return entry.usedVars
	}
	cacheMu.RUnlock()

	// Compile (and cache) to populate usedVars
	if _, err := compileFromCacheByHash(exprStr, hash); err != nil {
		return nil
	}
	cacheMu.RLock()
	entry, ok := cache[hash]
	cacheMu.RUnlock()
	if ok {
		return entry.usedVars
	}
	return nil
}

// UsedUsageKeys returns literal keys referenced by u("...") calls. Calls with
// dynamic arguments are intentionally omitted because they cannot be
// validated statically.
func UsedUsageKeys(exprStr string) map[string]bool {
	if exprStr == "" {
		return nil
	}
	hash := ExprHashString(exprStr)
	cacheMu.RLock()
	if entry, ok := cache[hash]; ok {
		cacheMu.RUnlock()
		return entry.usedUsageKeys
	}
	cacheMu.RUnlock()

	if _, err := compileFromCacheByHash(exprStr, hash); err != nil {
		return nil
	}
	cacheMu.RLock()
	entry, ok := cache[hash]
	cacheMu.RUnlock()
	if ok {
		return entry.usedUsageKeys
	}
	return nil
}

// InvalidateCache clears the compiled-expression cache.
// Called when billing rules are updated.
func InvalidateCache() {
	cacheMu.Lock()
	cache = make(map[string]*cachedEntry, 64)
	cacheMu.Unlock()
}
