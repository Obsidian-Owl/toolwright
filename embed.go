package toolwright

import "embed"

// Schemas contains the embedded JSON Schema files for manifest validation.
//
//go:embed schemas/*
var Schemas embed.FS

// InitTemplates contains the embedded project template files for init scaffolding.
//
//go:embed all:templates/init
var InitTemplates embed.FS
