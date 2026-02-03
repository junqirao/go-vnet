package middleware

import (
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/junqirao/gocomponents/response"
)

func Response(r *ghttp.Request) {
	r.Middleware.Next()

	var (
		err  = r.GetError()
		ec   = response.CodeFromError(err)
		data = r.GetHandlerResponse()
	)
	if ec == nil {
		ec = response.DefaultSuccess()
	}

	response.WriteData(r, ec, data)
}
