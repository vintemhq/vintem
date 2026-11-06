package main

import (
	"runtime/debug"
	"testing"
)

func TestResolverVersao(t *testing.T) {
	modulo := func(v string) *debug.BuildInfo {
		return &debug.BuildInfo{Main: debug.Module{Version: v}}
	}
	casos := []struct {
		nome     string
		injetada string
		info     *debug.BuildInfo
		quer     string
	}{
		{"go install usa a versão do módulo", "dev", modulo("v0.9.0"), "v0.9.0"},
		{"build local sem tag continua dev", "dev", modulo("(devel)"), "dev"},
		{"sem build info continua dev", "dev", nil, "dev"},
		{"release injetado prevalece", "v1.2.3", modulo("v0.9.0"), "v1.2.3"},
	}
	for _, c := range casos {
		if got := resolverVersao(c.injetada, c.info); got != c.quer {
			t.Errorf("%s: resolverVersao(%q) = %q, quer %q", c.nome, c.injetada, got, c.quer)
		}
	}
}
