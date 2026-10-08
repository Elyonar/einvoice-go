// Package schemaexample builds an example value from an OpenAPI schema of the API-key snapshot
// (scripts/openapi-api-key-ops.json), the way einvoice-js's guides exporter does: `example`, then
// `default`, then the first enum value, then the first branch of oneOf/anyOf, the merged allOf, and
// for an object its required properties in the schema's order. Used by scripts/export_guides (the
// curl bodies) and scripts/check_operations (the sample values a rendered snippet is compiled with).
// Not shipped.
package schemaexample

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// OrderedMap keeps the schema's `required` order in the rendered example, as JSON.stringify would.
type OrderedMap []KV

// KV is one property of an OrderedMap.
type KV struct {
	Key   string
	Value any
}

// MarshalJSON writes the properties in order.
func (m OrderedMap) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, e := range m {
		if i > 0 {
			buf.WriteByte(',')
		}
		k, _ := json.Marshal(e.Key)
		buf.Write(k)
		buf.WriteByte(':')
		v, err := Marshal(e.Value, "")
		if err != nil {
			return nil, err
		}
		buf.Write(v)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// Marshal encodes without HTML escaping; indent "" compacts, otherwise indents.
func Marshal(v any, indent string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if indent != "" {
		enc.SetIndent("", indent)
	}
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// Builder resolves `$ref`s against the snapshot's schemas.
type Builder struct {
	Schemas map[string]map[string]any
}

// MissingSchemaError is a `$ref` the snapshot does not define.
type MissingSchemaError struct{ Name string }

func (e *MissingSchemaError) Error() string {
	return fmt.Sprintf("schema %s is not in the snapshot", e.Name)
}

// Of is an example value of schema (nil for an empty schema).
func (b Builder) Of(schema map[string]any) (value any, err error) {
	defer func() {
		if r := recover(); r != nil {
			missing, ok := r.(*MissingSchemaError)
			if !ok {
				panic(r)
			}
			value, err = nil, missing
		}
	}()
	return b.of(schema, 0), nil
}

func (b Builder) resolveRef(ref string) map[string]any {
	name := strings.TrimPrefix(ref, "#/components/schemas/")
	schema, ok := b.Schemas[name]
	if !ok {
		panic(&MissingSchemaError{name})
	}
	return schema
}

func list(v any) []any {
	l, _ := v.([]any)
	return l
}

func (b Builder) of(schema map[string]any, depth int) any {
	if schema == nil || depth > 8 {
		return nil
	}
	if ref, ok := schema["$ref"].(string); ok {
		return b.of(b.resolveRef(ref), depth+1)
	}
	if ex, ok := schema["example"]; ok {
		return ex
	}
	if def, ok := schema["default"]; ok {
		return def
	}
	if enum := list(schema["enum"]); len(enum) > 0 {
		return enum[0]
	}
	if one := list(schema["oneOf"]); len(one) > 0 {
		m, _ := one[0].(map[string]any)
		return b.of(m, depth+1)
	}
	if anyOf := list(schema["anyOf"]); len(anyOf) > 0 {
		m, _ := anyOf[0].(map[string]any)
		return b.of(m, depth+1)
	}
	if all := list(schema["allOf"]); len(all) > 0 {
		var parts []any
		for _, s := range all {
			m, _ := s.(map[string]any)
			parts = append(parts, b.of(m, depth+1))
		}
		merged := OrderedMap{}
		for _, p := range parts {
			om, ok := p.(OrderedMap)
			if !ok {
				return parts[0]
			}
			merged = append(merged, om...)
		}
		return merged
	}
	kind, _ := schema["type"].(string)
	switch kind {
	case "object":
		props, _ := schema["properties"].(map[string]any)
		out := OrderedMap{}
		for _, name := range list(schema["required"]) {
			n, _ := name.(string)
			p, _ := props[n].(map[string]any)
			out = append(out, KV{n, b.of(p, depth+1)})
		}
		return out
	case "array":
		items, _ := schema["items"].(map[string]any)
		return []any{b.of(items, depth+1)}
	case "integer", "number":
		if min, ok := schema["minimum"]; ok {
			return min
		}
		return 1
	case "boolean":
		return false
	case "string":
		switch schema["format"] {
		case "uuid":
			return "00000000-0000-7000-8000-000000000000"
		case "date":
			return "2026-10-05"
		case "date-time":
			return "2026-10-05T12:00:00Z"
		}
		return "string"
	}
	if _, ok := schema["properties"]; ok {
		copied := map[string]any{}
		for k, v := range schema {
			copied[k] = v
		}
		copied["type"] = "object"
		return b.of(copied, depth)
	}
	return nil
}
