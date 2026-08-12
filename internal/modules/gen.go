package modules

// The gen_*.go descriptors and manifest_generated.go in this package are
// generated from the committed specifications in specs/ and carry a DO NOT EDIT
// header. Regenerate them after any change to specs/, to the generator, or to the
// legacy overlay:
//
//	./scripts/dev.sh gen        (Windows: .\scripts\dev.ps1 gen)
//
// or directly:
//
//	go generate ./internal/modules/...
//
// Per-object behaviour a descriptor cannot express belongs in overrides.go, which
// is hand-written and never generated.
//
//go:generate go run ../../tools/semp-gen -software ../../specs/software/10.26.0.8755/semp-v2-config.json -appliance ../../specs/appliance/10.26.0.8755/semp-v2-config.json -out .
