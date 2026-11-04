package service

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"
)

func EncodeRequest(q Request) ([]byte, error) {
	if err := ValidateRequest(q); err != nil {
		return nil, err
	}
	return encode(q)
}
func DecodeRequest(data []byte) (Request, error) {
	var q Request
	if err := decode(data, &q); err != nil {
		return Request{}, err
	}
	if err := ValidateRequest(q); err != nil {
		return Request{}, err
	}
	return q, nil
}
func EncodeResponse(r Response, q Request) ([]byte, error) {
	if err := ValidateResponse(r, q); err != nil {
		return nil, err
	}
	return encode(r)
}
func DecodeResponse(data []byte, q Request) (Response, error) {
	var r Response
	if err := decode(data, &r); err != nil {
		return Response{}, err
	}
	if err := ValidateResponse(r, q); err != nil {
		return Response{}, err
	}
	return r, nil
}
func encode(v any) ([]byte, error) {
	// encoding/json replaces invalid UTF-8 in strings. Refuse before marshaling
	// so provider facts are never silently changed; byte content stays opaque.
	if !wireUTF8(reflect.ValueOf(v)) {
		return nil, ErrInvalid
	}
	b, err := json.Marshal(v)
	if err != nil || len(b) > MaxMessageBytes {
		return nil, ErrInvalid
	}
	return b, nil
}

// wireUTF8 checks exported wire text only. It neither normalizes domain values
// nor changes nil/known-empty collections or binary content.
func wireUTF8(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.String:
		return utf8.ValidString(v.String())
	case reflect.Pointer, reflect.Interface:
		return v.IsNil() || wireUTF8(v.Elem())
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if _, _, ok := wireField(v.Type().Field(i)); ok && !wireUTF8(v.Field(i)) {
				return false
			}
		}
	case reflect.Slice, reflect.Array:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return true
		}
		for i := 0; i < v.Len(); i++ {
			if !wireUTF8(v.Index(i)) {
				return false
			}
		}
	case reflect.Map:
		entries := v.MapRange()
		for entries.Next() {
			if entries.Key().Kind() != reflect.String || !text(entries.Key().String(), MaxTextBytes, false) || !wireUTF8(entries.Value()) {
				return false
			}
		}
	}
	return true
}

func decode(data []byte, target any) error {
	if len(data) == 0 || len(data) > MaxMessageBytes || !utf8.Valid(data) {
		return ErrInvalid
	}
	// encoding/json owns syntax and typed decoding. Its token API additionally
	// enforces this protocol's exact names, duplicate refusal and depth/count caps.
	// No domain classifier, schema evaluator or credential parser is copied here.
	d := json.NewDecoder(bytes.NewReader(data))
	if err := closedTokens(d, reflect.TypeOf(target).Elem(), 0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return ErrInvalid
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return ErrInvalid
	}
	return nil
}
func wireField(f reflect.StructField) (name string, required, ok bool) {
	if f.PkgPath != "" {
		return "", false, false
	}
	parts := strings.Split(f.Tag.Get("json"), ",")
	name = parts[0]
	if name == "-" {
		return "", false, false
	}
	if name == "" {
		name = f.Name
	}
	required = true
	for _, option := range parts[1:] {
		if option == "omitempty" {
			required = false
		}
	}
	return name, required, true
}
func fieldType(t reflect.Type, name string) (reflect.Type, bool) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		wire, _, ok := wireField(f)
		if ok && name == wire {
			return f.Type, true
		}
	}
	return nil, false
}
func closedTokens(d *json.Decoder, t reflect.Type, depth int) error {
	if depth > 32 {
		return ErrInvalid
	}
	nullable := t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Map || t.Kind() == reflect.Interface
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	v, err := d.Token()
	if err != nil {
		return ErrInvalid
	}
	delim, aggregate := v.(json.Delim)
	if !aggregate {
		if v == nil && !nullable {
			return ErrInvalid
		}
		return nil // The standard typed decoder checks the scalar's actual type.
	}
	if t == reflect.TypeFor[time.Time]() {
		return ErrInvalid
	}
	switch delim {
	case '{':
		if t.Kind() != reflect.Struct && t.Kind() != reflect.Map {
			return ErrInvalid
		}
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return ErrInvalid
			}
			name, ok := key.(string)
			if !ok || seen[name] || len(seen) >= MaxItems || !text(name, MaxTextBytes, false) {
				return ErrInvalid
			}
			seen[name] = true
			var child reflect.Type
			if t.Kind() == reflect.Map {
				child = t.Elem()
			} else {
				child, ok = fieldType(t, name)
				if !ok {
					return ErrInvalid
				}
			}
			if err := closedTokens(d, child, depth+1); err != nil {
				return err
			}
		}
		if t.Kind() == reflect.Struct {
			for i := 0; i < t.NumField(); i++ {
				name, required, ok := wireField(t.Field(i))
				if ok && required && !seen[name] {
					return ErrInvalid
				}
			}
		}
	case '[':
		if t.Kind() != reflect.Slice || t.Elem().Kind() == reflect.Uint8 {
			return ErrInvalid
		}
		n := 0
		for d.More() {
			if n >= MaxItems {
				return ErrInvalid
			}
			if err := closedTokens(d, t.Elem(), depth+1); err != nil {
				return err
			}
			n++
		}
	default:
		return ErrInvalid
	}
	if _, err := d.Token(); err != nil {
		return ErrInvalid
	}
	return nil
}
