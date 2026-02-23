package http

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestParseIDRejectsNonUUIDV7(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "not-a-uuid"}}

	_, err := parseID(c, "id")
	if err == nil {
		t.Fatalf("expected parseID error")
	}
}

func TestParseIDAcceptsUUIDV7(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuidv7: %v", err)
	}
	c.Params = gin.Params{{Key: "id", Value: id.String()}}

	parsed, err := parseID(c, "id")
	if err != nil {
		t.Fatalf("parseID: %v", err)
	}
	if parsed != id.String() {
		t.Fatalf("expected %s, got %s", id.String(), parsed)
	}
}

func TestParseCursorPaginationRejectsInvalidLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/clinics?limit=0", nil)

	_, _, err := parseCursorPagination(c)
	if err == nil {
		t.Fatalf("expected parseCursorPagination error")
	}
}

func TestParseCursorPaginationRejectsInvalidCursor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/clinics?cursor=invalid", nil)

	_, _, err := parseCursorPagination(c)
	if err == nil {
		t.Fatalf("expected parseCursorPagination error")
	}
}

func TestSetCursorHeadersSetsNextHeadersWhenPresent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/clinics?limit=2", nil)

	next, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuidv7: %v", err)
	}
	nextStr := next.String()
	setCursorHeaders(c, 2, &nextStr)

	if got := c.Writer.Header().Get("X-Page-Limit"); got != "2" {
		t.Fatalf("expected X-Page-Limit=2, got %q", got)
	}
	if got := c.Writer.Header().Get("X-Next-Cursor"); got != nextStr {
		t.Fatalf("expected X-Next-Cursor=%q, got %q", nextStr, got)
	}
	if got := c.Writer.Header().Get("Link"); got == "" {
		t.Fatalf("expected Link header")
	}
}
