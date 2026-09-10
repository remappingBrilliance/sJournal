package store

import (
	"regexp"
	"strings"

	"sjournal/internal/crypto"
)

// h1Re matches a single-line Markdown H1 ("# Title") — one leading '#', then
// whitespace, then non-empty text. Used to derive a friendly dropdown name
// for a Todo/Notes file from its first line.
var h1Re = regexp.MustCompile(`^#[ \t]+(\S.*?)[ \t]*$`)

// FileTitle returns a display title for a note file, taken from a leading
// Markdown H1 on its first block. ok is false — meaning the caller should
// fall back to the filename — when the file is empty, its first block does
// not decrypt against any of passwords, or its first line is not an H1.
func FileTitle(path string, codec *crypto.Codec, passwords []string) (title string, ok bool) {
	lines, err := ReadLines(path)
	if err != nil || len(lines) == 0 {
		return "", false
	}
	var plain string
	decrypted := false
	for _, pw := range passwords {
		if pt, err := codec.Decrypt(pw, lines[0]); err == nil {
			plain, decrypted = pt, true
			break
		}
	}
	if !decrypted {
		return "", false
	}
	first := plain
	if i := strings.IndexByte(first, '\n'); i >= 0 {
		first = first[:i]
	}
	m := h1Re.FindStringSubmatch(first)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// IsHeadingLine reports whether s is exactly a single Markdown H1 line
// ("# Title"), with no embedded newline. Todo Enter Mode writes such a line
// as its own block instead of wrapping it as a checklist item, and the Todo
// editor renders it as the list heading rather than an item.
func IsHeadingLine(s string) bool {
	s = strings.TrimSpace(s)
	return !strings.ContainsRune(s, '\n') && h1Re.MatchString(s)
}

// TodoIndentUnit is the on-disk indentation of one Todo nesting level: four
// spaces, the width Markdown needs for an unambiguous nested list.
const TodoIndentUnit = "    "

// IndentDepth is the Todo nesting depth implied by the leading whitespace of
// s: a tab counts as one level, and every four leading spaces as one level
// (rounded to nearest, so a hand-indented import at 2–3 or 5–6 spaces still
// lands somewhere sensible).
func IndentDepth(s string) int {
	spaces := 0
	for _, r := range s {
		switch r {
		case ' ':
			spaces++
		case '\t':
			spaces += 4
		default:
			return (spaces + 2) / 4
		}
	}
	return (spaces + 2) / 4
}

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

// EncryptIndividually re-encrypts each decrypted block on its own — no
// merging across blocks — under sessionPassword, leaving still-encrypted
// blocks byte-for-byte unchanged and dropping Removed ones. Unlike
// Consolidate, this preserves one-block-per-chunk boundaries, which Todo
// files rely on: each chunk may hold one or more checklist items (parsed
// from its decrypted markdown), and merging chunks together on every save
// would keep folding old items into new ones.
func EncryptIndividually(blocks []*Block, sessionPassword string, codec *crypto.Codec) ([]string, error) {
	var out []string
	for _, b := range blocks {
		if b.Removed {
			continue
		}
		if !b.Decrypted {
			out = append(out, b.Raw)
			continue
		}
		line, err := codec.Encrypt(sessionPassword, b.Plaintext)
		if err != nil {
			return nil, err
		}
		out = append(out, line)
	}
	return out, nil
}
