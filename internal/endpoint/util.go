package endpoint

import (
	"encoding/json"
	"github.com/valyala/fasthttp"
)

type errorResponse struct {
	Error string `json:"error"`
}

func writeError(ctx *fasthttp.RequestCtx, message string, status int) {
	response := errorResponse{Error: message}
	raw, err := json.Marshal(&response)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetStatusCode(status)
	ctx.Response.Header.Add(fasthttp.HeaderContentType, "application/json")
	_, _ = ctx.Write(raw)
}

func writeObject(ctx *fasthttp.RequestCtx, obj any, status int) {
	raw, err := json.Marshal(obj)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetStatusCode(status)
	ctx.Response.Header.Add(fasthttp.HeaderContentType, "application/json")
	_, _ = ctx.Write(raw)
}
