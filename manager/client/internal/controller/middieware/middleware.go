package middieware

import (
	"net/http"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/text/gstr"
	"github.com/junqirao/gocomponents/response"
)

func WhiteList(ips []string) ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		if !gstr.InArray(ips, r.GetClientIp()) {
			response.Error(r, response.CodeFromHttpStatus(http.StatusForbidden).WithDetail("invalid ip address"))
			return
		}
		r.Middleware.Next()
	}
}
