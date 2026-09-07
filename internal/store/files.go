// Package store manages sJournal's on-disk layout: the notes folder, the
// three file kinds (Journal/Todo/Notes), and line-level read/write of the
// encrypted block format.
package store

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	JournalDateLayout = "2006-01-02"
)

var (
	journalNameRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\.md$`)
	todoNameRe    = regexp.MustCompile(`^TODO(\d{3})\.md$`)
	notesNameRe   = regexp.MustCompile(`^NOTES(\d{3})\.md$`)
)

// EnsureNotesDir creates the notes folder if it doesn't exist.
func EnsureNotesDir(dir string) error {
	return os.MkdirAll(dir, 0o700)
}

// ListJournalFiles returns journal file names (e.g. "2026-09-07.md") present
// in dir, newest date first.
func ListJournalFiles(dir string) ([]string, error) {
	return listMatching(dir, journalNameRe, func(a, b string) bool { return a > b })
}

// ListNumbered returns Todo or Notes file names for the given prefix
// ("TODO" or "NOTES"), highest number first.
func ListNumbered(dir, prefix string) ([]string, error) {
	re := todoNameRe
	if prefix == "NOTES" {
		re = notesNameRe
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	type numbered struct {
		name string
		n    int
	}
	var found []numbered
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := re.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		found = append(found, numbered{e.Name(), n})
	}
	sort.Slice(found, func(i, j int) bool { return found[i].n > found[j].n })
	names := make([]string, len(found))
	for i, f := range found {
		names[i] = f.name
	}
	return names, nil
}

// NextNumberedName returns the next filename in sequence for prefix
// ("TODO" or "NOTES"), e.g. "TODO001.md" if none exist yet, else one past
// the current highest.
func NextNumberedName(dir, prefix string) (string, error) {
	existing, err := ListNumbered(dir, prefix)
	if err != nil {
		return "", err
	}
	next := 1
	if len(existing) > 0 {
		re := todoNameRe
		if prefix == "NOTES" {
			re = notesNameRe
		}
		m := re.FindStringSubmatch(existing[0]) // highest is first (sorted desc)
		if m != nil {
			n, _ := strconv.Atoi(m[1])
			next = n + 1
		}
	}
	return fmt.Sprintf("%s%03d.md", prefix, next), nil
}

func listMatching(dir string, re *regexp.Regexp, less func(a, b string) bool) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if re.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Slice(names, func(i, j int) bool { return less(names[i], names[j]) })
	return names, nil
}

// TodayJournalName returns today's journal filename, e.g. "2026-09-07.md".
func TodayJournalName() string {
	return time.Now().Format(JournalDateLayout) + ".md"
}

// MostRecentFile scans Journal + Todo + Notes files in dir and returns the
// path of whichever was modified most recently. ok is false if the notes
// folder contains no note files at all.
func MostRecentFile(dir string) (path string, ok bool, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}

	var bestName string
	var bestMod time.Time
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !journalNameRe.MatchString(name) && !todoNameRe.MatchString(name) && !notesNameRe.MatchString(name) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if bestName == "" || info.ModTime().After(bestMod) {
			bestName = name
			bestMod = info.ModTime()
		}
	}
	if bestName == "" {
		return "", false, nil
	}
	return filepath.Join(dir, bestName), true, nil
}

// ReadLines reads a note file's non-empty lines (each one block). Returns an
// empty slice (not an error) if the file doesn't exist yet.
func ReadLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

// AppendLine appends one block line to path, creating the file (and its
// parent dir) if necessary.
func AppendLine(path string, line string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line + "\n")
	return err
}

// WriteLines overwrites path with the given block lines, one per line.
func WriteLines(path string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0o600)
}
