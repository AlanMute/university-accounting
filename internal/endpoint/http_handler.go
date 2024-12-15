package endpoint

import (
	"github.com/AlanMute/university-accounting/pkg/cast"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

func init() {
	for path, info := range routingMap {
		info.path = path
		routingMap[path] = info
	}
}

type route struct {
	handler func(ctx *fasthttp.RequestCtx, h *HttpHandler)
	path    string
}

var routingMap = map[string]route{
	"/status": {handler: func(ctx *fasthttp.RequestCtx, h *HttpHandler) {
		_, _ = ctx.WriteString("OK")
	}},
}

type HttpHandler struct {
	metricsHandler fasthttp.RequestHandler

	swaggerHandler fasthttp.RequestHandler
	fsHandler      fasthttp.RequestHandler
}

func NewHttpHandlerChips() *HttpHandler {
	h := &HttpHandler{}

	return h
}

func (h *HttpHandler) Handle(ctx *fasthttp.RequestCtx) {
	defer func() {
		err := recover()
		if err != nil {
			logrus.Error(err)
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		}
	}()

	if r, ok := routingMap[cast.ByteArrayToString(ctx.Path())]; ok {
		r.handler(ctx, h)
	} else {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
	}
}
