package metadata

import (
	"context"
	"fmt"
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
