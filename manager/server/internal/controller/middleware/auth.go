package middleware

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/crypto/gmd5"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/junqirao/gocomponents/response"
)

const (
	salt = "go_v_net_web_server"
)

func CheckSignature(r *ghttp.Request) {
	var (
		appid     = r.GetHeader("X-Appid")
		timestamp = r.GetHeader("X-Timestamp")
		signature = r.GetHeader("X-Signature")
	)

	if appid == "" || timestamp == "" || signature == "" {
		response.Error(r, response.CodeUnauthorized, "missing required header")
		return
	}

	diff := g.Cfg().MustGet(r.Context(), "auth.allowed_timestamp_diff").Int64()
	if diff > 0 {
		if time.Now().Unix()-gconv.Int64(timestamp) > diff {
			response.Error(r, response.CodeUnauthorized, "invalid timestamp")
			return
		}
	}

	sign, err := Sign(r.Context(), appid, timestamp)
	if err != nil {
		response.Error(r, response.CodeUnauthorized, err.Error())
		return
	}

	if sign != signature {
		response.Error(r, response.CodeUnauthorized, "signature mismatch")
		return
	}

	r.Middleware.Next()
}

func Sign(ctx context.Context, appid, timestamp string) (v string, err error) {
	var (
		apps   = g.Cfg().MustGet(ctx, "auth.apps").Maps()
		secret string
	)

	for _, app := range apps {
		if app["appid"] == appid {
			secret = gconv.String(app["secret"])
			break
		}
	}

	if secret == "" {
		err = errors.New("appid not found")
		return
	}

	v = gmd5.MustEncryptString(fmt.Sprintf("%s%s%s%s", salt, appid, timestamp, secret))
	return
}
