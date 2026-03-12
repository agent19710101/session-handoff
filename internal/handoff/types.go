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
	Version   int           `json:"version"`
	Checksum  string        `json:"checksum,omitempty"`
	Record    HandoffRecord `json:"record"`
	Signer    *SignerMeta   `json:"signer,omitempty"`
	Signature string        `json:"signature,omitempty"`
}

type SignerMeta struct {
	Name      string `json:"name,omitempty"`
	KeyID     string `json:"key_id,omitempty"`
	PublicKey string `json:"public_key,omitempty"`
}

type EncryptedBundle struct {
	Version    int    `json:"version"`
	Algorithm  string `json:"algorithm"`
	KDF        string `json:"kdf"`
	Salt       string `json:"salt"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type multiFlag []string
