//go:build !linux

package symbolize

import "github.com/criticast/criticast/internal/trace"

func newPlatformResolver(tf *trace.File, targetBinary string) (Resolver, error) {
	if targetBinary == "" {
		targetBinary = tf.Header.TargetBinary
	}
	tr := NewTraceResolver(tf.Stacks)
	elf, err := OpenELFIfExists(targetBinary)
	if err != nil {
		return nil, err
	}
	r := &ChainedResolver{trace: tr, elf: elf}
	if elf == nil && len(tr.stacks) > 0 {
		r.hint = StrippedBinaryHint
	}
	return r, nil
}
