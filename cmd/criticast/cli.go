package main

import (
	"flag"
	"fmt"
	"strings"
)

// parseTracePathArgs handles Go flag's "flags must follow positionals" quirk.
// Returns trace path and remaining args for flag.Parse.
func parseTracePathArgs(args []string) (tracePath string, flagArgs []string, err error) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0], args[1:], nil
	}
	return "", args, nil
}

// traceAnalyzeFlags registers shared analyze flags on fs.
func traceAnalyzeFlags(fs *flag.FlagSet) (request *string, gtLog *string, scopeFrom *string, scopeTo *string, scopePad *string, scopeHandlerGoid *uint64, topN *int, minConf *uint, spuriousUS *uint64) {
	request = fs.String("request", "", "scope: cookie (0x…), tid (decimal or tid=N), goid (goid=N), token (token=X)")
	gtLog = fs.String("gt-log", "", "ground-truth log (required for token= scope)")
	scopeFrom = fs.String("scope-from", "", "RFC3339 wall start (one request; use with token= and --scope-to)")
	scopeTo = fs.String("scope-to", "", "RFC3339 wall end")
	scopePad = fs.String("scope-pad", "", "padding each side of scope window (e.g. 10ms; default 10ms)")
	scopeHandlerGoid = fs.Uint64("scope-handler-goid", 0, "handler goid for single-request Bar B scope")
	topN = fs.Int("top", 10, "dominant waits when not scoped")
	minConf = fs.Uint("min-confidence", 0, "exclude edges below this confidence (0=off; heterogeneous scores per mechanism)")
	spuriousUS = fs.Uint64("spurious-wake-us", 10, "false-wakeup threshold µs")
	return request, gtLog, scopeFrom, scopeTo, scopePad, scopeHandlerGoid, topN, minConf, spuriousUS
}

func usageError(name, msg string) error {
	return fmt.Errorf("%s: %s", name, msg)
}
