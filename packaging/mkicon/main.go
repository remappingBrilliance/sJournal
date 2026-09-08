// mkicon packs a set of PNG files into a single Windows .ico file.
//
//	go run ./packaging/mkicon out.ico 16.png 32.png 48.png 256.png
//
// PNG-compressed icon entries are used throughout, which every supported
// Windows version understands. Input PNGs must be square; 256px is encoded
// as dimension 0 per the ICO spec.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	_ "image/png"
	"os"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: mkicon out.ico in1.png [in2.png ...]")
		os.Exit(2)
	}
	out := os.Args[1]
	pngs := os.Args[2:]

	type entry struct {
		w, h int
		data []byte
	}
	var entries []entry
	for _, p := range pngs {
		data, err := os.ReadFile(p)
		if err != nil {
			fail(err)
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			fail(fmt.Errorf("%s: %w", p, err))
		}
		entries = append(entries, entry{cfg.Width, cfg.Height, data})
	}

	var buf bytes.Buffer
	w16 := func(v uint16) { binary.Write(&buf, binary.LittleEndian, v) }
	w32 := func(v uint32) { binary.Write(&buf, binary.LittleEndian, v) }

	w16(0) // reserved
	w16(1) // type: icon
	w16(uint16(len(entries)))

	offset := uint32(6 + 16*len(entries))
	for _, e := range entries {
		buf.WriteByte(dim(e.w))
		buf.WriteByte(dim(e.h))
		buf.WriteByte(0) // palette
		buf.WriteByte(0) // reserved
		w16(1)           // color planes
		w16(32)          // bits per pixel
		w32(uint32(len(e.data)))
		w32(offset)
		offset += uint32(len(e.data))
	}
	for _, e := range entries {
		buf.Write(e.data)
	}

	if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil {
		fail(err)
	}
}

func dim(v int) byte {
	if v >= 256 {
		return 0
	}
	return byte(v)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "mkicon:", err)
	os.Exit(1)
}
