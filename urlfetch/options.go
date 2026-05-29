package urlfetch

import (
	"net/http"
	"time"
)

const defaultHTTPTimeout = 30 * time.Second

var defaultClient = &http.Client{Timeout: defaultHTTPTimeout}

// Options configures urlfetch tool behavior that is intentionally owned by the
// caller rather than hidden inside the tool.
type Options struct {
	// HTTPClient overrides the default HTTP client used for https:// requests.
	// If nil, a client with a 30-second timeout is used.
	HTTPClient *http.Client
}

func (o Options) withDefaults() Options {
	if o.HTTPClient == nil {
		o.HTTPClient = defaultClient
	}
	return o
}
