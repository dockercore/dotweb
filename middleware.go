package dotweb

import (
	"time"

	"github.com/devfeel/dotweb/framework/convert"
)

const (
	middleware_App    = "app"
	middleware_Group  = "group"
	middleware_Router = "router"
)

type MiddlewareFunc func() Middleware

// middleware execution priority:
// app > group > router

// Middleware middleware interface
type Middleware interface {
	Handle(ctx Context) error
	SetNext(m Middleware)
	Next(ctx Context) error
	Exclude(routers ...string)
	HasExclude() bool
	ExistsExcludeRouter(router string) bool
}

// BaseMiddlware is a shortcut for BaseMiddleware
// Deprecated: 由于该struct命名有误，将在2.0版本弃用，请大家尽快修改自己的middleware
type BaseMiddlware struct {
	BaseMiddleware
}

// BaseMiddleware is the base struct, user defined middleware should extend this
type BaseMiddleware struct {
	next           Middleware
	excludeRouters map[string]struct{}
}

func (bm *BaseMiddleware) SetNext(m Middleware) {
	bm.next = m
}

func (bm *BaseMiddleware) Next(ctx Context) error {
	httpCtx := ctx.(*HttpContext)
	if httpCtx.middlewareStep == "" {
		httpCtx.middlewareStep = middleware_App
	}
	if bm.next == nil {
		if httpCtx.middlewareStep == middleware_App {
			httpCtx.middlewareStep = middleware_Group
			if len(httpCtx.RouterNode().GroupMiddlewares()) > 0 {
				return httpCtx.RouterNode().GroupMiddlewares()[0].Handle(ctx)
			}
		}
		if httpCtx.middlewareStep == middleware_Group {
			httpCtx.middlewareStep = middleware_Router
			if len(httpCtx.RouterNode().Middlewares()) > 0 {
				return httpCtx.RouterNode().Middlewares()[0].Handle(ctx)
			}
		}

		if httpCtx.middlewareStep == middleware_Router {
			return httpCtx.Handler()(ctx)
		}
	} else {
		// check exclude config
		if ctx.RouterNode().Node().hasExcludeMiddleware && bm.next.HasExclude() {
			if bm.next.ExistsExcludeRouter(ctx.RouterNode().Node().fullPath) {
				return bm.next.Next(ctx)
			}
		}
		return bm.next.Handle(ctx)
	}
	return nil
}

// Exclude Exclude this middleware with router
func (bm *BaseMiddleware) Exclude(routers ...string) {
	if bm.excludeRouters == nil {
		bm.excludeRouters = make(map[string]struct{})
	}
	for _, v := range routers {
		bm.excludeRouters[v] = struct{}{}
	}
}

// HasExclude check has set exclude router
func (bm *BaseMiddleware) HasExclude() bool {
	if bm.excludeRouters == nil {
		return false
	}
	if len(bm.excludeRouters) > 0 {
		return true
	} else {
		return false
	}
}

// ExistsExcludeRouter check is exists router in exclude map
func (bm *BaseMiddleware) ExistsExcludeRouter(router string) bool {
	if bm.excludeRouters == nil {
		return false
	}
	_, exists := bm.excludeRouters[router]
	return exists
}

type xMiddleware struct {
	BaseMiddleware
	IsEnd bool
}

func (x *xMiddleware) Handle(ctx Context) error {
	httpCtx := ctx.(*HttpContext)
	if httpCtx.middlewareStep == "" {
		httpCtx.middlewareStep = middleware_App
	}
	if x.IsEnd {
		return httpCtx.Handler()(ctx)
	}
	return x.Next(ctx)
}

type RequestLogMiddleware struct {
	BaseMiddleware
}

func (m *RequestLogMiddleware) Handle(ctx Context) error {
	var timeDuration time.Duration
	var timeTaken uint64
	err := m.Next(ctx)
	if ctx.Items().Exists(ItemKeyHandleDuration) {
		var errParse error
		timeDuration, errParse = time.ParseDuration(ctx.Items().GetString(ItemKeyHandleDuration))
		if errParse != nil {
			timeTaken = 0
		} else {
			timeTaken = uint64(timeDuration / time.Millisecond)
		}
	} else {
		var begin time.Time
		beginVal, exists := ctx.Items().Get(ItemKeyHandleStartTime)
		if !exists {
			begin = time.Now()
		} else {
			begin = beginVal.(time.Time)
		}
		timeTaken = uint64(time.Now().Sub(begin) / time.Millisecond)
	}
	log := ctx.Request().Url() + " " + logContext(ctx, timeTaken)
	ctx.HttpServer().Logger().Debug(log, LogTarget_HttpRequest)
	return err
}

// get default log string
func logContext(ctx Context, timetaken uint64) string {
	var reqbytelen, resbytelen, method, proto, status, userip string
	if ctx != nil {
		reqbytelen = convert.Int642String(ctx.Request().ContentLength)
		resbytelen = convert.Int642String(ctx.Response().Size)
		method = ctx.Request().Method
		proto = ctx.Request().Proto
		status = convert.Int2String(ctx.Response().Status)
		userip = ctx.RemoteIP()
	}

	log := method + " "
	log += userip + " "
	log += proto + " "
	log += status + " "
	log += reqbytelen + " "
	log += resbytelen + " "
	log += convert.UInt642String(timetaken)

	return log
}

type TimeoutHookMiddleware struct {
	BaseMiddleware
	HookHandle      StandardHandle
	TimeoutDuration time.Duration
}

func (m *TimeoutHookMiddleware) Handle(ctx Context) error {
	var begin time.Time
	if m.HookHandle != nil {
		beginVal, exists := ctx.Items().Get(ItemKeyHandleStartTime)
		if !exists {
			begin = time.Now()
		} else {
			begin = beginVal.(time.Time)
		}
	}
	// Do next
	err := m.Next(ctx)
	if m.HookHandle != nil {
		realDuration := time.Now().Sub(begin)
		ctx.Items().Set(ItemKeyHandleDuration, realDuration)
		if realDuration > m.TimeoutDuration {
			m.HookHandle(ctx)
		}
	}
	return err
}
