package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caddyserver/caddy/caddyfile"
)

func TestStop(t *testing.T) {
	config := "proxy . %s {\n health_check /healthcheck:%s %dms \n}"
	tests := []struct {
		intervalInMilliseconds  int
		numHealthcheckIntervals int
	}{
		{5, 1},
		{5, 2},
		{5, 3},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {

			// Set up proxy.
			var counter int64
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.Body.Close()
				atomic.AddInt64(&counter, 1)
			}))

			defer backend.Close()

			port := backend.URL[17:] // Remove all crap up to the port
			back := backend.URL[7:]  // Remove http://
			c := caddyfile.NewDispenser("Testfile", strings.NewReader(fmt.Sprintf(config, back, port, test.intervalInMilliseconds)))
			upstreams, err := NewStaticUpstreams(&c)
			if err != nil {
				t.Errorf("Test %d, expected no error. Got: %s", i, err)
			}

			// Give some time for healthchecks to hit the server.
			time.Sleep(time.Duration(test.intervalInMilliseconds*test.numHealthcheckIntervals) * time.Millisecond)

			for _, upstream := range upstreams {
				if err := upstream.Stop(); err != nil {
					t.Errorf("Test %d, expected no error stopping upstream, got: %s", i, err)
				}
			}

			counterAfterShutdown := atomic.LoadInt64(&counter)

			// Give some time to see if healthchecks are still hitting the server.
			time.Sleep(time.Duration(test.intervalInMilliseconds*test.numHealthcheckIntervals) * time.Millisecond)

			if counterAfterShutdown == 0 {
				t.Errorf("Test %d, Expected healthchecks to hit test server, got none", i)
			}

			// health checks are in a go routine now, so one may well occur after we shutdown,
			// but we only ever expect one more
			counterAfterWaiting := atomic.LoadInt64(&counter)
			if counterAfterWaiting > (counterAfterShutdown + 1) {
				t.Errorf("Test %d, expected no more healthchecks after shutdown. got: %d healthchecks after shutdown", i, counterAfterWaiting-counterAfterShutdown)
			}
		})
	}
}
