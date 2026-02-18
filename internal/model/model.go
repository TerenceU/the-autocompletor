// Package model defines the shared data structures used across parsers and generators.
package model

// Flag represents a single CLI flag with its metadata.
type Flag struct {
	Short       string // e.g. "-u"
	Long        string // e.g. "--url"
	Description string
	TakesArg    bool // true if the flag requires a value
}

// Command represents a CLI command (root or subcommand) and its tree.
type Command struct {
	Name        string
	Description string
	Flags       []Flag
	Subcommands []*Command
}
