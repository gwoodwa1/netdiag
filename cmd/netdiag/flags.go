package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func commandFlags(name, usage string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ExitOnError)
	flags.SetOutput(os.Stderr)
	flags.Usage = func() {
		_, _ = fmt.Fprintln(flags.Output(), usage)
		flags.PrintDefaults()
	}
	return flags
}

func parseCommandFlags(flags *flag.FlagSet, args []string) {
	_ = flags.Parse(interspersedArgs(flags, args))
}

// interspersedArgs keeps netdiag's documented input-first syntax while using
// the standard flag package for parsing, validation, aliases, and key=value.
func interspersedArgs(flags *flag.FlagSet, args []string) []string {
	var options, positionals []string
	literalPositionals := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			positionals = append(positionals, args[index+1:]...)
			literalPositionals = true
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			positionals = append(positionals, arg)
			continue
		}

		name := strings.TrimLeft(arg, "-")
		if option, _, ok := strings.Cut(name, "="); ok {
			name = option
		}
		value := flags.Lookup(name)
		options = append(options, arg)
		if strings.Contains(arg, "=") || value == nil || isBoolFlag(value) {
			continue
		}
		if index+1 < len(args) {
			index++
			options = append(options, args[index])
		}
	}
	if literalPositionals {
		options = append(options, "--")
	}
	return append(options, positionals...)
}

func isBoolFlag(value *flag.Flag) bool {
	boolean, ok := value.Value.(interface{ IsBoolFlag() bool })
	return ok && boolean.IsBoolFlag()
}
