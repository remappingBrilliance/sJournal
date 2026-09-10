package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"sjournal/internal/crypto"
)

func TestNextNumberedNameSequencing(t *testing.T) {
	dir := t.TempDir()

	name, err := NextNumberedName(dir, "TODO")
	if err != nil || name != "TODO001.md" {
		t.Fatalf("got %q, %v; want TODO001.md", name, err)
	}

	for _, n := range []string{"TODO001.md", "TODO002.md", "TODO005.md"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	name, err = NextNumberedName(dir, "TODO")
	if err != nil || name != "TODO006.md" {
		t.Fatalf("got %q, %v; want TODO006.md", name, err)
	}
}

func TestListNumberedDescending(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"NOTES001.md", "NOTES003.md", "NOTES002.md"} {
		os.WriteFile(filepath.Join(dir, n), []byte("x\n"), 0o600)
	}
	got, err := ListNumbered(dir, "NOTES")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"NOTES003.md", "NOTES002.md", "NOTES001.md"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestMostRecentFilePicksNewestAcrossKinds(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "2020-01-01.md")
	newer := filepath.Join(dir, "TODO001.md")
	os.WriteFile(old, []byte("a\n"), 0o600)
	time.Sleep(20 * time.Millisecond)
	os.WriteFile(newer, []byte("b\n"), 0o600)

	path, ok, err := MostRecentFile(dir)
	if err != nil || !ok {
		t.Fatalf("err=%v ok=%v", err, ok)
	}
	if path != newer {
		t.Fatalf("got %q, want %q", path, newer)
	}
}

func TestMostRecentFileEmptyDir(t *testing.T) {
	dir := t.TempDir()
	_, ok, err := MostRecentFile(dir)
	if err != nil || ok {
		t.Fatalf("err=%v ok=%v, want ok=false", err, ok)
	}
}

func TestAppendAndReadLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "2026-09-07.md")
	if err := AppendLine(path, "line1"); err != nil {
		t.Fatal(err)
	}
	if err := AppendLine(path, "line2"); err != nil {
		t.Fatal(err)
	}
	lines, err := ReadLines(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 || lines[0] != "line1" || lines[1] != "line2" {
		t.Fatalf("got %v", lines)
	}
}

// TestConsolidatePreservesOrderAndMigratesUnlocked exercises the actual
// spec-critical behavior: consecutive decrypted runs merge into one new
// block each, still-encrypted blocks are left untouched and in place, and a
// block unlocked via a one-off password this session gets folded in.
func TestConsolidatePreservesOrderAndMigratesUnlocked(t *testing.T) {
	codec := crypto.NewCodec()
	const sessionPW = "current-password"
	const oldPW = "old-password"

	line1, _ := codec.Encrypt(sessionPW, "entry one")
	line2, _ := codec.Encrypt(sessionPW, "entry two")
	lockedLine, _ := codec.Encrypt(oldPW, "old secret")
	line3, _ := codec.Encrypt(sessionPW, "entry three")

	dir := t.TempDir()
	path := filepath.Join(dir, "2026-09-07.md")
	for _, l := range []string{line1, line2, lockedLine, line3} {
		if err := AppendLine(path, l); err != nil {
			t.Fatal(err)
		}
	}

	blocks, err := LoadDocument(path, codec, []string{sessionPW})
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 4 {
		t.Fatalf("got %d blocks, want 4", len(blocks))
	}
	if !blocks[0].Decrypted || !blocks[1].Decrypted || blocks[2].Decrypted || !blocks[3].Decrypted {
		t.Fatalf("unexpected decrypt state: %+v %+v %+v %+v", blocks[0], blocks[1], blocks[2], blocks[3])
	}

	// First consolidation: the locked block stays locked, so blocks 1+2
	// merge into one new block, block 3 (locked) passes through unchanged,
	// block 4 becomes its own new block (nothing to merge with across the
	// locked gap).
	lines, err := Consolidate(blocks, sessionPW, codec)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3: %v", len(lines), lines)
	}
	if lines[1] != lockedLine {
		t.Fatalf("locked block was not preserved byte-for-byte in place: got %q want %q", lines[1], lockedLine)
	}
	merged, err := codec.Decrypt(sessionPW, lines[0])
	if err != nil {
		t.Fatalf("merged block should decrypt with session password: %v", err)
	}
	if merged != "entry one\nentry two" {
		t.Fatalf("got merged plaintext %q", merged)
	}

	// Now unlock the still-locked block with its one-off password and
	// confirm it migrates onto the session password on the next save.
	if !TryUnlock(blocks[2], oldPW, codec) {
		t.Fatal("TryUnlock should have succeeded with the correct one-off password")
	}
	if !blocks[2].Migrated {
		t.Fatal("block should be marked Migrated after a successful one-off unlock")
	}

	lines2, err := Consolidate(blocks, sessionPW, codec)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines2) != 1 {
		t.Fatalf("all blocks should now merge into a single new block, got %d: %v", len(lines2), lines2)
	}
	final, err := codec.Decrypt(sessionPW, lines2[0])
	if err != nil {
		t.Fatalf("final consolidated block should decrypt under the session password: %v", err)
	}
	if final != "entry one\nentry two\nold secret\nentry three" {
		t.Fatalf("got %q", final)
	}

	// The old password must no longer be needed at all: re-loading with only
	// the session password decrypts every block after the migration+save.
	if err := WriteLines(path, lines2); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadDocument(path, codec, []string{sessionPW})
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded) != 1 || !reloaded[0].Decrypted {
		t.Fatalf("expected the single migrated block to decrypt under session password alone: %+v", reloaded)
	}
}

func TestConsolidateDropsRemovedBlocks(t *testing.T) {
	codec := crypto.NewCodec()
	const pw = "pw"
	l1, _ := codec.Encrypt(pw, "keep me")
	l2, _ := codec.Encrypt(pw, "- [ ] delete me")

	blocks := []*Block{
		{Raw: l1, Plaintext: "keep me", Decrypted: true},
		{Raw: l2, Plaintext: "- [ ] delete me", Decrypted: true, Removed: true},
	}
	lines, err := Consolidate(blocks, pw, codec)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1 (removed block dropped)", len(lines))
	}
	pt, err := codec.Decrypt(pw, lines[0])
	if err != nil || pt != "keep me" {
		t.Fatalf("pt=%q err=%v", pt, err)
	}
}

// TestEncryptIndividuallyNeverMerges is the regression test for the bug
// where Todo saves were merging every checklist item into one chunk
// (Consolidate's behavior, correct for Journal/Notes but wrong for Todo:
// each item must stay independently editable/decryptable).
func TestEncryptIndividuallyNeverMerges(t *testing.T) {
	codec := crypto.NewCodec()
	const pw = "pw"
	const oldPW = "old-pw"

	l1, _ := codec.Encrypt(pw, "- [ ] item one")
	l2, _ := codec.Encrypt(pw, "- [x] item two")
	lockedLine, _ := codec.Encrypt(oldPW, "- [ ] old item")

	blocks := []*Block{
		{Raw: l1, Plaintext: "- [ ] item one", Decrypted: true},
		{Raw: l2, Plaintext: "- [x] item two", Decrypted: true},
		{Raw: lockedLine, Decrypted: false},
	}

	lines, err := EncryptIndividually(blocks, pw, codec)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (one per block, no merging): %v", len(lines), lines)
	}
	if lines[2] != lockedLine {
		t.Fatalf("locked block should pass through unchanged")
	}
	pt1, err := codec.Decrypt(pw, lines[0])
	if err != nil || pt1 != "- [ ] item one" {
		t.Fatalf("pt1=%q err=%v", pt1, err)
	}
	pt2, err := codec.Decrypt(pw, lines[1])
	if err != nil || pt2 != "- [x] item two" {
		t.Fatalf("pt2=%q err=%v", pt2, err)
	}
}

func TestEncryptIndividuallyDropsRemoved(t *testing.T) {
	codec := crypto.NewCodec()
	const pw = "pw"
	l1, _ := codec.Encrypt(pw, "- [ ] keep")
	l2, _ := codec.Encrypt(pw, "- [ ] drop")

	blocks := []*Block{
		{Raw: l1, Plaintext: "- [ ] keep", Decrypted: true},
		{Raw: l2, Plaintext: "- [ ] drop", Decrypted: true, Removed: true},
	}
	lines, err := EncryptIndividually(blocks, pw, codec)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
}

func TestIsHeadingLine(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"# Shopping list", true},
		{"#\tTabbed", true},
		{"  # leading space trimmed  ", true},
		{"# multi\nline", false},
		{"## h2 is not h1", false},
		{"#nospace", false},
		{"- [ ] a task", false},
		{"just text", false},
		{"", false},
		{"#", false},
	}
	for _, c := range cases {
		if got := IsHeadingLine(c.in); got != c.want {
			t.Errorf("IsHeadingLine(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestIndentDepth(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"- [ ] top", 0},
		{"  - [ ] two spaces", 1},
		{"    - [ ] four spaces", 1},
		{"      - [ ] six spaces", 2},
		{"        - [ ] eight spaces", 2},
		{"\t- [ ] one tab", 1},
		{"\t\t- [ ] two tabs", 2},
		{"   - [ ] three spaces (their example: a sub-item)", 1},
		{"", 0},
	}
	for _, c := range cases {
		if got := IndentDepth(c.in); got != c.want {
			t.Errorf("IndentDepth(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestFileTitle(t *testing.T) {
	codec := crypto.NewCodec()
	const pw = "current"
	const oldPW = "old"
	dir := t.TempDir()

	write := func(name string, blocks ...string) string {
		p := filepath.Join(dir, name)
		for _, b := range blocks {
			line, err := codec.Encrypt(pw, b)
			if err != nil {
				t.Fatal(err)
			}
			if err := AppendLine(p, line); err != nil {
				t.Fatal(err)
			}
		}
		return p
	}

	t.Run("h1 first block", func(t *testing.T) {
		p := write("NOTES001.md", "# Groceries\n\nmilk, eggs", "- [ ] later")
		got, ok := FileTitle(p, codec, []string{pw})
		if !ok || got != "Groceries" {
			t.Fatalf("got (%q, %v), want (\"Groceries\", true)", got, ok)
		}
	})

	t.Run("no heading falls back", func(t *testing.T) {
		p := write("NOTES002.md", "plain first line")
		if _, ok := FileTitle(p, codec, []string{pw}); ok {
			t.Fatal("expected ok=false for a non-heading first line")
		}
	})

	t.Run("undecryptable first block falls back", func(t *testing.T) {
		p := filepath.Join(dir, "NOTES003.md")
		locked, _ := codec.Encrypt(oldPW, "# Secret heading")
		if err := AppendLine(p, locked); err != nil {
			t.Fatal(err)
		}
		if _, ok := FileTitle(p, codec, []string{pw}); ok {
			t.Fatal("expected ok=false when the first block does not decrypt")
		}
	})

	t.Run("empty / missing file falls back", func(t *testing.T) {
		if _, ok := FileTitle(filepath.Join(dir, "nope.md"), codec, []string{pw}); ok {
			t.Fatal("expected ok=false for a missing file")
		}
	})
}
