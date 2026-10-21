package handlers

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
)

func TestDocumentEndpointsAuthAndValidation(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	s := &Server{
		JWTSecret:  secret,
		CORSOrigin: "http://localhost:3000",
	}
	handler := s.Routes()

	// 1. GET /api/documents without token returns 401
	req := httptest.NewRequest(http.MethodGet, "/api/documents", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for GET /api/documents without token, got %d", rec.Code)
	}

	// 2. POST /api/documents without token returns 401
	req = httptest.NewRequest(http.MethodPost, "/api/documents", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for POST /api/documents without token, got %d", rec.Code)
	}

	// 3. GET /api/documents/abc with token returns 400 Bad Request
	token, err := auth.NewAccessToken(secret, 1, "test@example.com", "Test User", "USER")
	if err != nil {
		t.Fatalf("failed to create access token: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/documents/abc", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for GET /api/documents/abc, got %d", rec.Code)
	}

	// 4. GET /api/documents/0 with token returns 400 Bad Request
	req = httptest.NewRequest(http.MethodGet, "/api/documents/0", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for GET /api/documents/0, got %d", rec.Code)
	}

	// 5. POST /api/documents without multipart file field returns 400 Bad Request
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("title", "Test Title")
	_ = writer.Close()

	req = httptest.NewRequest(http.MethodPost, "/api/documents", &body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for POST /api/documents without file, got %d", rec.Code)
	}

	// 6. POST /api/documents with valid multipart upload returns 201 Created
	body.Reset()
	writer = multipart.NewWriter(&body)
	_ = writer.WriteField("title", "Executed Illinois Lease Agreement")
	_ = writer.WriteField("category", "lease")
	_ = writer.WriteField("property_id", "1")
	part, err := writer.CreateFormFile("file", "sample_lease.pdf")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	_, _ = part.Write(GenerateIllinoisLeasePDF())
	_ = writer.Close()

	req = httptest.NewRequest(http.MethodPost, "/api/documents", &body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for valid POST /api/documents, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	// 7. GET /api/documents with token returns 200 OK
	req = httptest.NewRequest(http.MethodGet, "/api/documents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for GET /api/documents, got %d", rec.Code)
	}
}

func TestClassifyMediaType(t *testing.T) {
	tests := []struct {
		filename string
		mimeType string
		expected string
	}{
		{"lease.pdf", "application/pdf", "pdf"},
		{"LEASE.PDF", "binary/octet-stream", "pdf"},
		{"photo.jpg", "image/jpeg", "image"},
		{"building.PNG", "image/png", "image"},
		{"tour.mp4", "video/mp4", "video"},
		{"audio_memo.mp3", "audio/mpeg", "audio"},
		{"notes.m4a", "audio/mp4", "audio"},
		{"rent_roll.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "document"},
		{"contract.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "document"},
		{"archive.tar.gz", "application/gzip", "other"},
	}

	for _, tc := range tests {
		actual := classifyMediaType(tc.filename, tc.mimeType)
		if actual != tc.expected {
			t.Errorf("classifyMediaType(%q, %q) = %q, expected %q", tc.filename, tc.mimeType, actual, tc.expected)
		}
	}
}

func TestDocumentFileAuthMW(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	s := &Server{
		JWTSecret:  secret,
		CORSOrigin: "http://localhost:3000",
	}
	handler := s.Routes()

	// 1. GET /api/documents/1/view without token returns 401
	req := httptest.NewRequest(http.MethodGet, "/api/documents/1/view", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for GET /api/documents/1/view without token, got %d", rec.Code)
	}

	// 2. GET /api/documents/1/download without token returns 401
	req = httptest.NewRequest(http.MethodGet, "/api/documents/1/download", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for GET /api/documents/1/download without token, got %d", rec.Code)
	}

	// 3. GET /api/documents/1/view with ?token= query parameter authenticates (reaches handler: 404 with no DB, not 401)
	token, err := auth.NewAccessToken(secret, 1, "test@example.com", "Test User", "USER")
	if err != nil {
		t.Fatalf("failed to create access token: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/documents/1/view?token="+token, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (authenticated, but document not in mock DB) for GET /api/documents/1/view?token=..., got %d", rec.Code)
	}

	// 4. GET /api/documents/1/download with ?access_token= query parameter authenticates
	req = httptest.NewRequest(http.MethodGet, "/api/documents/1/download?access_token="+token, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (authenticated, but document not in mock DB) for GET /api/documents/1/download?access_token=..., got %d", rec.Code)
	}
}

