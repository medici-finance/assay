package reviewflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"
)

// DecodeObservation accepts one closed v1 JSON value, including explicit false,
// zero, and empty collections. Missing/null fields are not silently defaulted.
// Call PlanObservation with a caller-owned policy before using its contents.
func DecodeObservation(data []byte) (Observation, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	value, err := readJSONValue(d)
	if err != nil {
		return Observation{}, invalid("invalid observation JSON", err)
	}
	if _, err = d.Token(); err != io.EOF {
		return Observation{}, invalid("trailing observation JSON", err)
	}
	if err = requiredShape(value, reflect.TypeOf(Observation{}), "observation"); err != nil {
		return Observation{}, invalid("invalid observation shape", err)
	}
	var o Observation
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err = d.Decode(&o); err != nil {
		return Observation{}, invalid("invalid observation fields", err)
	}
	if o.Schema != ObservationSchema {
		return Observation{}, invalid("unsupported review observation schema", nil)
	}
	return o, nil
}
func readJSONValue(d *json.Decoder) (any, error) {
	token, err := d.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	switch delim {
	case '{':
		out := map[string]any{}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return nil, err
			}
			key, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("object key is not a string")
			}
			if _, ok = out[key]; ok {
				return nil, fmt.Errorf("duplicate field %q", key)
			}
			v, err := readJSONValue(d)
			if err != nil {
				return nil, err
			}
			out[key] = v
		}
		_, err := d.Token()
		return out, err
	case '[':
		out := []any{}
		for d.More() {
			v, err := readJSONValue(d)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		_, err := d.Token()
		return out, err
	default:
		return nil, fmt.Errorf("unexpected delimiter")
	}
}
func requiredShape(v any, t reflect.Type, path string) error {
	if v == nil {
		return fmt.Errorf("%s must not be null", path)
	}
	if t == reflect.TypeOf(time.Time{}) {
		return nil
	}
	switch t.Kind() {
	case reflect.Struct:
		obj, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("%s must be an object", path)
		}
		fields := map[string]bool{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if name == "" {
				name = f.Name
			}
			fields[name] = true
			item, ok := obj[name]
			if !ok {
				return fmt.Errorf("%s.%s is required", path, name)
			}
			if err := requiredShape(item, f.Type, path+"."+name); err != nil {
				return err
			}
		}
		for name := range obj {
			if !fields[name] {
				return fmt.Errorf("%s.%s is unknown", path, name)
			}
		}
	case reflect.Slice:
		items, ok := v.([]any)
		if !ok {
			return fmt.Errorf("%s must be an array", path)
		}
		for _, item := range items {
			if err := requiredShape(item, t.Elem(), path+"[]"); err != nil {
				return err
			}
		}
	case reflect.Map:
		items, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("%s must be an object", path)
		}
		for k, item := range items {
			if err := requiredShape(item, t.Elem(), path+"."+k); err != nil {
				return err
			}
		}
	}
	return nil
}
