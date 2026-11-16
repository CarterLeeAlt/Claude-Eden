package installfiles

import (
	"context"
	"flag"
	"fmt"
	"io"
)

type stringList []string

func (s *stringList) String() string     { return fmt.Sprint([]string(*s)) }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

func CLI(ctx context.Context, args []string, output, stderr io.Writer) int {
	var o Options
	var keep stringList
	f := flag.NewFlagSet("eden-install", flag.ContinueOnError)
	f.SetOutput(stderr)
	f.StringVar(&o.Operation, "Operation", "", "installation operation")
	f.StringVar(&o.Target, "Target", "", "destination directory")
	f.StringVar(&o.Source, "Source", "", "source directory")
	f.StringVar(&o.Receipt, "Receipt", "", "transaction receipt")
	f.StringVar(&o.Manifest, "Manifest", "", "embedded ownership manifest")
	f.StringVar(&o.Backup, "Backup", "", "backup directory")
	f.StringVar(&o.Diagnostics, "Diagnostics", "", "private diagnostic output")
	f.BoolVar(&o.LegacyTarget, "LegacyTarget", false, "restore a legacy program snapshot")
	f.Var(&keep, "KeepInPlace", "top-level directory retained in place; repeatable")
	if e := f.Parse(args); e != nil {
		return 2
	}
	if f.NArg() != 0 {
		fmt.Fprintln(stderr, "Unexpected installation arguments")
		return 2
	}
	o.KeepInPlace = keep
	o.Output = output
	if e := Run(ctx, o); e != nil {
		fmt.Fprintln(stderr, e)
		return 1
	}
	return 0
}
