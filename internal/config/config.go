package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

type Config struct {
	Datasets []Dataset `yaml:"datasets"`

	dir string
}

type Dataset struct {
	Name             string        `yaml:"name"`
	Source           string        `yaml:"source,omitempty"`
	QuasiIdentifiers ColumnList    `yaml:"quasi_identifiers"`
	Sensitive        SensitiveList `yaml:"sensitive,omitempty"`
	Delimiter        string        `yaml:"delimiter,omitempty"`

	Suppression *string `yaml:"suppression,omitempty"`

	Thresholds Thresholds `yaml:"thresholds"`
}

type Thresholds struct {
	K Whole `yaml:"k"`

	L Whole `yaml:"l,omitempty"`

	T float64 `yaml:"t,omitempty"`
}

func (d Dataset) SensitiveKinds() ([]SensitiveType, bool) {
	out := make([]SensitiveType, len(d.Sensitive))

	complete := len(d.Sensitive) > 0

	for i, s := range d.Sensitive {
		out[i] = s.Type

		if s.Type == TypeUnspecified {
			complete = false
		}
	}

	return out, complete
}

func (d Dataset) SensitiveNames() []string {
	out := make([]string, len(d.Sensitive))

	for i, s := range d.Sensitive {
		out[i] = s.Name
	}

	return out
}

func (d Dataset) Delim() rune {
	if d.Delimiter == "" {
		return ','
	}

	r, _ := utf8.DecodeRuneInString(d.Delimiter)

	return r
}

func (c *Config) SourcePath(d Dataset) string {
	if d.Source == "" || d.Source == Stdin || c.dir == "" || filepath.IsAbs(d.Source) {
		return d.Source
	}

	return filepath.Join(c.dir, d.Source)
}

const Stdin = "-"

var ErrNoDatasets = errors.New("config: no datasets defined")

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)

	if err != nil {
		if os.IsNotExist(err) {
			return nil, &NotFoundError{Path: path, Err: err}
		}

		return nil, fmt.Errorf("config: reading %s: %w", path, err)
	}

	cfg, err := Parse(data)

	if err != nil {
		return nil, err
	}

	cfg.dir = filepath.Dir(path)

	return cfg, nil
}

func Parse(data []byte) (*Config, error) {
	expanded, err := expandEnv(data)

	if err != nil {
		return nil, err
	}

	var cfg Config

	dec := yaml.NewDecoder(bytes.NewReader(expanded))
	dec.KnownFields(true)

	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("config: parsing config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Dataset(name string) (Dataset, bool) {
	for _, d := range c.Datasets {
		if d.Name == name {
			return d, true
		}
	}

	return Dataset{}, false
}
