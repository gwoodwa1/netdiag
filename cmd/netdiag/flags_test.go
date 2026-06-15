package main

import (
	"flag"
	"reflect"
	"testing"
)

func TestInterspersedArgsSupportsInputFirstAndKeyValue(t *testing.T) {
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.String("output", "", "")
	flags.String("o", "", "")
	flags.String("renderer", "", "")
	flags.Bool("report", false, "")

	got := interspersedArgs(flags, []string{
		"diagram.yaml", "--renderer=drawio", "--report", "-o", "diagram.drawio",
	})
	want := []string{"--renderer=drawio", "--report", "-o", "diagram.drawio", "diagram.yaml"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("interspersedArgs() = %#v, want %#v", got, want)
	}
	if err := flags.Parse(got); err != nil {
		t.Fatal(err)
	}
	if flags.NArg() != 1 || flags.Arg(0) != "diagram.yaml" {
		t.Fatalf("unexpected positional arguments: %#v", flags.Args())
	}
}

func TestInterspersedArgsPreservesDashInputAndDoubleDash(t *testing.T) {
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.String("format", "", "")

	got := interspersedArgs(flags, []string{"--format=auto", "-", "--", "--literal"})
	want := []string{"--format=auto", "--", "-", "--literal"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("interspersedArgs() = %#v, want %#v", got, want)
	}
	if err := flags.Parse(got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(flags.Args(), []string{"-", "--literal"}) {
		t.Fatalf("unexpected literal positional arguments: %#v", flags.Args())
	}
}
