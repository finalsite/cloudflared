package streamhandler

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/cloudflare/cloudflared/h2mux"
	"github.com/pkg/errors"
)

const (
	lbProbeUserAgentPrefix = "Mozilla/5.0 (compatible; Cloudflare-Traffic-Manager/1.0; +https://www.cloudflare.com/traffic-manager/;"
)

func FindCfRayHeader(h1 *http.Request) string {
	return h1.Header.Get("Cf-Ray")
}

func IsLBProbeRequest(req *http.Request) bool {
	return strings.HasPrefix(req.UserAgent(), lbProbeUserAgentPrefix)
}

func createRequest(stream *h2mux.MuxedStream, url *url.URL) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, url.String(), h2mux.MuxedStreamReader{MuxedStream: stream})
	if err != nil {
		return nil, errors.Wrap(err, "unexpected error from http.NewRequest")
	}
	err = H2RequestHeadersToH1Request(stream.Headers, req)
	if err != nil {
		return nil, errors.Wrap(err, "invalid request received")
	}
	return req, nil
}

func H2RequestHeadersToH1Request(h2 []h2mux.Header, h1 *http.Request) error {
	for _, header := range h2 {
		switch header.Name {
		case ":method":
			h1.Method = header.Value
		case ":scheme":
		case ":authority":
			// Otherwise the host header will be based on the origin URL
			h1.Host = header.Value
		case ":path":
			// We can't just set `URL.Path`, because there's no way to ask the library to *not* escape it,
			// causing https://github.com/cloudflare/cloudflared/issues/124.
			// The only way to bypass the URL escape seems to currently be `URL.Opaque`.
			// See https://github.com/golang/go/issues/5777
			h1.URL.Opaque = fmt.Sprintf("//%v%v", h1.URL.Host, header.Value)
		case "content-length":
			contentLength, err := strconv.ParseInt(header.Value, 10, 64)
			if err != nil {
				return fmt.Errorf("unparseable content length")
			}
			h1.ContentLength = contentLength
		default:
			h1.Header.Add(http.CanonicalHeaderKey(header.Name), header.Value)
		}
	}
	return nil
}
