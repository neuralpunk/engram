package cmd

import (
	"fmt"
	"strconv"
	"strings"
)

// Args provides consistent flag parsing that works with flags anywhere in the
// argument list (before, after, or between positional arguments). Go's stdlib
// flag package stops at the first non-flag argument, which breaks commands
// like: engram store "some fact" --scope global
//
// Usage:
//
//	a := newArgs(args)
//	a.String(&scope, "scope", "global", "Filter by scope")
//	a.Bool(&raw, "raw", false, "Plain text output")
//	a.Int(&limit, "limit", 10, "Maximum results")
//	if err := a.Parse(); err != nil { return err }
//	positional := a.Positional()
type Args struct {
	raw   []string
	flags []flagDef
	help  string
	pos   []string
}

type flagDef struct {
	name     string
	desc     string
	kind     string // "string", "bool", "int", "int64"
	strPtr   *string
	boolPtr  *bool
	intPtr   *int
	int64Ptr *int64
	defStr   string
	defBool  bool
	defInt   int
	defInt64 int64
}

func newArgs(args []string, helpPrefix string) *Args {
	return &Args{raw: args, help: helpPrefix}
}

func (a *Args) String(ptr *string, name string, def string, desc string) {
	*ptr = def
	a.flags = append(a.flags, flagDef{name: name, desc: desc, kind: "string", strPtr: ptr, defStr: def})
}

func (a *Args) Bool(ptr *bool, name string, def bool, desc string) {
	*ptr = def
	a.flags = append(a.flags, flagDef{name: name, desc: desc, kind: "bool", boolPtr: ptr, defBool: def})
}

func (a *Args) Int(ptr *int, name string, def int, desc string) {
	*ptr = def
	a.flags = append(a.flags, flagDef{name: name, desc: desc, kind: "int", intPtr: ptr, defInt: def})
}

func (a *Args) Int64(ptr *int64, name string, def int64, desc string) {
	*ptr = def
	a.flags = append(a.flags, flagDef{name: name, desc: desc, kind: "int64", int64Ptr: ptr, defInt64: def})
}

func (a *Args) Parse() error {
	flagMap := make(map[string]*flagDef, len(a.flags))
	for i := range a.flags {
		flagMap[a.flags[i].name] = &a.flags[i]
	}

	for i := 0; i < len(a.raw); i++ {
		arg := a.raw[i]

		if arg == "--help" || arg == "-h" {
			return errHelp{a.helpText()}
		}

		if !strings.HasPrefix(arg, "--") {
			a.pos = append(a.pos, arg)
			continue
		}

		name := strings.TrimPrefix(arg, "--")

		// Handle --flag=value
		if idx := strings.IndexByte(name, '='); idx >= 0 {
			flagName := name[:idx]
			flagVal := name[idx+1:]
			f, ok := flagMap[flagName]
			if !ok {
				return fmt.Errorf("unknown flag: --%s\n\n%s", flagName, a.helpText())
			}
			return a.setFlag(f, flagVal)
		}

		f, ok := flagMap[name]
		if !ok {
			return fmt.Errorf("unknown flag: --%s\n\n%s", name, a.helpText())
		}

		if f.kind == "bool" {
			*f.boolPtr = true
			continue
		}

		// Value flags require a next argument
		i++
		if i >= len(a.raw) {
			return fmt.Errorf("flag --%s requires a value", name)
		}
		if err := a.setFlag(f, a.raw[i]); err != nil {
			return err
		}
	}
	return nil
}

func (a *Args) setFlag(f *flagDef, val string) error {
	switch f.kind {
	case "string":
		*f.strPtr = val
	case "int":
		n, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid value for --%s: %q (expected integer)", f.name, val)
		}
		*f.intPtr = n
	case "int64":
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid value for --%s: %q (expected integer)", f.name, val)
		}
		*f.int64Ptr = n
	case "bool":
		switch strings.ToLower(val) {
		case "true", "1", "yes":
			*f.boolPtr = true
		case "false", "0", "no":
			*f.boolPtr = false
		default:
			return fmt.Errorf("invalid value for --%s: %q (expected true/false)", f.name, val)
		}
	}
	return nil
}

func (a *Args) Positional() []string {
	return a.pos
}

func (a *Args) helpText() string {
	var b strings.Builder
	b.WriteString(a.help)
	if len(a.flags) > 0 {
		b.WriteString("\n\nFlags:\n")
		for _, f := range a.flags {
			switch f.kind {
			case "bool":
				fmt.Fprintf(&b, "  --%-14s %s\n", f.name, f.desc)
			case "int":
				fmt.Fprintf(&b, "  --%-14s %s (default: %d)\n", f.name, f.desc, f.defInt)
			case "int64":
				fmt.Fprintf(&b, "  --%-14s %s (default: %d)\n", f.name, f.desc, f.defInt64)
			case "string":
				if f.defStr != "" {
					fmt.Fprintf(&b, "  --%-14s %s (default: %s)\n", f.name, f.desc, f.defStr)
				} else {
					fmt.Fprintf(&b, "  --%-14s %s\n", f.name, f.desc)
				}
			}
		}
	}
	return b.String()
}

// errHelp is returned when --help is requested. It's not an error,
// but uses the error path to short-circuit execution.
type errHelp struct {
	text string
}

func (e errHelp) Error() string {
	return e.text
}

// IsHelp returns true if the error is a help request (not a real error).
func IsHelp(err error) bool {
	_, ok := err.(errHelp)
	return ok
}
