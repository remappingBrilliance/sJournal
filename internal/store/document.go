package store

import (
	"strings"

	"sjournal/internal/crypto"
)

// Block is one line of a note file, either successfully decrypted (for Edit
// Mode display/editing) or still opaque (shown greyed-out, unlockable via a
// one-off password).
type Block struct {
	Raw       string // original base64 line; always kept so it can be written back unchanged if never unlocked
	Plaintext string
	Decrypted bool
	// Migrated is set once a block is unlocked via a one-off password during
	// this Edit Mode session; migrated blocks are folded into consolidation
	// like any other decrypted block.
	Migrated bool
	// Removed marks a block (Todo item) edited down to an empty string; it
	// is dropped entirely on the next Consolidate.
	Removed bool
}

// LoadDocument reads path and attempts to decrypt every block against each
// of knownPasswords in turn (first match wins).
func LoadDocument(path string, codec *crypto.Codec, knownPasswords []string) ([]*Block, error) {
	lines, err := ReadLines(path)
	if err != nil {
		return nil, err
	}
	blocks := make([]*Block, 0, len(lines))
	for _, line := range lines {
		b := &Block{Raw: line}
		for _, pw := range knownPasswords {
			if pt, err := codec.Decrypt(pw, line); err == nil {
				b.Plaintext = pt
				b.Decrypted = true
				break
			}
		}
		blocks = append(blocks, b)
	}
	return blocks, nil
}

// TryUnlock attempts a one-off decrypt of a single still-encrypted block.
// Returns true on success.
func TryUnlock(block *Block, password string, codec *crypto.Codec) bool {
	if block.Decrypted {
		return true
	}
	pt, err := codec.Decrypt(password, block.Raw)
	if err != nil {
		return false
	}
	block.Plaintext = pt
	block.Decrypted = true
	block.Migrated = true
	return true
}

// Consolidate merges consecutive decrypted blocks into single re-encrypted
// blocks (under sessionPassword), leaving any still-encrypted blocks
// untouched and in their original relative position. Returns the block
// lines ready to write to disk.
func Consolidate(blocks []*Block, sessionPassword string, codec *crypto.Codec) ([]string, error) {
	var out []string
	var run []string

	flush := func() error {
		if len(run) == 0 {
			return nil
		}
		joined := strings.Join(run, "\n")
		line, err := codec.Encrypt(sessionPassword, joined)
		if err != nil {
			return err
		}
		out = append(out, line)
		run = nil
		return nil
	}

	for _, b := range blocks {
		if b.Removed {
			continue
		}
		if b.Decrypted {
			run = append(run, b.Plaintext)
			continue
		}
		if err := flush(); err != nil {
			return nil, err
		}
		out = append(out, b.Raw)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return out, nil
}
