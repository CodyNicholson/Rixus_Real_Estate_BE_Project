package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
	"github.com/jackc/pgx/v5"
)

// DocumentItem represents a multimedia file or document metadata response.
type DocumentItem struct {
	ID              int64     `json:"id"`
	UserID          int64     `json:"user_id"`
	PropertyID      *int64    `json:"property_id"`
	PropertyName    *string   `json:"property_name"`
	PropertyAddress *string   `json:"property_address"`
	Title           string    `json:"title"`
	Description     *string   `json:"description"`
	Category        string    `json:"category"`
	MediaType       string    `json:"media_type"`
	FileName        string    `json:"file_name"`
	FileSize        int64     `json:"file_size"`
	MimeType        string    `json:"mime_type"`
	IsTemplate      bool      `json:"is_template"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	DownloadURL     string    `json:"download_url"`
	ViewURL         string    `json:"view_url"`
}

// registerDocumentRoutes registers document and multimedia endpoints.
func (s *Server) registerDocumentRoutes(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	mux.Handle("GET /api/documents", authMW(http.HandlerFunc(s.listDocuments)))
	mux.Handle("POST /api/documents", authMW(http.HandlerFunc(s.uploadDocument)))
	mux.Handle("GET /api/documents/{id}", authMW(http.HandlerFunc(s.getDocument)))
	mux.Handle("GET /api/documents/{id}/download", authMW(http.HandlerFunc(s.downloadDocumentFile)))
	mux.Handle("GET /api/documents/{id}/view", authMW(http.HandlerFunc(s.viewDocumentFile)))
	mux.Handle("DELETE /api/documents/{id}", authMW(http.HandlerFunc(s.deleteDocument)))

	// Property-scoped aliases
	mux.Handle("GET /api/properties/{id}/documents", authMW(http.HandlerFunc(s.listPropertyDocuments)))
	mux.Handle("POST /api/properties/{id}/documents", authMW(http.HandlerFunc(s.uploadPropertyDocument)))
}

// EnsureTemplateDocuments seeds the Illinois 2026 Lease Agreement and Form I-9 if missing.
func (s *Server) EnsureTemplateDocuments(ctx context.Context) error {
	if s.DB == nil {
		return nil
	}

	// Find a user ID to associate template documents with
	var firstUserID int64
	err := s.DB.QueryRow(ctx, `SELECT id FROM users ORDER BY id ASC LIMIT 1`).Scan(&firstUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			// No users exist yet; templates will be created when first user registers
			return nil
		}
		// Table might not exist yet during initial tests
		return nil
	}

	templates := []struct {
		fileName    string
		title       string
		description string
		category    string
		generator   func() []byte
	}{
		{
			fileName:    "illinois_residential_lease_2026.pdf",
			title:       "Illinois Residential Lease Agreement (2026 Edition)",
			description: "Standard statutory Illinois residential lease agreement template compliant with 765 ILCS 705/ (Landlord and Tenant Act) and 765 ILCS 710/ (Security Deposit Return Act).",
			category:    "lease",
			generator:   GenerateIllinoisLeasePDF,
		},
		{
			fileName:    "form_i9_2026.pdf",
			title:       "Form I-9, Employment Eligibility Verification (2026 Edition)",
			description: "Official U.S. Citizenship and Immigration Services (USCIS) Form I-9 for identity and work authorization verification.",
			category:    "form",
			generator:   GenerateI9FormPDF,
		},
	}

	for _, tmpl := range templates {
		var exists bool
		err := s.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM multimedia WHERE file_name = $1 AND is_template = true)`, tmpl.fileName).Scan(&exists)
		if err == nil && !exists {
			pdfBytes := tmpl.generator()
			_, insertErr := s.DB.Exec(ctx, `
				INSERT INTO multimedia (
					user_id, property_id, title, description, category, media_type,
					file_name, file_size, mime_type, file_path, file_data, is_template
				) VALUES ($1, NULL, $2, $3, $4, 'pdf', $5, $6, 'application/pdf', '', $7, true)
			`, firstUserID, tmpl.title, tmpl.description, tmpl.category, tmpl.fileName, int64(len(pdfBytes)), pdfBytes)
			if insertErr != nil {
				log.Printf("EnsureTemplateDocuments: failed to insert %s: %v", tmpl.fileName, insertErr)
			} else {
				log.Printf("EnsureTemplateDocuments: seeded statutory template %s (%d bytes)", tmpl.fileName, len(pdfBytes))
			}
		}
	}
	return nil
}

// classifyMediaType determines if a file is a pdf, image, video, audio, or general document.
func classifyMediaType(filename, mimeType string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	mimeLower := strings.ToLower(mimeType)

	if ext == ".pdf" || strings.Contains(mimeLower, "pdf") {
		return "pdf"
	}
	if strings.HasPrefix(mimeLower, "image/") || slicesContains([]string{".jpg", ".jpeg", ".png", ".webp", ".gif", ".svg", ".bmp"}, ext) {
		return "image"
	}
	if strings.HasPrefix(mimeLower, "video/") || slicesContains([]string{".mp4", ".mov", ".avi", ".webm", ".mkv", ".m4v"}, ext) {
		return "video"
	}
	if strings.HasPrefix(mimeLower, "audio/") || slicesContains([]string{".mp3", ".wav", ".m4a", ".aac", ".ogg", ".flac", ".wma"}, ext) {
		return "audio"
	}
	if slicesContains([]string{".doc", ".docx", ".xls", ".xlsx", ".csv", ".txt", ".rtf", ".odt", ".ods"}, ext) {
		return "document"
	}
	return "other"
}

func slicesContains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

// listDocuments handles GET /api/documents
func (s *Server) listDocuments(w http.ResponseWriter, r *http.Request) {
	if s.DB == nil {
		writeJSON(w, http.StatusOK, []DocumentItem{})
		return
	}

	callerID := auth.UserID(r.Context())
	callerRole := auth.UserRole(r.Context())

	// Ensure standard templates are available
	_ = s.EnsureTemplateDocuments(r.Context())

	q := r.URL.Query()
	propertyIDParam := strings.TrimSpace(q.Get("property_id"))
	mediaTypeParam := strings.TrimSpace(q.Get("media_type"))
	categoryParam := strings.TrimSpace(q.Get("category"))
	searchParam := strings.TrimSpace(q.Get("search"))

	query := `
		SELECT 
			m.id, m.user_id, m.property_id, 
			p.name, p.street_address,
			m.title, m.description, m.category, m.media_type,
			m.file_name, m.file_size, m.mime_type, m.is_template,
			m.created_at, m.updated_at
		FROM multimedia m
		LEFT JOIN properties p ON p.id = m.property_id
		WHERE (m.user_id = $1 OR m.is_template = true OR $2 = 'ADMIN')
	`
	args := []any{callerID, callerRole}
	argIdx := 3

	// Filter by property
	if propertyIDParam != "" && propertyIDParam != "all" {
		if propertyIDParam == "none" || propertyIDParam == "unassigned" {
			query += " AND m.property_id IS NULL"
		} else if pid, err := strconv.ParseInt(propertyIDParam, 10, 64); err == nil && pid > 0 {
			query += fmt.Sprintf(" AND m.property_id = $%d", argIdx)
			args = append(args, pid)
			argIdx++
		}
	}

	// Filter by media type
	if mediaTypeParam != "" && mediaTypeParam != "all" {
		query += fmt.Sprintf(" AND m.media_type = $%d", argIdx)
		args = append(args, mediaTypeParam)
		argIdx++
	}

	// Filter by category
	if categoryParam != "" && categoryParam != "all" {
		query += fmt.Sprintf(" AND m.category = $%d", argIdx)
		args = append(args, categoryParam)
		argIdx++
	}

	// Search by title or file name
	if searchParam != "" {
		searchPattern := "%" + strings.ToLower(searchParam) + "%"
		query += fmt.Sprintf(" AND (LOWER(m.title) LIKE $%d OR LOWER(m.file_name) LIKE $%d OR LOWER(COALESCE(m.description, '')) LIKE $%d)", argIdx, argIdx, argIdx)
		args = append(args, searchPattern)
		argIdx++
	}

	query += " ORDER BY m.is_template DESC, m.created_at DESC"

	rows, err := s.DB.Query(r.Context(), query, args...)
	if err != nil {
		log.Printf("listDocuments: query error: %v", err)
		writeErr(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	items := make([]DocumentItem, 0)
	for rows.Next() {
		var item DocumentItem
		var propName, propAddr, desc *string

		err := rows.Scan(
			&item.ID, &item.UserID, &item.PropertyID,
			&propName, &propAddr,
			&item.Title, &desc, &item.Category, &item.MediaType,
			&item.FileName, &item.FileSize, &item.MimeType, &item.IsTemplate,
			&item.CreatedAt, &item.UpdatedAt,
		)
		if err != nil {
			log.Printf("listDocuments: scan error: %v", err)
			continue
		}

		item.PropertyName = propName
		item.PropertyAddress = propAddr
		item.Description = desc
		item.DownloadURL = fmt.Sprintf("/api/documents/%d/download", item.ID)
		item.ViewURL = fmt.Sprintf("/api/documents/%d/view", item.ID)

		items = append(items, item)
	}

	writeJSON(w, http.StatusOK, items)
}

// listPropertyDocuments handles GET /api/properties/{id}/documents
func (s *Server) listPropertyDocuments(w http.ResponseWriter, r *http.Request) {
	propIDStr := r.PathValue("id")
	// Re-route with property_id query parameter
	q := r.URL.Query()
	q.Set("property_id", propIDStr)
	r.URL.RawQuery = q.Encode()
	s.listDocuments(w, r)
}

// uploadDocument handles POST /api/documents
func (s *Server) uploadDocument(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserID(r.Context())
	if callerID == 0 {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Allow up to 64MB file uploads
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "failed to parse multipart form: file too large or invalid format")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "file field is required")
		return
	}
	defer file.Close()

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = header.Filename
	}

	desc := strings.TrimSpace(r.FormValue("description"))
	var descPtr *string
	if desc != "" {
		descPtr = &desc
	}

	category := strings.TrimSpace(r.FormValue("category"))
	if category == "" {
		category = "document"
	}

	var propertyID *int64
	propertyIDStr := strings.TrimSpace(r.FormValue("property_id"))
	if propertyIDStr != "" && propertyIDStr != "none" {
		pid, err := strconv.ParseInt(propertyIDStr, 10, 64)
		if err == nil && pid > 0 {
			propertyID = &pid
		}
	}

	// Read file bytes
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to read file content")
		return
	}

	fileSize := int64(len(fileBytes))
	if fileSize == 0 {
		writeErr(w, http.StatusBadRequest, "uploaded file is empty")
		return
	}

	// Detect MIME type
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = mime.TypeByExtension(filepath.Ext(header.Filename))
		if mimeType == "" {
			mimeType = http.DetectContentType(fileBytes[:min(512, len(fileBytes))])
		}
	}

	mediaType := classifyMediaType(header.Filename, mimeType)

	// Save to disk in uploads directory
	uploadsDir := filepath.Join("uploads", "multimedia")
	_ = os.MkdirAll(uploadsDir, 0755)

	safeFilename := filepath.Base(header.Filename)
	diskFilename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), safeFilename)
	diskPath := filepath.Join(uploadsDir, diskFilename)
	_ = os.WriteFile(diskPath, fileBytes, 0644)

	// Insert into PostgreSQL multimedia table
	var item DocumentItem
	item.UserID = callerID
	item.PropertyID = propertyID
	item.Title = title
	item.Description = descPtr
	item.Category = category
	item.MediaType = mediaType
	item.FileName = safeFilename
	item.FileSize = fileSize
	item.MimeType = mimeType
	item.IsTemplate = false

	if s.DB != nil {
		err = s.DB.QueryRow(r.Context(), `
			INSERT INTO multimedia (
				user_id, property_id, title, description, category, media_type,
				file_name, file_size, mime_type, file_path, file_data, is_template
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, false)
			RETURNING id, created_at, updated_at
		`, callerID, propertyID, title, descPtr, category, mediaType, safeFilename, fileSize, mimeType, diskPath, fileBytes).Scan(
			&item.ID, &item.CreatedAt, &item.UpdatedAt,
		)
		if err != nil {
			log.Printf("uploadDocument: database insert error: %v", err)
			writeErr(w, http.StatusInternalServerError, "failed to save document")
			return
		}

		// Fetch property address and name if tied to a property
		if propertyID != nil {
			_ = s.DB.QueryRow(r.Context(),
				`SELECT name, street_address FROM properties WHERE id = $1`, *propertyID).Scan(&item.PropertyName, &item.PropertyAddress)
		}
	} else {
		// Mock ID for tests without database
		item.ID = time.Now().Unix()
		item.CreatedAt = time.Now()
		item.UpdatedAt = time.Now()
	}

	item.DownloadURL = fmt.Sprintf("/api/documents/%d/download", item.ID)
	item.ViewURL = fmt.Sprintf("/api/documents/%d/view", item.ID)

	writeJSON(w, http.StatusCreated, item)
}

// uploadPropertyDocument handles POST /api/properties/{id}/documents
func (s *Server) uploadPropertyDocument(w http.ResponseWriter, r *http.Request) {
	propIDStr := r.PathValue("id")
	// If property_id not in form, populate it
	r.Form = make(map[string][]string)
	r.Form.Set("property_id", propIDStr)
	s.uploadDocument(w, r)
}

// getDocument handles GET /api/documents/{id}
func (s *Server) getDocument(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid document id")
		return
	}

	callerID := auth.UserID(r.Context())
	callerRole := auth.UserRole(r.Context())

	if s.DB == nil {
		writeErr(w, http.StatusNotFound, "document not found")
		return
	}

	var item DocumentItem
	var propName, propAddr, desc *string

	err = s.DB.QueryRow(r.Context(), `
		SELECT 
			m.id, m.user_id, m.property_id, 
			p.name, p.street_address,
			m.title, m.description, m.category, m.media_type,
			m.file_name, m.file_size, m.mime_type, m.is_template,
			m.created_at, m.updated_at
		FROM multimedia m
		LEFT JOIN properties p ON p.id = m.property_id
		WHERE m.id = $1 AND (m.user_id = $2 OR m.is_template = true OR $3 = 'ADMIN')
	`, id, callerID, callerRole).Scan(
		&item.ID, &item.UserID, &item.PropertyID,
		&propName, &propAddr,
		&item.Title, &desc, &item.Category, &item.MediaType,
		&item.FileName, &item.FileSize, &item.MimeType, &item.IsTemplate,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "document not found or access denied")
			return
		}
		writeErr(w, http.StatusInternalServerError, "database error")
		return
	}

	item.PropertyName = propName
	item.PropertyAddress = propAddr
	item.Description = desc
	item.DownloadURL = fmt.Sprintf("/api/documents/%d/download", item.ID)
	item.ViewURL = fmt.Sprintf("/api/documents/%d/view", item.ID)

	writeJSON(w, http.StatusOK, item)
}

// downloadDocumentFile handles GET /api/documents/{id}/download (attachment)
func (s *Server) downloadDocumentFile(w http.ResponseWriter, r *http.Request) {
	s.serveDocumentFile(w, r, false)
}

// viewDocumentFile handles GET /api/documents/{id}/view (inline view for PDF/images)
func (s *Server) viewDocumentFile(w http.ResponseWriter, r *http.Request) {
	s.serveDocumentFile(w, r, true)
}

// serveDocumentFile serves the binary stream from disk or database BYTEA.
func (s *Server) serveDocumentFile(w http.ResponseWriter, r *http.Request, inline bool) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid document id")
		return
	}

	callerID := auth.UserID(r.Context())
	callerRole := auth.UserRole(r.Context())

	if s.DB == nil {
		writeErr(w, http.StatusNotFound, "document not found")
		return
	}

	var userID int64
	var isTemplate bool
	var fileName, mimeType, filePath string
	var fileSize int64
	var fileData []byte

	err = s.DB.QueryRow(r.Context(), `
		SELECT user_id, is_template, file_name, mime_type, file_size, file_path, file_data
		FROM multimedia
		WHERE id = $1
	`, id).Scan(&userID, &isTemplate, &fileName, &mimeType, &fileSize, &filePath, &fileData)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "document not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "database error")
		return
	}

	// Permission check
	if !isTemplate && userID != callerID && callerRole != "ADMIN" {
		writeErr(w, http.StatusForbidden, "forbidden: you do not have permission to view this document")
		return
	}

	// Determine Content-Disposition
	dispType := "attachment"
	if inline {
		dispType = "inline"
	}
	safeName := filepath.Base(fileName)
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=%q", dispType, safeName))
	w.Header().Set("Content-Type", mimeType)

	// Stream from disk if exists
	if filePath != "" {
		if fileInfo, statErr := os.Stat(filePath); statErr == nil && fileInfo.Size() > 0 {
			f, openErr := os.Open(filePath)
			if openErr == nil {
				defer f.Close()
				w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
				_, _ = io.Copy(w, f)
				return
			}
		}
	}

	// Fallback to database BYTEA
	if len(fileData) > 0 {
		w.Header().Set("Content-Length", strconv.Itoa(len(fileData)))
		_, _ = w.Write(fileData)
		return
	}

	writeErr(w, http.StatusNotFound, "file content is unavailable on the server")
}

// deleteDocument handles DELETE /api/documents/{id}
func (s *Server) deleteDocument(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid document id")
		return
	}

	callerID := auth.UserID(r.Context())
	callerRole := auth.UserRole(r.Context())

	if s.DB == nil {
		writeErr(w, http.StatusNotFound, "document not found")
		return
	}

	var userID int64
	var isTemplate bool
	var filePath string

	err = s.DB.QueryRow(r.Context(), `
		SELECT user_id, is_template, file_path
		FROM multimedia
		WHERE id = $1
	`, id).Scan(&userID, &isTemplate, &filePath)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "document not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "database error")
		return
	}

	// Templates cannot be deleted by standard users
	if isTemplate && callerRole != "ADMIN" {
		writeErr(w, http.StatusForbidden, "statutory legal templates cannot be deleted")
		return
	}

	if userID != callerID && callerRole != "ADMIN" {
		writeErr(w, http.StatusForbidden, "forbidden: you cannot delete documents owned by another user")
		return
	}

	// Delete from database
	_, err = s.DB.Exec(r.Context(), `DELETE FROM multimedia WHERE id = $1`, id)
	if err != nil {
		log.Printf("deleteDocument: database delete error: %v", err)
		writeErr(w, http.StatusInternalServerError, "failed to delete document")
		return
	}

	// Clean up disk file if present
	if filePath != "" {
		_ = os.Remove(filePath)
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "document successfully deleted",
	})
}
