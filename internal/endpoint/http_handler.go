package endpoint

import (
	"github.com/AlanMute/university-accounting/internal/accounting"
	"github.com/AlanMute/university-accounting/pkg/cast"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"time"
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

	"/api/v1/attendance-report": {handler: func(ctx *fasthttp.RequestCtx, h *HttpHandler) { //Lab1
		if cast.ByteArrayToString(ctx.Method()) == fasthttp.MethodGet {
			h.generateAttendanceReport(ctx)
		} else {
			ctx.SetStatusCode(fasthttp.StatusMethodNotAllowed)
		}
	}},

	"/api/v1/course-report": {handler: func(ctx *fasthttp.RequestCtx, h *HttpHandler) { //Lab2
		if cast.ByteArrayToString(ctx.Method()) == fasthttp.MethodGet {
			h.generateCourseReport(ctx)
		} else {
			ctx.SetStatusCode(fasthttp.StatusMethodNotAllowed)
		}
	}},

	"/api/v1/group-report": {handler: func(ctx *fasthttp.RequestCtx, h *HttpHandler) { //Lab3
		if cast.ByteArrayToString(ctx.Method()) == fasthttp.MethodGet {
			h.generateGroupReport(ctx)
		} else {
			ctx.SetStatusCode(fasthttp.StatusMethodNotAllowed)
		}
	}},

	"/api/v1/groups": {handler: func(ctx *fasthttp.RequestCtx, h *HttpHandler) { //Lab3
		if cast.ByteArrayToString(ctx.Method()) == fasthttp.MethodGet {
			h.getGroups(ctx)
		} else {
			ctx.SetStatusCode(fasthttp.StatusMethodNotAllowed)
		}
	}},
}

type HttpHandler struct {
	accountingClient *accounting.Client
}

func NewHttpHandler(accountingClient *accounting.Client) *HttpHandler {
	h := &HttpHandler{
		accountingClient: accountingClient,
	}

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

func (h *HttpHandler) generateAttendanceReport(ctx *fasthttp.RequestCtx) {
	termByte := ctx.QueryArgs().Peek("term")
	if termByte == nil {
		writeError(ctx, "term", fasthttp.StatusBadRequest)
		return
	}
	term := cast.ByteArrayToString(termByte)

	startDateByte := ctx.QueryArgs().Peek("startDate")
	if startDateByte == nil {
		writeError(ctx, "startDateByte", fasthttp.StatusBadRequest)
		return
	}
	startDate := cast.ByteArrayToString(startDateByte)
	if !isValidDate(startDate) {
		writeError(ctx, "'startDate' must be in the format YYYY-MM-DD", fasthttp.StatusBadRequest)
		return
	}

	endDateByte := ctx.QueryArgs().Peek("endDate")
	if endDateByte == nil {
		writeError(ctx, "endDate", fasthttp.StatusBadRequest)
		return
	}
	endDate := cast.ByteArrayToString(endDateByte)
	if !isValidDate(endDate) {
		writeError(ctx, "'endDate' must be in the format YYYY-MM-DD", fasthttp.StatusBadRequest)
		return
	}

	resp, err := h.accountingClient.GenerateAttendanceReport(term, startDate, endDate)
	if err != nil {
		writeError(ctx, err.Error(), fasthttp.StatusInternalServerError)
		return
	}

	writeObject(ctx, resp, fasthttp.StatusOK)
}

func (h *HttpHandler) generateCourseReport(ctx *fasthttp.RequestCtx) {
	year, err := ctx.QueryArgs().GetUint("year")
	if err != nil {
		writeError(ctx, "year", fasthttp.StatusBadRequest)
		return
	}

	semester, err := ctx.QueryArgs().GetUint("sem")
	if err != nil {
		writeError(ctx, "sem", fasthttp.StatusBadRequest)
		return
	}

	resp, err := h.accountingClient.GenerateCourseReport(year, semester)
	if err != nil {
		writeError(ctx, err.Error(), fasthttp.StatusInternalServerError)
		return
	}

	writeObject(ctx, resp, fasthttp.StatusOK)
}

func (h *HttpHandler) generateGroupReport(ctx *fasthttp.RequestCtx) {
	groupByte := ctx.QueryArgs().Peek("group")
	if groupByte == nil {
		writeError(ctx, "group", fasthttp.StatusBadRequest)
		return
	}
	group := cast.ByteArrayToString(groupByte)

	resp, err := h.accountingClient.GenerateGroupReport(group)
	if err != nil {
		writeError(ctx, err.Error(), fasthttp.StatusInternalServerError)
		return
	}

	writeObject(ctx, resp, fasthttp.StatusOK)
}

func (h *HttpHandler) getGroups(ctx *fasthttp.RequestCtx) {
	resp, err := h.accountingClient.GetAllGroups()
	if err != nil {
		writeError(ctx, err.Error(), fasthttp.StatusInternalServerError)
		return
	}

	writeObject(ctx, resp, fasthttp.StatusOK)
}

func isValidDate(dateStr string) bool {
	_, err := time.Parse("2006-01-02", dateStr)
	return err == nil
}
