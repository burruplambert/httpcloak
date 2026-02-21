//go:build ignore

package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sardanioss/httpcloak"
)

func main() {
	proxyURL := "http://sp83l3nge0:%2BkEynC61jHlFpy5x9v@isp.decodo.com:10000"
	url := "https://www.walmart.com/ip/3M-1426-Packing-Tape-w-Dispenser-2inx800ft-6-Rolls-Clear/14935484"

	for i := 1; i <= 10; i++ {
		var wg sync.WaitGroup
		var specResult, noSpecResult string

		wg.Add(2)
		go func() {
			defer wg.Done()
			sess := httpcloak.NewSession("chrome-144-ios",
				httpcloak.WithSessionTCPProxy(proxyURL),
				httpcloak.WithSessionTimeout(60*time.Second),
				httpcloak.WithForceHTTP2(),
			)
			t := time.Now()
			r, err := sess.Do(context.Background(), &httpcloak.Request{URL: url, Timeout: 60 * time.Second})
			if err != nil {
				specResult = fmt.Sprintf("ERR %v", time.Since(t).Round(time.Millisecond))
			} else {
				specResult = fmt.Sprintf("%d %v", r.StatusCode, time.Since(t).Round(time.Millisecond))
				r.Close()
			}
			sess.Close()
		}()

		go func() {
			defer wg.Done()
			sess := httpcloak.NewSession("chrome-144-ios",
				httpcloak.WithSessionTCPProxy(proxyURL),
				httpcloak.WithSessionTimeout(60*time.Second),
				httpcloak.WithForceHTTP2(),
				httpcloak.WithDisableSpeculativeTLS(),
			)
			t := time.Now()
			r, err := sess.Do(context.Background(), &httpcloak.Request{URL: url, Timeout: 60 * time.Second})
			if err != nil {
				noSpecResult = fmt.Sprintf("ERR %v", time.Since(t).Round(time.Millisecond))
			} else {
				noSpecResult = fmt.Sprintf("%d %v", r.StatusCode, time.Since(t).Round(time.Millisecond))
				r.Close()
			}
			sess.Close()
		}()

		wg.Wait()
		fmt.Printf("[%2d] spec=%-14s no-spec=%s\n", i, specResult, noSpecResult)
	}
}
