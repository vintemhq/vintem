package main

import "runtime/debug"

// resolverVersao devolve a versão do binário. O goreleaser injeta a versão no
// release; quando ela fica "dev" (go install ...@v0.9.0, go run ...@latest), usa
// a versão do módulo que o próprio build info carrega.
func resolverVersao(injetada string, info *debug.BuildInfo) string {
	if injetada != "dev" || info == nil {
		return injetada
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	return injetada
}
