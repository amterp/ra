# Ra

![ra-logo](./media/ra.png)

[![Go Report Card](https://goreportcard.com/badge/github.com/amterp/ra)](https://goreportcard.com/report/github.com/amterp/ra)
[![Go Reference](https://pkg.go.dev/badge/github.com/amterp/ra.svg)](https://pkg.go.dev/github.com/amterp/ra)
[![Latest Release](https://img.shields.io/github/v/release/amterp/ra)](https://github.com/amterp/ra/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Flexible CLI argument parsing for Go.** Ra lets you build sophisticated command-line tools where arguments work as
both positional parameters and named flags - giving your users the flexibility to use whichever style feels natural.

Powers the argument parsing in [Rad](https://github.com/amterp/rad)! ⚡️

## Quick Start

```bash
go get github.com/amterp/ra
```

```go
package main

import (
	"fmt"
	"os"
	"github.com/amterp/ra"
)

func main() {
	cmd := ra.NewCmd("myapp")

	// This arg works both positionally AND as a flag
	input, _ := ra.NewString("input").Register(cmd)
	format, _ := ra.NewString("format").SetDefault("json").Register(cmd)
	verbose, _ := ra.NewBool("verbose").SetShort("v").Register(cmd)

	cmd.ParseOrExit(os.Args[1:])

	fmt.Printf("Input: %s, Format: %s, Verbose: %t\n", *input, *format, *verbose)
}
```

**Both of these work identically:**

```bash
myapp data.txt json --verbose     # positional style
myapp --input data.txt --format json -v  # flag style  
```

## Key Features

- **🎭 Dual-Nature Arguments**: Every argument works as both a positional parameter and a named flag
- **🔒 Rich Constraints**: Enum values, regex patterns, min/max ranges, and relational constraints (requires/excludes)
- **🧠 Smart Parsing**: Bool flag clustering (`-abc`), negative number handling, variadic arguments
- **🛡️ Type Safety**: Generic flag types with compile-time safety and automatic validation
- **🌳 Subcommands**: Nested commands with global flag inheritance
- **💫 User-Friendly**: Auto-help generation, colorized output, clear error messages

## Why Ra?

**vs cobra/pflag:** While cobra and pflag are excellent for traditional flag-based CLIs, Ra's dual-nature arguments provide more flexibility. In cobra, you have to choose between positional args OR flags - Ra lets you have both. Ra also provides richer constraint validation and doesn't impose POSIX compliance patterns.

**Key strengths:**
- **POSIX-friendly, but not prescriptive** - you choose your parsing style, Ra adapts
- **Dual-nature arguments** - users can mix positional and flag styles naturally
- **Built for complex CLIs** - designed for sophisticated argument handling
- **Real-world battle-tested** - powers the Rad programming language's CLI

Perfect for CLIs that need more flexibility than basic flag parsing but less complexity than full frameworks.

## Example

```go
package main

import (
	"fmt"
	"os"
	"github.com/amterp/ra"
)

func main() {
	cmd := ra.NewCmd("fileproc")
	cmd.SetDescription("A file processor with various output formats")

	// Required input file (positional or --input)
	input, _ := ra.NewString("input").Register(cmd)

	// Format with enum constraint and default
	format, _ := ra.NewString("format").
		SetShort("f").
		SetDefault("json").
		SetEnumConstraint([]string{"json", "xml", "yaml"}).
		Register(cmd)

	// Timeout with range constraint
	timeout, _ := ra.NewInt("timeout").
		SetDefault(30).
		SetMin(1, true).SetMax(300, true).
		Register(cmd)

	// Mutually exclusive flags
	verbose, _ := ra.NewBool("verbose").
		SetShort("v").
		SetExcludes([]string{"quiet"}).
		Register(cmd)

	quiet, _ := ra.NewBool("quiet").
		SetShort("q").
		SetExcludes([]string{"verbose"}).
		Register(cmd)

	// Variadic tags
	tags, _ := ra.NewStringSlice("tags").
		SetVariadic(true).
		SetOptional(true).
		Register(cmd)

	// Subcommand
	validateCmd := ra.NewCmd("validate")
	validateInput, _ := ra.NewString("input").Register(validateCmd)
	strict, _ := ra.NewBool("strict").Register(validateCmd)
	validateUsed, _ := cmd.RegisterCmd(validateCmd)

	// Parse
	err := cmd.ParseOrError(os.Args[1:])
	if err == ra.HelpInvokedErr {
		return // Help was shown
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Handle subcommand
	if *validateUsed {
		fmt.Printf("Validating: %s (strict: %t)\n", *validateInput, *strict)
		return
	}

	// Main processing
	fmt.Printf("Processing %s -> %s format (timeout: %ds)\n",
		*input, *format, *timeout)

	if *verbose {
		fmt.Println("Verbose mode enabled")
	} else if *quiet {
		fmt.Println("Quiet mode enabled")
	}

	if len(*tags) > 0 {
		fmt.Printf("Tags: %v\n", *tags)
	}
}
```

**Usage examples:**

```bash
# All equivalent ways to call the same thing:
fileproc data.txt                           # positional
fileproc --input data.txt                   # flag style
fileproc data.txt --format xml -v          # mixed style
fileproc data.txt xml 60 --tags api,prod   # mostly positional

# Subcommand
fileproc validate config.json --strict

# Auto-generated help
fileproc --help
```

## Documentation

- **[API Docs](https://pkg.go.dev/github.com/amterp/ra)** - Full API reference

## Requirements

- Go 1.24+

## License

[MIT License.](./LICENSE)

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for development guidelines.
