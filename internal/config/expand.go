package config

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

var envRef = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::-([^}]*))?}`)

func expandEnv(data []byte) ([]byte, error) {
	if !bytes.Contains(data, []byte("${")) {
		return data, nil
	}

	var doc yaml.Node

	if err := yaml.Unmarshal(data, &doc); err != nil {

		return nil, fmt.Errorf(
			"config: parsing config: %w (a ${VAR} reference inside {braces} must be quoted)", err)
	}

	if doc.Kind == 0 {
		return data, nil
	}

	var missing []string

	expandNode(&doc, &missing)

	if len(missing) == 1 {
		return nil, fmt.Errorf("config: %s is not set and has no default", missing[0])
	}

	if len(missing) > 1 {
		return nil, fmt.Errorf("config: %s are not set and have no default", strings.Join(missing, ", "))
	}

	out, err := yaml.Marshal(&doc)

	if err != nil {
		return nil, fmt.Errorf("config: expanding the environment: %w", err)
	}

	return out, nil
}

func expandNode(n *yaml.Node, missing *[]string) {
	if n.Kind == yaml.ScalarNode && strings.Contains(n.Value, "${") {
		expanded, ok := expandString(n.Value, missing)

		if ok && expanded != n.Value {
			n.Value = expanded

			n.Tag = ""
			n.Style = 0
		}
	}

	for _, c := range n.Content {
		expandNode(c, missing)
	}
}

func expandString(s string, missing *[]string) (string, bool) {
	matches := envRef.FindAllStringSubmatchIndex(s, -1)

	if matches == nil {
		return s, false
	}

	var (
		b    strings.Builder
		last int
	)

	for _, m := range matches {
		b.WriteString(s[last:m[0]])
		last = m[1]

		name := s[m[2]:m[3]]

		if v, ok := os.LookupEnv(name); ok {
			b.WriteString(v)

			continue
		}

		if m[4] >= 0 {
			b.WriteString(s[m[4]:m[5]])

			continue
		}

		if !slices.Contains(*missing, name) {
			*missing = append(*missing, name)
		}
	}

	b.WriteString(s[last:])

	return b.String(), true
}
