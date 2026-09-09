package studio

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
)

const maxBSLEditorItems = 200_000

// BSLPosition is a one-based source position understood by ML Studio.
type BSLPosition struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// BSLRange is a half-open source range.
type BSLRange struct {
	Start BSLPosition `json:"start"`
	End   BSLPosition `json:"end"`
}

// BSLHighlight assigns an editor style to one non-whitespace source range.
type BSLHighlight struct {
	Kind  string   `json:"kind"`
	Range BSLRange `json:"range"`
}

// BSLDiagnostic is a bounded syntax problem displayed by Studio.
type BSLDiagnostic struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Range   BSLRange `json:"range"`
}

// BSLSymbol is one navigable module structure item.
type BSLSymbol struct {
	Kind   string   `json:"kind"`
	Name   string   `json:"name"`
	Detail string   `json:"detail,omitempty"`
	Range  BSLRange `json:"range"`
}

// BSLPair connects matching brackets or language block boundaries.
type BSLPair struct {
	Kind  string   `json:"kind"`
	Open  BSLRange `json:"open"`
	Close BSLRange `json:"close"`
}

// BSLFold identifies a collapsible multi-line block.
type BSLFold struct {
	Kind      string `json:"kind"`
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
	Label     string `json:"label"`
}

// BSLAnalysis contains all lightweight features needed by the first BSL editor.
type BSLAnalysis struct {
	Highlights  []BSLHighlight  `json:"highlights"`
	Diagnostics []BSLDiagnostic `json:"diagnostics"`
	Symbols     []BSLSymbol     `json:"symbols"`
	Pairs       []BSLPair       `json:"pairs"`
	Folds       []BSLFold       `json:"folds"`
	Truncated   bool            `json:"truncated,omitempty"`
}

// AnalyzeBSL performs bounded, side-effect-free analysis of an unsaved module.
func AnalyzeBSL(filename, source string) (BSLAnalysis, error) {
	if filename == "" || !strings.HasSuffix(strings.ToLower(filename), ".bsl") {
		return BSLAnalysis{}, fmt.Errorf("BSL analysis requires a .bsl source path")
	}
	if len(source) > MaxEditableFileBytes || !utf8.ValidString(source) || strings.IndexByte(source, 0) >= 0 {
		return BSLAnalysis{}, fmt.Errorf("editable source must be valid UTF-8 and at most %d bytes", MaxEditableFileBytes)
	}

	module, tokens, diagnostics := syntax.ParseWithTokens(filename, source)
	analysis := BSLAnalysis{
		Highlights:  make([]BSLHighlight, 0, min(len(tokens), maxBSLEditorItems)),
		Diagnostics: make([]BSLDiagnostic, 0, min(len(diagnostics), 256)),
		Symbols:     make([]BSLSymbol, 0, min(len(module.Variables)+len(module.Routines), maxBSLEditorItems)),
		Pairs:       make([]BSLPair, 0, min(len(tokens)/8, maxBSLEditorItems)),
		Folds:       make([]BSLFold, 0, min(len(module.Routines)*2, maxBSLEditorItems)),
	}
	analysis.addHighlights(tokens)
	for _, diagnostic := range diagnostics {
		if len(analysis.Diagnostics) == maxBSLEditorItems {
			analysis.Truncated = true
			break
		}
		analysis.Diagnostics = append(analysis.Diagnostics, BSLDiagnostic{
			Code: diagnostic.Code, Message: diagnostic.Message, Range: editorRange(diagnostic.Span),
		})
	}
	analysis.addSymbols(module)
	analysis.addPairsAndFolds(tokens)
	return analysis, nil
}

func (analysis *BSLAnalysis) addHighlights(tokens []syntax.Token) {
	directiveLine := 0
	for _, token := range tokens {
		for _, trivia := range token.LeadingTrivia {
			switch trivia.Kind {
			case syntax.WhitespaceTrivia:
				analysis.addIndentGuides(trivia)
			case syntax.LineCommentTrivia:
				analysis.addHighlight("comment", trivia.Span)
			}
		}
		kind := highlightKind(token.Kind)
		if token.Kind == syntax.Ampersand || token.Kind == syntax.Hash {
			directiveLine = token.Span.Start.Line
		} else if token.Span.Start.Line == directiveLine && token.Kind == syntax.Identifier {
			kind = "directive"
		}
		if kind != "" && token.Kind != syntax.EOF {
			analysis.addHighlight(kind, token.Span)
		}
	}
}

func (analysis *BSLAnalysis) addIndentGuides(trivia syntax.Trivia) {
	line, column := trivia.Span.Start.Line, trivia.Span.Start.Column
	atLineStart := column == 1
	for offset := 0; offset < len(trivia.Lexeme); {
		current := trivia.Lexeme[offset]
		switch current {
		case '\r':
			offset++
			if offset < len(trivia.Lexeme) && trivia.Lexeme[offset] == '\n' {
				offset++
			}
			line, column, atLineStart = line+1, 1, true
		case '\n':
			offset++
			line, column, atLineStart = line+1, 1, true
		case '\t':
			if atLineStart {
				analysis.addHighlight("indent", syntax.Span{
					Start: syntax.Position{Line: line, Column: column}, End: syntax.Position{Line: line, Column: column + 1},
				})
			}
			offset++
			column++
		default:
			if current == ' ' && atLineStart {
				if (column-1)%4 == 0 {
					analysis.addHighlight("indent", syntax.Span{
						Start: syntax.Position{Line: line, Column: column}, End: syntax.Position{Line: line, Column: column + 1},
					})
				}
				offset++
				column++
				continue
			}
			_, width := utf8.DecodeRuneInString(trivia.Lexeme[offset:])
			offset += width
			column++
			atLineStart = false
		}
	}
}

func (analysis *BSLAnalysis) addHighlight(kind string, span syntax.Span) {
	if len(analysis.Highlights) == maxBSLEditorItems {
		analysis.Truncated = true
		return
	}
	analysis.Highlights = append(analysis.Highlights, BSLHighlight{Kind: kind, Range: editorRange(span)})
}

func highlightKind(kind syntax.Kind) string {
	switch {
	case kind.IsKeyword():
		switch kind {
		case syntax.True, syntax.False, syntax.Undefined, syntax.Null:
			return "constant"
		default:
			return "keyword"
		}
	case kind == syntax.Identifier:
		return "identifier"
	case kind == syntax.Number:
		return "number"
	case kind == syntax.String || kind == syntax.StringStart || kind == syntax.StringPart || kind == syntax.StringEnd || kind == syntax.Date:
		return "string"
	case kind == syntax.Ampersand || kind == syntax.Hash:
		return "directive"
	case kind != syntax.EOF:
		return "operator"
	default:
		return ""
	}
}

func (analysis *BSLAnalysis) addSymbols(module *syntax.Module) {
	for _, variable := range module.Variables {
		if len(analysis.Symbols) == maxBSLEditorItems {
			analysis.Truncated = true
			return
		}
		analysis.Symbols = append(analysis.Symbols, BSLSymbol{
			Kind: "variable", Name: variable.Name, Detail: exportDetail(variable.Export), Range: editorRange(variable.SourceSpan),
		})
	}
	for _, routine := range module.Routines {
		if len(analysis.Symbols) == maxBSLEditorItems {
			analysis.Truncated = true
			return
		}
		parameters := make([]string, 0, len(routine.Parameters))
		for _, parameter := range routine.Parameters {
			parameters = append(parameters, parameter.Name)
		}
		kind := "procedure"
		if routine.Function {
			kind = "function"
		}
		detail := "(" + strings.Join(parameters, ", ") + ")"
		if routine.Export {
			detail += " · Экспорт"
		}
		analysis.Symbols = append(analysis.Symbols, BSLSymbol{
			Kind: kind, Name: routine.Name, Detail: detail, Range: editorRange(routine.SourceSpan),
		})
	}
	sort.SliceStable(analysis.Symbols, func(left, right int) bool {
		return positionBefore(analysis.Symbols[left].Range.Start, analysis.Symbols[right].Range.Start)
	})
}

func exportDetail(exported bool) string {
	if exported {
		return "Экспорт"
	}
	return ""
}

type blockStart struct {
	kind     string
	expected syntax.Kind
	token    syntax.Token
}

func (analysis *BSLAnalysis) addPairsAndFolds(tokens []syntax.Token) {
	blocks := make([]blockStart, 0, 16)
	delimiters := make([]blockStart, 0, 16)
	regions := make([]syntax.Token, 0, 8)
	preprocessorLine := 0
	for index, token := range tokens {
		if token.Kind == syntax.EOF {
			break
		}
		if token.Kind == syntax.Hash {
			preprocessorLine = token.Span.Start.Line
			if index+1 < len(tokens) && tokens[index+1].Span.Start.Line == preprocessorLine {
				directive := strings.ToLower(tokens[index+1].Lexeme)
				switch directive {
				case "область", "region":
					regions = append(regions, token)
				case "конецобласти", "endregion":
					if len(regions) != 0 {
						open := regions[len(regions)-1]
						regions = regions[:len(regions)-1]
						analysis.addPairAndFold("region", open, token, "Область")
					}
				}
			}
			continue
		}
		if token.Span.Start.Line == preprocessorLine {
			continue
		}
		switch token.Kind {
		case syntax.LeftParen:
			delimiters = append(delimiters, blockStart{kind: "parentheses", expected: syntax.RightParen, token: token})
		case syntax.LeftBracket:
			delimiters = append(delimiters, blockStart{kind: "brackets", expected: syntax.RightBracket, token: token})
		case syntax.RightParen, syntax.RightBracket:
			if len(delimiters) != 0 && delimiters[len(delimiters)-1].expected == token.Kind {
				open := delimiters[len(delimiters)-1]
				delimiters = delimiters[:len(delimiters)-1]
				analysis.addPair(open.kind, open.token, token)
			}
		case syntax.Procedure:
			blocks = append(blocks, blockStart{kind: "procedure", expected: syntax.EndProcedure, token: token})
		case syntax.Function:
			blocks = append(blocks, blockStart{kind: "function", expected: syntax.EndFunction, token: token})
		case syntax.If:
			blocks = append(blocks, blockStart{kind: "if", expected: syntax.EndIf, token: token})
		case syntax.For, syntax.While:
			blocks = append(blocks, blockStart{kind: "loop", expected: syntax.EndDo, token: token})
		case syntax.Try:
			blocks = append(blocks, blockStart{kind: "try", expected: syntax.EndTry, token: token})
		case syntax.EndProcedure, syntax.EndFunction, syntax.EndIf, syntax.EndDo, syntax.EndTry:
			if len(blocks) != 0 && blocks[len(blocks)-1].expected == token.Kind {
				open := blocks[len(blocks)-1]
				blocks = blocks[:len(blocks)-1]
				analysis.addPairAndFold(open.kind, open.token, token, foldLabel(open.kind))
			}
		}
	}
	sort.SliceStable(analysis.Pairs, func(left, right int) bool {
		return positionBefore(analysis.Pairs[left].Open.Start, analysis.Pairs[right].Open.Start)
	})
	sort.SliceStable(analysis.Folds, func(left, right int) bool {
		if analysis.Folds[left].StartLine == analysis.Folds[right].StartLine {
			return analysis.Folds[left].EndLine > analysis.Folds[right].EndLine
		}
		return analysis.Folds[left].StartLine < analysis.Folds[right].StartLine
	})
}

func (analysis *BSLAnalysis) addPair(kind string, open, close syntax.Token) {
	if len(analysis.Pairs) == maxBSLEditorItems {
		analysis.Truncated = true
		return
	}
	analysis.Pairs = append(analysis.Pairs, BSLPair{Kind: kind, Open: editorRange(open.Span), Close: editorRange(close.Span)})
}

func (analysis *BSLAnalysis) addPairAndFold(kind string, open, close syntax.Token, label string) {
	analysis.addPair(kind, open, close)
	if close.Span.Start.Line <= open.Span.Start.Line+1 || len(analysis.Folds) == maxBSLEditorItems {
		if len(analysis.Folds) == maxBSLEditorItems {
			analysis.Truncated = true
		}
		return
	}
	analysis.Folds = append(analysis.Folds, BSLFold{
		Kind: kind, StartLine: open.Span.Start.Line, EndLine: close.Span.Start.Line, Label: label,
	})
}

func foldLabel(kind string) string {
	switch kind {
	case "procedure":
		return "Процедура"
	case "function":
		return "Функция"
	case "if":
		return "Если"
	case "loop":
		return "Цикл"
	case "try":
		return "Попытка"
	default:
		return kind
	}
}

func editorRange(span syntax.Span) BSLRange {
	return BSLRange{Start: editorPosition(span.Start), End: editorPosition(span.End)}
}

func editorPosition(position syntax.Position) BSLPosition {
	line, column := position.Line, position.Column
	if line < 1 {
		line = 1
	}
	if column < 1 {
		column = 1
	}
	return BSLPosition{Line: line, Column: column}
}

func positionBefore(left, right BSLPosition) bool {
	return left.Line < right.Line || left.Line == right.Line && left.Column < right.Column
}
