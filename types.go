package main

// handoffRecord captures one saved coding-session handoff.
type handoffRecord struct {
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
	Items   []handoffRecord `json:"items"`
}

type exportBundle struct {
	Version  int           `json:"version"`
	Checksum string        `json:"checksum,omitempty"`
	Record   handoffRecord `json:"record"`
}

type multiFlag []string
