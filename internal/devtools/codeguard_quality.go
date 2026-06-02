package devtools

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type parsedCodeGuardFile struct {
	Path string
	File *ast.File
	Fset *token.FileSet
	Src  []byte
}

type duplicateFunction struct {
	Path string
	Name string
	Line int
}

func (r Runner) runCodeGuardDrySection(targets []string) codeGuardSectionResult {
	result := codeGuardSectionResult{Name: "DRY", Status: codeGuardStatusPass}
	if len(targets) == 0 {
		result.Status = codeGuardStatusSkip
		result.Note = "no changed non-test Go files"
		return result
	}

	thresholds := loadCodeGuardThresholds()
	parsedFiles, violations := r.parseCodeGuardFiles(targets)
	if len(violations) > 0 {
		result.Status = codeGuardStatusFail
		result.Note = "could not parse changed Go files"
		result.Violations = violations
		return result
	}

	duplicates := make(map[string][]duplicateFunction)
	for _, parsed := range parsedFiles {
		for _, decl := range parsed.File.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			start := parsed.Fset.Position(fn.Pos()).Line
			end := parsed.Fset.Position(fn.End()).Line
			if end-start+1 < thresholds.DryMinFunctionLines {
				continue
			}

			key := normalizedFunctionBody(fn.Body)
			if key == "" {
				continue
			}
			duplicates[key] = append(duplicates[key], duplicateFunction{
				Path: parsed.Path,
				Name: fn.Name.Name,
				Line: start,
			})
		}
	}

	for _, group := range duplicates {
		if len(group) < 2 {
			continue
		}
		for i, item := range group {
			other := group[(i+1)%len(group)]
			result.Violations = append(result.Violations, codeGuardViolation{
				Path: item.Path,
				Message: "function " + item.Name + " duplicates " + other.Name +
					" (" + other.Path + ":" + itoa(other.Line) + ")",
			})
		}
	}

	return finalizeCodeGuardQualitySection(result, "CODE_GUARD_DRY_BLOCKING", false, "duplicate function bodies")
}

func (r Runner) runCodeGuardCleanCodeSection(targets []string) codeGuardSectionResult {
	result := codeGuardSectionResult{Name: "Clean Code", Status: codeGuardStatusPass}
	if len(targets) == 0 {
		result.Status = codeGuardStatusSkip
		result.Note = "no changed non-test Go files"
		return result
	}

	thresholds := loadCodeGuardThresholds()
	parsedFiles, violations := r.parseCodeGuardFiles(targets)
	if len(violations) > 0 {
		result.Status = codeGuardStatusFail
		result.Note = "could not parse changed Go files"
		result.Violations = violations
		return result
	}

	for _, parsed := range parsedFiles {
		for _, decl := range parsed.File.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			name := fn.Name.Name
			start := parsed.Fset.Position(fn.Pos()).Line
			end := parsed.Fset.Position(fn.End()).Line
			lines := end - start + 1
			if lines > thresholds.MaxFunctionLines {
				result.Violations = append(result.Violations, codeGuardViolation{
					Path: parsed.Path,
					Message: "function " + name + " (line " + itoa(start) + ") is " + itoa(lines) +
						" lines (>" + itoa(thresholds.MaxFunctionLines) + ")",
				})
			}

			params := countFunctionParams(fn.Type)
			if params > thresholds.MaxFunctionParams {
				result.Violations = append(result.Violations, codeGuardViolation{
					Path: parsed.Path,
					Message: "function " + name + " (line " + itoa(start) + ") takes " + itoa(params) +
						" params (>" + itoa(thresholds.MaxFunctionParams) + ")",
				})
			}

			depth := maxStmtNesting(fn.Body.List, 0)
			if depth > thresholds.MaxNestingDepth {
				result.Violations = append(result.Violations, codeGuardViolation{
					Path: parsed.Path,
					Message: "function " + name + " (line " + itoa(start) + ") nests " + itoa(depth) +
						" levels deep (>" + itoa(thresholds.MaxNestingDepth) + ")",
				})
			}
		}
	}

	return finalizeCodeGuardQualitySection(result, "CODE_GUARD_CLEAN_CODE_BLOCKING", false, "clean-code heuristics")
}

func (r Runner) runCodeGuardPrinciplesSection(targets []string) codeGuardSectionResult {
	result := codeGuardSectionResult{Name: "Design Principles (SOLID/SoC)", Status: codeGuardStatusPass}
	if len(targets) == 0 {
		result.Status = codeGuardStatusSkip
		result.Note = "no changed non-test Go files"
		return result
	}

	thresholds := loadCodeGuardThresholds()
	parsedFiles, violations := r.parseCodeGuardFiles(targets)
	if len(violations) > 0 {
		result.Status = codeGuardStatusFail
		result.Note = "could not parse changed Go files"
		result.Violations = violations
		return result
	}

	for _, parsed := range parsedFiles {
		for _, decl := range parsed.File.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			name := fn.Name.Name
			start := parsed.Fset.Position(fn.Pos()).Line

			boolParams := countBoolParams(fn.Type)
			if boolParams > thresholds.MaxBoolParams {
				result.Violations = append(result.Violations, codeGuardViolation{
					Path: parsed.Path,
					Message: "function " + name + " (line " + itoa(start) + ") has " + itoa(boolParams) +
						" bool params (>" + itoa(thresholds.MaxBoolParams) + ")",
				})
			}

			returns := countReturns(fn.Body)
			if returns > thresholds.MaxFunctionReturns {
				result.Violations = append(result.Violations, codeGuardViolation{
					Path: parsed.Path,
					Message: "function " + name + " (line " + itoa(start) + ") has " + itoa(returns) +
						" return points (>" + itoa(thresholds.MaxFunctionReturns) + ")",
				})
			}

			chain := maxSelectorChain(fn.Body)
			if chain > thresholds.MaxDemeterChain {
				result.Violations = append(result.Violations, codeGuardViolation{
					Path: parsed.Path,
					Message: "function " + name + " (line " + itoa(start) + ") reaches selector chain depth " +
						itoa(chain) + " (>" + itoa(thresholds.MaxDemeterChain) + ")",
				})
			}
		}
	}

	return finalizeCodeGuardQualitySection(result, "CODE_GUARD_PRINCIPLES_BLOCKING", false, "SOLID/SoC heuristics")
}

func finalizeCodeGuardQualitySection(result codeGuardSectionResult, envName string, defaultBlocking bool, note string) codeGuardSectionResult {
	if len(result.Violations) == 0 {
		result.Note = "No findings."
		return result
	}
	sortCodeGuardViolations(result.Violations)
	result.Note = note
	if codeGuardSectionBlocking(envName, defaultBlocking) {
		result.Status = codeGuardStatusFail
	} else {
		result.Status = codeGuardStatusWarn
	}
	return result
}

func (r Runner) parseCodeGuardFiles(targets []string) ([]parsedCodeGuardFile, []codeGuardViolation) {
	parsedFiles := make([]parsedCodeGuardFile, 0, len(targets))
	var violations []codeGuardViolation
	for _, target := range targets {
		path := filepath.Join(r.WorkDir, filepath.FromSlash(target))
		src, err := os.ReadFile(path)
		if err != nil {
			violations = append(violations, codeGuardViolation{Path: target, Message: err.Error()})
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			violations = append(violations, codeGuardViolation{Path: target, Message: err.Error()})
			continue
		}
		parsedFiles = append(parsedFiles, parsedCodeGuardFile{
			Path: target,
			File: file,
			Fset: fset,
			Src:  src,
		})
	}
	return parsedFiles, violations
}

func normalizedFunctionBody(body *ast.BlockStmt) string {
	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), body); err != nil {
		return ""
	}
	return strings.Join(strings.Fields(buf.String()), " ")
}

func countFunctionParams(fnType *ast.FuncType) int {
	if fnType == nil || fnType.Params == nil {
		return 0
	}
	count := 0
	for _, field := range fnType.Params.List {
		names := len(field.Names)
		if names == 0 {
			names = 1
		}
		count += names
	}
	return count
}

func countBoolParams(fnType *ast.FuncType) int {
	if fnType == nil || fnType.Params == nil {
		return 0
	}
	count := 0
	for _, field := range fnType.Params.List {
		if ident, ok := field.Type.(*ast.Ident); !ok || ident.Name != "bool" {
			continue
		}
		names := len(field.Names)
		if names == 0 {
			names = 1
		}
		count += names
	}
	return count
}

func countReturns(node ast.Node) int {
	count := 0
	ast.Inspect(node, func(n ast.Node) bool {
		_, ok := n.(*ast.ReturnStmt)
		if ok {
			count++
		}
		return true
	})
	return count
}

func maxSelectorChain(node ast.Node) int {
	maxDepth := 0
	ast.Inspect(node, func(n ast.Node) bool {
		if expr, ok := n.(*ast.SelectorExpr); ok {
			if depth := selectorDepth(expr); depth > maxDepth {
				maxDepth = depth
			}
		}
		return true
	})
	return maxDepth
}

func selectorDepth(expr ast.Expr) int {
	switch node := expr.(type) {
	case *ast.SelectorExpr:
		return 1 + selectorDepth(node.X)
	default:
		return 0
	}
}

func maxStmtNesting(stmts []ast.Stmt, depth int) int {
	best := depth
	for _, stmt := range stmts {
		if nested := nestedStmtDepth(stmt, depth); nested > best {
			best = nested
		}
	}
	return best
}

func nestedStmtDepth(stmt ast.Stmt, depth int) int {
	switch node := stmt.(type) {
	case *ast.IfStmt:
		best := maxStmtNesting(node.Body.List, depth+1)
		if node.Else != nil {
			if elseDepth := nestedStmtDepthFromElse(node.Else, depth+1); elseDepth > best {
				best = elseDepth
			}
		}
		return best
	case *ast.ForStmt:
		return maxStmtNesting(node.Body.List, depth+1)
	case *ast.RangeStmt:
		return maxStmtNesting(node.Body.List, depth+1)
	case *ast.SwitchStmt:
		return maxCaseClauseDepth(node.Body.List, depth+1)
	case *ast.TypeSwitchStmt:
		return maxCaseClauseDepth(node.Body.List, depth+1)
	case *ast.SelectStmt:
		return maxCommClauseDepth(node.Body.List, depth+1)
	default:
		return depth
	}
}

func nestedStmtDepthFromElse(stmt ast.Stmt, depth int) int {
	switch node := stmt.(type) {
	case *ast.BlockStmt:
		return maxStmtNesting(node.List, depth)
	default:
		return nestedStmtDepth(node, depth)
	}
}

func maxCaseClauseDepth(list []ast.Stmt, depth int) int {
	best := depth
	for _, stmt := range list {
		clause, ok := stmt.(*ast.CaseClause)
		if !ok {
			continue
		}
		if nested := maxStmtNesting(clause.Body, depth); nested > best {
			best = nested
		}
	}
	return best
}

func maxCommClauseDepth(list []ast.Stmt, depth int) int {
	best := depth
	for _, stmt := range list {
		clause, ok := stmt.(*ast.CommClause)
		if !ok {
			continue
		}
		if nested := maxStmtNesting(clause.Body, depth); nested > best {
			best = nested
		}
	}
	return best
}

func itoa(value int) string {
	return strconv.Itoa(value)
}
