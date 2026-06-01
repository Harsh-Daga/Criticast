package symbolize

// Module is an executable mapping captured at record time.
type Module struct {
	Path    string `json:"path"`
	Start   uint64 `json:"start"`
	End     uint64 `json:"end"`
	BuildID string `json:"build_id,omitempty"`
}
