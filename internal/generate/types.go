package generate

// ManifestGenerateOptions describes what the manifest generator should produce.
type ManifestGenerateOptions struct {
	Provider    string // "anthropic", "openai", "gemini"
	Description string // User-provided description of what the toolkit does
	OutputPath  string // Where to write the manifest (empty = stdout for dry-run)
	DryRun      bool   // If true, print to stdout instead of writing
	Model       string // LLM model override; empty = use provider default
	NoMerge     bool   // If true and OutputPath exists, return error instead of overwriting
}

// ManifestGenerateResult holds the output of a manifest generation.
type ManifestGenerateResult struct {
	Manifest string // The generated YAML content
	Provider string // Which provider was used
}
