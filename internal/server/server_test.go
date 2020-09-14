package server

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestParameters(t *testing.T, method string, uri string, body []byte) (*bytes.Buffer, *httptest.ResponseRecorder, *http.Request) {
	l := logrus.New()
	b := bytes.NewBufferString("")
	l.SetOutput(b)

	w := httptest.NewRecorder()
	ctx := context.WithValue(context.Background(), logCtxKey{}, l)
	r, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("http://localhost/%s", uri), bytes.NewReader(body))
	require.NoError(t, err)

	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	r.Header.Set("User-Agent", "mac")

	return b, w, r
}

func Test_getLogger(t *testing.T) {
	ctx := context.WithValue(context.Background(), logCtxKey{}, logrus.WithError(nil))
	require.NotNil(t, getLogger(ctx))
}

func Test_writeOK(t *testing.T) {
	w := httptest.NewRecorder()
	writeOK(w, http.StatusCreated, struct {
		M int
		N string
	}{
		M: 5,
		N: "str",
	})

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, `{"M":5,"N":"str"}`, w.Body.String())
}

func Test_writeError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusNotFound, "some error")

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, `{"error":"some error"}`, w.Body.String())
}

func Test_writeErrorf(t *testing.T) {
	w := httptest.NewRecorder()
	writeErrorf(w, http.StatusForbidden, "some error %d", 1)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, `{"error":"some error 1"}`, w.Body.String())
}

func Test_writeInternalError(t *testing.T) {
	b, w, r := newTestParameters(t, http.MethodGet, "", nil)

	writeInternalError(getLogger(r.Context()), w, "some error")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Greater(t, len(b.String()), 20) // stacktrace
	assert.True(t, strings.Contains(b.String(), "some error"))
	assert.Equal(t, `{"error":"internal error"}`, w.Body.String())
}
