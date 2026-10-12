package metadata

import (
	"context"
	"fmt"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/kinds"
)

type bslEventCall struct {
	handler any
	parent  *bslEventCall
}

type bslEventCallKey struct{}

func enterBSLEvent(ctx context.Context, handler any) (context.Context, error) {
	active, _ := ctx.Value(bslEventCallKey{}).(*bslEventCall)
	for call := active; call != nil; call = call.parent {
		if call.handler == handler {
			return nil, fmt.Errorf("recursive metadata event call is not allowed")
		}
	}
	return context.WithValue(ctx, bslEventCallKey{}, &bslEventCall{handler: handler, parent: active}), nil
}

// compileObjectModule compiles one module of an object on its own, under the
// name and with the words it has in the whole program (ModuleNamer).
func compileObjectModule(kind Kind, object string, module kinds.Module, filename, source string) (*bytecode.Program, []syntax.Diagnostic) {
	return compiler.CompileModules([]compiler.ModuleSource{{
		Name: ObjectModuleName(kind, object, string(module)), Filename: filename, Source: source,
		PredefinedVariables: moduleWords(kind, module),
	}})
}

// boundWord is a word of a module and the value it holds for one call.
type boundWord struct {
	words kinds.Words
	value bytecode.Value
}

// bindModuleWords gives the words of a module their values for one call of
// an event, in both languages, and returns what takes them back. A failure
// half way takes back what was already given, so a module never keeps the
// object of a call that did not happen.
func bindModuleWords(context *vm.Context, module string, words ...boundWord) (func(), error) {
	var bound []string
	unbind := func() {
		for _, name := range bound {
			_ = context.SetModuleVariable(module, name, bytecode.Undefined())
		}
	}
	for _, word := range words {
		for _, name := range []string{word.words.Russian, word.words.English} {
			if err := context.SetModuleVariable(module, name, word.value); err != nil {
				unbind()
				return nil, err
			}
			bound = append(bound, name)
		}
	}
	return unbind, nil
}
