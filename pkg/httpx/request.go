package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

const maxJSONBody = 8 << 20 // 8 MiB

// Parse binds JSON, form, query, header, and path values to v.
// Fields require explicit source tags; path values take precedence.
func Parse(r *http.Request, v any) error {
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaType == "application/json" || strings.HasSuffix(mediaType, "+json") {
		if err := ParseJSON(r, v); err != nil {
			return err
		}
	}

	if err := ParseForm(r, v); err != nil {
		return err
	}
	if err := ParseQuery(r, v); err != nil {
		return err
	}
	if err := ParseHeaders(r, v); err != nil {
		return err
	}
	return ParsePath(r, v)
}

// ParseJSON decodes a single JSON value. An empty body is allowed.
func ParseJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return nil
	}

	data, err := io.ReadAll(io.LimitReader(r.Body, maxJSONBody+1))
	if err != nil {
		return fmt.Errorf("read JSON body: %w", err)
	}
	if len(data) > maxJSONBody {
		return fmt.Errorf("JSON body exceeds %d bytes", maxJSONBody)
	}
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("decode JSON body: %w", err)
	}

	return nil
}

// ParsePath binds route parameters using path tags and Request.PathValue.
func ParsePath(r *http.Request, v any) error {
	return bind(v, "path", func(name string) []string {
		if value := r.PathValue(name); value != "" {
			return []string{value}
		}
		return nil
	})
}

// ParseQuery binds URL query parameters using query tags.
func ParseQuery(r *http.Request, v any) error {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return fmt.Errorf("parse query: %w", err)
	}
	return bind(v, "query", func(name string) []string { return values[name] })
}

// ParseForm binds query and URL-encoded form values using form tags.
// Multipart form data must be handled separately.
func ParseForm(r *http.Request, v any) error {
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("parse form: %w", err)
	}
	return bind(v, "form", func(name string) []string { return r.Form[name] })
}

// ParseHeaders binds request headers using header tags.
func ParseHeaders(r *http.Request, v any) error {
	return bind(v, "header", r.Header.Values)
}

func bind(v any, tag string, values func(string) []string) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() || rv.Elem().Kind() != reflect.Struct {
		return errors.New("httpx: expected a non-nil pointer to struct")
	}

	rv = rv.Elem()
	rt := rv.Type()
	for i := range rt.NumField() {
		field := rt.Field(i)
		fv := rv.Field(i)
		if !fv.CanSet() {
			continue
		}

		name := field.Tag.Get(tag)
		if name == "" || name == "-" {
			continue
		}
		vs := values(name)
		if len(vs) == 0 {
			continue
		}
		if err := setValue(fv, vs); err != nil {
			return fmt.Errorf("%s %q: %w", tag, name, err)
		}
	}

	return nil
}

func setValue(v reflect.Value, values []string) error {
	if v.Kind() != reflect.Slice {
		return setScalar(v, values[0])
	}

	slice := reflect.MakeSlice(v.Type(), len(values), len(values))
	for i, value := range values {
		if err := setScalar(slice.Index(i), value); err != nil {
			return fmt.Errorf("index %d: %w", i, err)
		}
	}
	v.Set(slice)
	return nil
}

func setScalar(v reflect.Value, value string) error {
	switch v.Kind() {
	case reflect.String:
		v.SetString(value)
	case reflect.Bool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		v.SetBool(parsed)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(value, 10, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetInt(parsed)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		parsed, err := strconv.ParseUint(value, 10, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetUint(parsed)
	case reflect.Float32, reflect.Float64:
		parsed, err := strconv.ParseFloat(value, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetFloat(parsed)
	default:
		return fmt.Errorf("unsupported type %s", v.Type())
	}
	return nil
}
