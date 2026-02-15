package ra

type BaseFlag struct {
	Name              string         // Primary identifier for the flag
	Short             string         // Single character short flag (e.g., 'v' for -v)
	Usage             string         // Help text description shown in usage
	CustomUsageType   string         // Custom type string for usage display (overrides auto-detection)
	Optional          bool           // Whether the flag is optional (default: required)
	Hidden            bool           // Hide from all help output
	HiddenInShortHelp bool           // Hide from short help (-h), show in long help (--help)
	PositionalOnly    bool           // Can only be passed positionally, not as --flag
	FlagOnly          bool           // Can only be passed as --flag, not positionally
	Excludes          *[]string      // Flags that cannot be used with this flag
	Requires          *[]string      // Flags that must be present when this flag is used
	BypassValidation  bool           // If true, this flag can bypass normal validation requirements
	CompletionFunc    CompletionFunc // Custom completion function for shell completion
}
type Flag[T any] struct {
	BaseFlag
	Default *T // Default value when flag is not specified
	Value   *T // Pointer to the parsed value (set during parsing)
}
