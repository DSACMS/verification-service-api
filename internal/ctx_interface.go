package internal

import (
	"context"
)

type Ctx[C context.Context] interface {
	Locals(key any, value ...any) any
	Context() C
	UserContext() context.Context
	SetUserContext(ctx context.Context)
}

type EmptyCtx struct {
	locals  map[any]any
	ctx     context.Context
	userCtx context.Context
}

func (c *EmptyCtx) Locals(key any, value ...any) any {
	if len(value) == 0 {
		return c.locals[key]
	}
	c.locals[key] = value[0]
	return value[0]
}

func (c *EmptyCtx) Context() context.Context {
	return c.ctx
}

func (c *EmptyCtx) UserContext() context.Context {
	return c.userCtx
}

func (c *EmptyCtx) SetUserContext(ctx context.Context) {
	c.userCtx = ctx
}

func TODO() Ctx[context.Context] {
	return &EmptyCtx{
		locals:  make(map[any]any),
		ctx:     context.TODO(),
		userCtx: context.TODO(),
	}
}
