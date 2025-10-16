package server

import "embed"

//go:embed templates/*
var TemplatesFS embed.FS
