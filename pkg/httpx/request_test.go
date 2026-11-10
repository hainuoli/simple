package httpx

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	t.Run("JSON and tagged sources", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/users/42?page=3", strings.NewReader(`{"name":"Ada"}`))
		r.Header.Set("Content-Type", "application/json; charset=utf-8")
		r.Header.Set("X-Trace-ID", "trace-1")
		r.SetPathValue("id", "42")

		var got struct {
			Name  string `json:"name"`
			Page  int    `query:"page"`
			Trace string `header:"X-Trace-ID"`
			ID    int    `path:"id"`
		}
		require.NoError(t, Parse(r, &got))
		require.Equal(t, "Ada", got.Name)
		require.Equal(t, 3, got.Page)
		require.Equal(t, "trace-1", got.Trace)
		require.Equal(t, 42, got.ID)
	})

	t.Run("structured JSON media type", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Grace"}`))
		r.Header.Set("Content-Type", "application/problem+json")
		var got struct {
			Name string `json:"name"`
		}
		require.NoError(t, Parse(r, &got))
		require.Equal(t, "Grace", got.Name)
	})

	t.Run("URL-encoded form", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("name=Lin&name=Ada"))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		var got struct {
			Names []string `form:"name"`
		}
		require.NoError(t, Parse(r, &got))
		require.Equal(t, []string{"Lin", "Ada"}, got.Names)
	})

	t.Run("non-JSON body", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"id":5}`))
		r.Header.Set("Content-Type", "text/plain")
		var got struct {
			ID int `json:"id"`
		}
		require.NoError(t, Parse(r, &got))
		require.Zero(t, got.ID)
	})

	t.Run("path overrides JSON", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/users/real", strings.NewReader(`{"id":"spoofed"}`))
		r.Header.Set("Content-Type", "application/json")
		r.SetPathValue("id", "real")
		var got struct {
			ID string `json:"id" path:"id"`
		}
		require.NoError(t, Parse(r, &got))
		require.Equal(t, "real", got.ID)
	})
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name  string
		setup func() *http.Request
		want  string
	}{
		{
			name: "JSON",
			setup: func() *http.Request {
				r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{"))
				r.Header.Set("Content-Type", "application/json")
				return r
			},
			want: "decode JSON body",
		},
		{
			name: "form",
			setup: func() *http.Request {
				r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("name=%ZZ"))
				r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				return r
			},
			want: "parse form",
		},
		{
			name: "query",
			setup: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				r.URL.RawQuery = "page=%ZZ"
				r.Form = url.Values{}
				return r
			},
			want: "parse query",
		},
		{
			name: "headers",
			setup: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				r.Header.Set("X-Enabled", "invalid")
				return r
			},
			want: `header "X-Enabled"`,
		},
		{
			name: "path",
			setup: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/users/not-a-number", nil)
				r.SetPathValue("id", "not-a-number")
				return r
			},
			want: `path "id"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got struct {
				ID      int  `path:"id"`
				Enabled bool `header:"X-Enabled"`
			}
			require.ErrorContains(t, Parse(tt.setup(), &got), tt.want)
		})
	}
}

func TestParseJSON(t *testing.T) {
	tests := []struct {
		name    string
		body    io.Reader
		want    string
		wantErr string
	}{
		{"empty", strings.NewReader(""), "", ""},
		{"whitespace", strings.NewReader("   "), "", "decode JSON body"},
		{"valid", strings.NewReader(`{"name":"Ada"}`), "Ada", ""},
		{"trailing whitespace", strings.NewReader("{\"name\":\"Grace\"}\n\t"), "Grace", ""},
		{"unknown length", strings.NewReader(`{"name":"Lin"}`), "Lin", ""},
		{"multiple JSON values", strings.NewReader(`{"name":"Ada"}{"name":"Eve"}`), "", "decode JSON body"},
		{"invalid JSON", strings.NewReader(`{"name":`), "", "decode JSON body"},
		{"oversized", strings.NewReader(strings.Repeat("x", maxJSONBody+1)), "", "exceeds"},
		{"read error", errReader{}, "", "read JSON body"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", tt.body)
			if tt.name == "unknown length" {
				r.ContentLength = -1
			}
			if tt.name == "empty" {
				r.ContentLength = 0
			}

			var got struct {
				Name string `json:"name"`
			}
			err := ParseJSON(r, &got)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got.Name)
		})
	}

	t.Run("nil body", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Body = nil
		require.NoError(t, ParseJSON(r, new(struct{})))
	})

	t.Run("zero ContentLength with body", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Ada"}`))
		r.ContentLength = 0
		var got struct {
			Name string `json:"name"`
		}
		require.NoError(t, ParseJSON(r, &got))
		require.Equal(t, "Ada", got.Name)
	})
}

func TestParsePath(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/orgs/42", nil)
	r.SetPathValue("org", "42")
	got := struct {
		Org     int    `path:"org"`
		Missing string `path:"missing"`
	}{Missing: "unchanged"}

	require.NoError(t, ParsePath(r, &got))
	require.Equal(t, 42, got.Org)
	require.Equal(t, "unchanged", got.Missing)

	r.SetPathValue("org", "bad")
	require.ErrorContains(t, ParsePath(r, &got), `path "org"`)
	require.ErrorContains(t, ParsePath(r, nil), "non-nil pointer")
}

func TestParseQuery(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?page=2&names=a&names=b", nil)
	var got struct {
		Page  int      `query:"page"`
		Names []string `query:"names"`
	}

	require.NoError(t, ParseQuery(r, &got))
	require.Equal(t, 2, got.Page)
	require.Equal(t, []string{"a", "b"}, got.Names)

	r.URL.RawQuery = "page=%GG"
	require.ErrorContains(t, ParseQuery(r, &got), "parse query")
	r.URL.RawQuery = "page=invalid"
	require.ErrorContains(t, ParseQuery(r, &got), `query "page"`)
}

func TestParseForm(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/?page=1", strings.NewReader("name=Ada&name=Lin"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var got struct {
		Name []string `form:"name"`
		Page int      `form:"page"`
	}

	require.NoError(t, ParseForm(r, &got))
	require.Equal(t, 1, got.Page)
	require.Equal(t, []string{"Ada", "Lin"}, got.Name)

	r = httptest.NewRequest(http.MethodPost, "/", strings.NewReader("name=%XX"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	require.ErrorContains(t, ParseForm(r, &got), "parse form")

	r = httptest.NewRequest(http.MethodGet, "/?page=invalid", nil)
	require.ErrorContains(t, ParseForm(r, &got), `form "page"`)
}

func TestParseHeaders(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Request-ID", "123")
	var got struct {
		ID int `header:"X-Request-ID"`
	}

	require.NoError(t, ParseHeaders(r, &got))
	require.Equal(t, 123, got.ID)

	r.Header.Set("X-Request-ID", "bad")
	require.ErrorContains(t, ParseHeaders(r, &got), `header "X-Request-ID"`)
}

func TestBind(t *testing.T) {
	values := func(string) []string { return []string{"value"} }
	invalid := []struct {
		name string
		v    any
	}{
		{"nil", nil},
		{"non-pointer", struct{}{}},
		{"nil pointer", (*struct{})(nil)},
		{"pointer to scalar", new(int)},
	}

	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorContains(t, bind(tt.v, "query", values), "non-nil pointer to struct")
		})
	}

	got := struct {
		private  string `query:"secret"`
		Untagged string
		Skipped  string `query:"-"`
		Absent   string `query:"absent"`
		Value    string `query:"value"`
	}{
		private:  "private",
		Untagged: "unchanged",
		Skipped:  "unchanged",
		Absent:   "unchanged",
	}

	require.NoError(t, bind(&got, "query", func(name string) []string {
		if name == "value" || name == "secret" || name == "-" {
			return []string{"value"}
		}
		return nil
	}))
	require.Equal(t, "private", got.private)
	require.Equal(t, "unchanged", got.Untagged)
	require.Equal(t, "unchanged", got.Skipped)
	require.Equal(t, "unchanged", got.Absent)
	require.Equal(t, "value", got.Value)

	bad := struct {
		Value int `query:"value"`
	}{}
	require.ErrorContains(t, bind(&bad, "query", values), `query "value"`)
}

func TestSetValue(t *testing.T) {
	t.Run("scalar", func(t *testing.T) {
		var value string
		require.NoError(t, setValue(reflect.ValueOf(&value).Elem(), []string{"first", "second"}))
		require.Equal(t, "first", value)
	})

	t.Run("slice", func(t *testing.T) {
		var value []int
		require.NoError(t, setValue(reflect.ValueOf(&value).Elem(), []string{"1", "2"}))
		require.Equal(t, []int{1, 2}, value)
	})

	t.Run("slice conversion failure is atomic", func(t *testing.T) {
		value := []int{9}
		require.ErrorContains(t, setValue(reflect.ValueOf(&value).Elem(), []string{"1", "bad"}), "index 1")
		require.Equal(t, []int{9}, value)
	})
}

func TestSetScalar(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		input   string
		want    any
		wantErr bool
	}{
		{"string", new(string), "Ada", "Ada", false},
		{"bool true", new(bool), "true", true, false},
		{"bool invalid", new(bool), "maybe", false, true},
		{"int", new(int), "-12", int(-12), false},
		{"int8", new(int8), "127", int8(127), false},
		{"int16", new(int16), "20", int16(20), false},
		{"int32", new(int32), "20", int32(20), false},
		{"int64", new(int64), "20", int64(20), false},
		{"int overflow", new(int8), "128", int8(0), true},
		{"int invalid", new(int), "no", int(0), true},
		{"uint", new(uint), "12", uint(12), false},
		{"uint8", new(uint8), "255", uint8(255), false},
		{"uint16", new(uint16), "20", uint16(20), false},
		{"uint32", new(uint32), "20", uint32(20), false},
		{"uint64", new(uint64), "20", uint64(20), false},
		{"uint overflow", new(uint8), "256", uint8(0), true},
		{"uint negative", new(uint), "-1", uint(0), true},
		{"float32", new(float32), "1.25", float32(1.25), false},
		{"float64", new(float64), "2.5", float64(2.5), false},
		{"float invalid", new(float64), "no", float64(0), true},
		{"unsupported struct", new(struct{}), "x", struct{}{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := reflect.ValueOf(tt.value).Elem()
			err := setScalar(value, tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, value.Interface())
		})
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}
