package run

import "strings"

func Overlay(parent []string, secrets map[string]string) []string {
	seen := map[string]int{}
	out := make([]string, 0, len(parent)+len(secrets))
	for _, kv := range parent {
		k, _, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if _, take := secrets[k]; take {
			continue
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = len(out)
		out = append(out, kv)
	}
	for k, v := range secrets {
		out = append(out, k+"="+v)
	}
	return out
}
