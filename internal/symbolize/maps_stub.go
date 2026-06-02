//go:build !linux

package symbolize

// ModulesFromPID is only available on Linux.
func ModulesFromPID(int, string) ([]Module, error) {
	return nil, nil
}
