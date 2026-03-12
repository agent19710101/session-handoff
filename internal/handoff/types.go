package handoff

// HandoffRecord captures one saved coding-session handoff.
type HandoffRecord struct {
	ID        string   `json:"id"`
	CreatedAt string   `json:"created_at"`
	Tool      string   `json:"tool"`
	Project   string   `json:"project"`
	Title     string   `json:"title"`
	Summary   string   `json:"summary"`
	Next      []string `json:"next"`
	Changed   []string `json:"changed,omitempty"`
}

type storeFile struct {
	Version int             `json:"version"`
	Items   []HandoffRecord `json:"items"`
}

type ExportBundle struct {
	Version  int           `json:"version"`
	Checksum string        `json:"checksum,omitempty"`
	Record   HandoffRecord `json:"record"`
}

type multiFlag []string
