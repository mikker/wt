package main

import "strings"

// stripEnv returns env with any entries for key removed. Environment names
// are case-insensitive on Windows, so normalize that way on every platform.
func stripEnv(env []string, key string) []string {
	out := make([]string, 0, len(env))
	for _, variable := range env {
		name, _, _ := strings.Cut(variable, "=")
		if strings.EqualFold(name, key) {
			continue
		}
		out = append(out, variable)
	}
	return out
}

func setEnv(env []string, key, value string) []string {
	return append(stripEnv(env, key), key+"="+value)
}
