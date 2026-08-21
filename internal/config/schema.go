package config

import (
	"errors"
	"fmt"

	"go.yaml.in/yaml/v3"
)

type Whole int

func (n *Whole) UnmarshalYAML(node *yaml.Node) error {
	if node.Tag != "!!int" {
		return fmt.Errorf("line %d: must be a whole number, got %s", node.Line, describe(node))
	}

	var v int

	if err := node.Decode(&v); err != nil {
		return err
	}

	*n = Whole(v)

	return nil
}

func describe(node *yaml.Node) string {
	switch node.Tag {
	case "!!str":
		return fmt.Sprintf("the string %q", node.Value)
	case "!!float":
		return fmt.Sprintf("the decimal %s", node.Value)
	case "!!bool":
		return fmt.Sprintf("the boolean %s", node.Value)
	case "!!null":
		return "nothing"
	case "!!seq":
		return "a list"
	case "!!map":
		return "a mapping"
	}

	return node.Value
}

type SensitiveType string

const (
	TypeUnspecified SensitiveType = ""
	TypeNumeric     SensitiveType = "numeric"
	TypeCategorical SensitiveType = "categorical"
)

var sensitiveTypes = []SensitiveType{TypeNumeric, TypeCategorical}

type Sensitive struct {
	Name string        `yaml:"name"`
	Type SensitiveType `yaml:"type,omitempty"`
}

type ColumnList []string

func (l *ColumnList) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.SequenceNode {
		return errors.New("quasi_identifiers: expected a list of column names")
	}

	out := make([]string, 0, len(node.Content))

	for i, item := range node.Content {
		if item.Kind != yaml.ScalarNode || item.Tag != "!!str" {
			return fmt.Errorf(
				"quasi_identifiers: entry %d is not a column name: %s (quote it if the column really is named that)",
				i, item.Value)
		}

		out = append(out, item.Value)
	}

	*l = out

	return nil
}

type SensitiveList []Sensitive

func (l *SensitiveList) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.SequenceNode {
		return errors.New("sensitive: expected a list of columns")
	}

	out := make([]Sensitive, 0, len(node.Content))

	for i, item := range node.Content {
		if item.Tag == "!!null" {
			return fmt.Errorf("sensitive: entry %d is empty", i)
		}

		var entry Sensitive

		if err := item.Decode(&entry); err != nil {
			return err
		}

		out = append(out, entry)
	}

	*l = out

	return nil
}

func (s *Sensitive) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		if node.Tag != "!!str" {
			return fmt.Errorf("sensitive: %s is not a column name", node.Value)
		}

		return node.Decode(&s.Name)
	}

	if node.Kind != yaml.MappingNode {
		return errors.New("sensitive: expected a column name or a mapping with name and type")
	}

	seen := make(map[string]bool, len(node.Content)/2)

	for i := 0; i+1 < len(node.Content); i += 2 {
		key, val := node.Content[i].Value, node.Content[i+1]

		if seen[key] {
			return fmt.Errorf("sensitive: mapping key %q already defined", key)
		}

		seen[key] = true

		switch key {
		case "name":
			if val.Tag != "!!str" {
				return fmt.Errorf("sensitive: name %s is not a column name", val.Value)
			}

			if err := val.Decode(&s.Name); err != nil {
				return fmt.Errorf("sensitive: name: %w", err)
			}
		case "type":
			if err := val.Decode(&s.Type); err != nil {
				return fmt.Errorf("sensitive: type: %w", err)
			}
		default:
			return fmt.Errorf("sensitive: field %s not found", key)
		}
	}

	return nil
}
