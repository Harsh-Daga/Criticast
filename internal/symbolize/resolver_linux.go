//go:build linux

package symbolize

import "github.com/criticast/criticast/internal/trace"

func newPlatformResolver(tf *trace.File, targetBinary string) (Resolver, error) {
	if targetBinary == "" {
		targetBinary = tf.Header.TargetBinary
	}
	return NewModuleResolver(tf, targetBinary)
}
