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
func traceAnalyzeFlags(fs *flag.FlagSet) (request *string, topN *int, minConf *uint, spuriousUS *uint64) {
	request = fs.String("request", "", "scope by cookie (0x…) or tid")
	topN = fs.Int("top", 10, "dominant waits when not scoped")
	minConf = fs.Uint("min-confidence", 0, "minimum edge confidence 0-100")
	spuriousUS = fs.Uint64("spurious-wake-us", 10, "false-wakeup threshold µs")
	return request, topN, minConf, spuriousUS
}

func usageError(name, msg string) error {
	return fmt.Errorf("%s: %s", name, msg)
}
