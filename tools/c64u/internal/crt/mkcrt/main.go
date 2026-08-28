// Command mkcrt wraps a raw 8 KB ROM image in a VICE .crt container, the
// format the Ultimate expects in /flash/carts.
//
//	go run ./internal/crt/mkcrt -in wedge.bin -out wedge.crt -name "ULTIMATE WEDGE"
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/crt"
)

func main() {
	in := flag.String("in", "", "raw 8 KB ROM image")
	out := flag.String("out", "", "output .crt file")
	name := flag.String("name", "CARTRIDGE", "cartridge name stored in the header")
	hw := flag.Uint("type", uint(crt.HardwareNormal), "CRT hardware type (0 normal, 19 Magic Desk)")
	flag.Parse()

	if *in == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}

	rom, err := os.ReadFile(*in)
	if err != nil {
		fail(err)
	}

	cart, err := crt.EightKType(*name, rom, uint16(*hw))
	if err != nil {
		fail(err)
	}

	f, err := os.Create(*out)
	if err != nil {
		fail(err)
	}
	defer f.Close()

	n, err := cart.WriteTo(f)
	if err != nil {
		fail(err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d bytes)\n", *out, n)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "mkcrt:", err)
	os.Exit(1)
}
