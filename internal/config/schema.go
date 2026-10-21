package config

import (
	"bytes"
	"encoding/json"
)

// SchemaBaseURL is where the committed schema/ directory is served from. Editors fetch
// the schema from there (see docs/host.md for the `#:schema` line to put in a config).
const SchemaBaseURL = "https://raw.githubusercontent.com/dhruvinsh/panoptes/main/schema/"

// Schema file name inside schema/.
const ConfigSchemaFile = "config.schema.json"

const schemaDraft = "http://json-schema.org/draft-07/schema#"

// jobj is a JSON object that keeps its key order, so the generated schema reads in a
// sensible order (and diffs stay small).
type jobj []jkv

type jkv struct {
	k string
	v any
}

func (o jobj) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, e := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		k, err := marshal(e.k)
		if err != nil {
			return nil, err
		}
		v, err := marshal(e.v)
		if err != nil {
			return nil, err
		}
		b.Write(k)
		b.WriteByte(':')
		b.Write(v)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

func marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

func render(o jobj) []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(o); err != nil {
		panic(err) // only strings, numbers and slices go in
	}
	return b.Bytes()
}

func (o *jobj) add(k string, v any) { *o = append(*o, jkv{k, v}) }

func strs(list []string) []any {
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = s
	}
	return out
}

func optionSchema(o Option) jobj {
	s := jobj{}
	s.add("type", o.Type)
	s.add("description", o.Doc)
	if o.Default != nil {
		s.add("default", o.Default)
	}
	if o.Min != nil {
		if o.MinExcl {
			s.add("exclusiveMinimum", *o.Min)
		} else {
			s.add("minimum", *o.Min)
		}
	}
	if o.Max != nil {
		s.add("maximum", *o.Max)
	}
	switch o.Type {
	case "array":
		items := jobj{{"type", "string"}}
		if o.Pattern != "" {
			items.add("pattern", o.Pattern)
		}
		if o.Enum != nil {
			items.add("enum", strs(o.Enum))
		}
		s.add("items", items)
		s.add("uniqueItems", true)
		if o.MaxItems > 0 {
			s.add("maxItems", o.MaxItems)
		}
	default:
		if o.Enum != nil {
			s.add("enum", strs(o.Enum))
		}
	}
	return s
}

// ConfigSchema renders schema/config.schema.json.
func ConfigSchema() []byte {
	props := jobj{}
	for _, o := range Options {
		props.add(o.Key, optionSchema(o))
	}
	// Older configs wrap everything in a [panoptes] table; same keys, one level down.
	legacy := append(jobj{}, props...)
	root := jobj{
		{"$schema", schemaDraft},
		{"$id", SchemaBaseURL + ConfigSchemaFile},
		{"title", "Panoptes config"},
		{"description", "Config file for the panoptes host agent (config.toml). Command-line options override it."},
		{"type", "object"},
	}
	root.add("properties", append(append(jobj{}, props...), jkv{"panoptes", jobj{
		{"type", "object"},
		{"description", "Legacy wrapper table: the same keys as the top level."},
		{"deprecated", true},
		{"properties", legacy},
		{"additionalProperties", false},
	}}))
	root.add("additionalProperties", false)
	return render(root)
}
