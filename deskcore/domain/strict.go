package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
)

// RequiredKeys is implemented by a struct type whose JSON object must carry certain keys. A
// decoder fills an absent field with its zero value, so presence is checked on the document
// itself: an absent or null required key is refused, never read as a proven zero.
type RequiredKeys interface {
	RequiredKeys() []string
}

var (
	requiredKeysType = reflect.TypeFor[RequiredKeys]()
	unmarshalerType  = reflect.TypeFor[json.Unmarshaler]()
)

// DecodeStrict decodes data, which must hold exactly one JSON value, into v (a non-nil
// pointer) under the exact JSON contract of v's type. It is the one place deskcore decodes a
// document it did not write.
//
// encoding/json on its own matches an object key to a struct field without regard to case, and
// keeps the last of two equal keys. A document that a JSON Schema validator refuses (a key that
// differs only in case, or a key given twice) could then decode to a different value than the
// contract describes. DecodeStrict refuses both, at every depth:
//
//   - every key of an object decoded into a struct must equal a field's JSON name exactly;
//   - no object, at any depth, may carry the same key twice;
//   - a struct type that implements RequiredKeys must carry each of those keys, and not as null;
//   - nothing may follow the value.
//
// Only after those checks pass does it decode, with unknown fields disallowed. Embedded struct
// fields are not promoted: their keys are refused, which fails closed.
func DecodeStrict(data []byte, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return errors.New("strict decode: the target must be a non-nil pointer")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var errs []error
	if _, err := walk(dec, rv.Type().Elem(), "$", &errs); err != nil {
		return fmt.Errorf("strict decode: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("strict decode: trailing data after the value")
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	full := json.NewDecoder(bytes.NewReader(data))
	full.DisallowUnknownFields()
	if err := full.Decode(v); err != nil {
		return fmt.Errorf("strict decode: %w", err)
	}
	return nil
}

// walk consumes one JSON value from dec and checks it against t, reporting whether the value
// was null. A syntax error is returned; contract violations are collected in errs so that every
// one is reported.
func walk(dec *json.Decoder, t reflect.Type, at string, errs *[]error) (bool, error) {
	tok, err := dec.Token()
	if err != nil {
		return false, err
	}
	if tok == nil {
		return true, nil
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	delim, isDelim := tok.(json.Delim)
	opaque := t.Implements(unmarshalerType) || reflect.PointerTo(t).Implements(unmarshalerType)
	switch {
	case opaque || !isDelim:
		// A scalar, or a type that decodes itself: still refuse duplicate keys inside it.
		return false, walkAny(dec, tok, at, errs)
	case t.Kind() == reflect.Struct && delim == '{':
		return false, walkStruct(dec, t, at, errs)
	case (t.Kind() == reflect.Slice || t.Kind() == reflect.Array) && delim == '[':
		for i := 0; dec.More(); i++ {
			if _, err := walk(dec, t.Elem(), fmt.Sprintf("%s[%d]", at, i), errs); err != nil {
				return false, err
			}
		}
		_, err := dec.Token()
		return false, err
	case t.Kind() == reflect.Map && delim == '{':
		seen := map[string]bool{}
		for dec.More() {
			key, err := objectKey(dec, at, seen, errs)
			if err != nil {
				return false, err
			}
			if _, err := walk(dec, t.Elem(), at+"."+key, errs); err != nil {
				return false, err
			}
		}
		_, err := dec.Token()
		return false, err
	default:
		// A type mismatch, which the full decode reports.
		return false, walkAny(dec, tok, at, errs)
	}
}

func walkStruct(dec *json.Decoder, t reflect.Type, at string, errs *[]error) error {
	fields := jsonFields(t)
	seen := map[string]bool{}
	null := map[string]bool{}
	for dec.More() {
		key, err := objectKey(dec, at, seen, errs)
		if err != nil {
			return err
		}
		ft, ok := fields[key]
		if !ok {
			*errs = append(*errs, fmt.Errorf("%s: key %q is not a field of this object (keys are case-sensitive)", at, key))
			if err := walkAny(dec, nil, at+"."+key, errs); err != nil {
				return err
			}
			continue
		}
		isNull, err := walk(dec, ft, at+"."+key, errs)
		if err != nil {
			return err
		}
		null[key] = isNull
	}
	if _, err := dec.Token(); err != nil {
		return err
	}
	if t.Implements(requiredKeysType) {
		for _, k := range reflect.Zero(t).Interface().(RequiredKeys).RequiredKeys() {
			if !seen[k] || null[k] {
				*errs = append(*errs, fmt.Errorf("%s: required field %q is missing", at, k))
			}
		}
	}
	return nil
}

// walkAny consumes one value whose first token is tok (read from dec when tok is nil),
// refusing duplicate keys in any object inside it.
func walkAny(dec *json.Decoder, tok json.Token, at string, errs *[]error) error {
	if tok == nil {
		var err error
		if tok, err = dec.Token(); err != nil {
			return err
		}
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			key, err := objectKey(dec, at, seen, errs)
			if err != nil {
				return err
			}
			if err := walkAny(dec, nil, at+"."+key, errs); err != nil {
				return err
			}
		}
	case '[':
		for i := 0; dec.More(); i++ {
			if err := walkAny(dec, nil, fmt.Sprintf("%s[%d]", at, i), errs); err != nil {
				return err
			}
		}
	}
	_, err := dec.Token()
	return err
}

// objectKey reads the next object key, recording it in seen and refusing a repeat.
func objectKey(dec *json.Decoder, at string, seen map[string]bool, errs *[]error) (string, error) {
	tok, err := dec.Token()
	if err != nil {
		return "", err
	}
	key, ok := tok.(string)
	if !ok {
		return "", fmt.Errorf("%s: object key %v is not a string", at, tok)
	}
	if seen[key] {
		*errs = append(*errs, fmt.Errorf("%s: key %q is given twice", at, key))
	}
	seen[key] = true
	return key, nil
}

// jsonFields maps each JSON name of t's exported fields to the field's type, named as
// encoding/json names them: the tag's name, or the Go field name when the tag gives none.
func jsonFields(t reflect.Type) map[string]reflect.Type {
	out := map[string]reflect.Type{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() || f.Anonymous {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		out[name] = f.Type
	}
	return out
}
