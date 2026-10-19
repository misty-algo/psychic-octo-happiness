package httpreq

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"
)

type testValidator struct {
	validator *validator.Validate
}

func (v *testValidator) Validate(i interface{}) error {
	return v.validator.Struct(i)
}

func TestPathIDParsesPositiveID(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/customers/12", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "12"}})

	id, err := PathID(c, "id", "customer")
	if err != nil {
		t.Fatalf("PathID() error = %v", err)
	}
	if id != 12 {
		t.Fatalf("PathID() = %d, want 12", id)
	}
}

func TestPaginationRejectsInvalidPage(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/customers?page=bad", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_, _, err := Pagination(c)
	if err == nil {
		t.Fatal("Pagination() error = nil, want invalid page error")
	}
}

func TestQueryBoolRejectsInvalidValue(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/products?active_only=maybe", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_, err := QueryBool(c, "active_only", false)
	if err == nil {
		t.Fatal("QueryBool() error = nil, want invalid bool error")
	}
}

func TestBindIDsAcceptsPositiveIDs(t *testing.T) {
	e := echo.New()
	e.Validator = &testValidator{validator: validator.New()}
	req := httptest.NewRequest(http.MethodPost, "/items/batch-delete", strings.NewReader(`{"ids":[3,7,9]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	ids, err := BindIDs(c)
	if err != nil {
		t.Fatalf("BindIDs() error = %v, want nil", err)
	}
	if len(ids) != 3 || ids[0] != 3 || ids[2] != 9 {
		t.Fatalf("BindIDs() = %v, want [3 7 9]", ids)
	}
}

func TestBindIDsRejectsInvalidBodies(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing ids", body: `{}`},
		{name: "empty ids", body: `{"ids":[]}`},
		{name: "non positive id", body: `{"ids":[5,0]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			e.Validator = &testValidator{validator: validator.New()}
			req := httptest.NewRequest(http.MethodPost, "/items/batch-delete", strings.NewReader(tt.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			ids, err := BindIDs(c)
			if err == nil {
				t.Fatalf("BindIDs() error = nil, ids = %v, want validation error", ids)
			}
		})
	}
}
