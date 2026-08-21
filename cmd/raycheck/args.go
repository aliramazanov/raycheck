package main

import (
	"flag"
	"strings"
)

func reorder(fs *flag.FlagSet, args []string) []string {
	var flags, positional []string

	for i := 0; i < len(args); i++ {
		a := args[i]

		if a == "--" {
			positional = append(positional, args[i+1:]...)

			break
		}

		if len(a) < 2 || a[0] != '-' {
			positional = append(positional, a)

			continue
		}

		flags = append(flags, a)

		name := strings.TrimLeft(a, "-")

		if strings.ContainsRune(name, '=') {
			continue
		}

		f := fs.Lookup(name)

		if f == nil {
			continue
		}

		if b, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && b.IsBoolFlag() {
			continue
		}

		if i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}

	if len(positional) == 0 {
		return flags
	}

	return append(append(flags, "--"), positional...)
}
