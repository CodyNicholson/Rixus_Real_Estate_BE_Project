package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/CodyNicholson/Rixus_Real_Estate_BE_Project/internal/auth"
	"github.com/jackc/pgx/v5"
)

// Contractor represents a service provider or general contractor.
type Contractor struct {
	ID            int64                      `json:"id"`
	UserID        int64                      `json:"user_id"`
	Name          string                     `json:"name"`
	ContactPerson string                     `json:"contact_person"`
	Phone         string                     `json:"phone"`
	Email         string                     `json:"email"`
	Specialty     string                     `json:"specialty"`
	ServiceArea   string                     `json:"service_area"`
	HourlyRate    *float64                   `json:"hourly_rate"`
	Rating        float64                    `json:"rating"`
	Notes         string                     `json:"notes"`
	IsActive      bool                       `json:"is_active"`
	CreatedAt     time.Time                  `json:"created_at"`
	UpdatedAt     time.Time                  `json:"updated_at"`
	TotalProjects int                        `json:"total_projects"`
	PastProjects  []ContractorProjectSummary `json:"past_projects"`
}

type ContractorProjectSummary struct {
	ProjectID     int64    `json:"project_id"`
	Title         string   `json:"title"`
	PropertyName  string   `json:"property_name"`
	Status        string   `json:"status"`
	Priority      string   `json:"priority"`
	EstimatedCost float64  `json:"estimated_cost"`
	ActualCost    float64  `json:"actual_cost"`
	DateStarted   *string  `json:"date_started"`
	DateCompleted *string  `json:"date_completed"`
}

// Project represents a property maintenance task, issue, or capital improvement.
type Project struct {
	ID                  int64                 `json:"id"`
	UserID              int64                 `json:"user_id"`
	PropertyID          int64                 `json:"property_id"`
	PropertyName        string                `json:"property_name"`
	PropertyAddress     string                `json:"property_address"`
	ContractorID        *int64                `json:"contractor_id"`
	ContractorName      *string               `json:"contractor_name"`
	ContractorSpecialty *string               `json:"contractor_specialty"`
	ContractorPhone     *string               `json:"contractor_phone"`
	Title               string                `json:"title"`
	Description         string                `json:"description"`
	Category            string                `json:"category"`
	Priority            string                `json:"priority"`
	Status              string                `json:"status"`
	DateIdentified      string                `json:"date_identified"`
	DateStarted         *string               `json:"date_started"`
	DateCompleted       *string               `json:"date_completed"`
	EstimatedCost       float64               `json:"estimated_cost"`
	ActualCost          float64               `json:"actual_cost"`
	CreatedAt           time.Time             `json:"created_at"`
	UpdatedAt           time.Time             `json:"updated_at"`
	Images              []ProjectImageSummary `json:"images"`
}

// ProjectImageSummary represents an uploaded photo metadata for a project.
type ProjectImageSummary struct {
	ID        int64     `json:"id"`
	ProjectID int64     `json:"project_id"`
	Caption   string    `json:"caption"`
	FileName  string    `json:"file_name"`
	FileSize  int64     `json:"file_size"`
	MimeType  string    `json:"mime_type"`
	CreatedAt time.Time `json:"created_at"`
	ViewURL   string    `json:"view_url"`
}

// registerProjectRoutes registers all project and contractor endpoints.
func (s *Server) registerProjectRoutes(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	// Projects endpoints
	mux.Handle("GET /api/projects", authMW(http.HandlerFunc(s.listProjects)))
	mux.Handle("POST /api/projects", authMW(http.HandlerFunc(s.createProject)))
	mux.Handle("GET /api/projects/{id}", authMW(http.HandlerFunc(s.getProject)))
	mux.Handle("PUT /api/projects/{id}", authMW(http.HandlerFunc(s.updateProject)))
	mux.Handle("DELETE /api/projects/{id}", authMW(http.HandlerFunc(s.deleteProject)))

	// Project Images endpoints
	mux.Handle("POST /api/projects/{id}/images", authMW(http.HandlerFunc(s.uploadProjectImage)))
	mux.Handle("GET /api/projects/{id}/images", authMW(http.HandlerFunc(s.listProjectImages)))
	mux.Handle("GET /api/project-images/{id}/view", s.projectImageAuthMW(s.viewProjectImage))
	mux.Handle("DELETE /api/project-images/{id}", authMW(http.HandlerFunc(s.deleteProjectImage)))

	// Contractors endpoints
	mux.Handle("GET /api/contractors", authMW(http.HandlerFunc(s.listContractors)))
	mux.Handle("POST /api/contractors", authMW(http.HandlerFunc(s.createContractor)))
	mux.Handle("GET /api/contractors/{id}", authMW(http.HandlerFunc(s.getContractor)))
	mux.Handle("PUT /api/contractors/{id}", authMW(http.HandlerFunc(s.updateContractor)))
	mux.Handle("DELETE /api/contractors/{id}", authMW(http.HandlerFunc(s.deleteContractor)))
}

// projectImageAuthMW authenticates image views via Bearer header or query token.
func (s *Server) projectImageAuthMW(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		tok, ok := strings.CutPrefix(h, "Bearer ")
		if !ok || strings.TrimSpace(tok) == "" {
			tok = r.URL.Query().Get("token")
			if tok == "" {
				tok = r.URL.Query().Get("access_token")
			}
			ok = tok != ""
		}

		if ok && tok != "" {
			claims, err := auth.ParseAccessTokenClaims(s.JWTSecret, tok)
			if err == nil {
				id, err := strconv.ParseInt(claims.Subject, 10, 64)
				if err == nil {
					ctx := auth.WithUser(r.Context(), id, claims.UserRole)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
		}

		// In development mode, allow if dev auth enabled
		if s.AllowDevAuth && s.DevCFEmail != "" && s.DB != nil {
			var userID int64
			var userRole string
			if err := s.DB.QueryRow(r.Context(), "SELECT id, user_role FROM users WHERE email = $1", s.DevCFEmail).Scan(&userID, &userRole); err == nil {
				ctx := auth.WithUser(r.Context(), userID, userRole)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		writeErr(w, http.StatusUnauthorized, "unauthorized")
	})
}

// --- PROJECTS HANDLERS ---

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserID(r.Context())
	callerRole := auth.UserRole(r.Context())
	if callerID == 0 {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	propertyIDStr := r.URL.Query().Get("property_id")
	statusFilter := r.URL.Query().Get("status")
	priorityFilter := r.URL.Query().Get("priority")

	var query strings.Builder
	query.WriteString(`
		SELECT 
			p.id, p.user_id, p.property_id,
			COALESCE(prop.name, prop.street_address) AS property_name,
			prop.street_address AS property_address,
			p.contractor_id,
			c.name AS contractor_name,
			c.specialty AS contractor_specialty,
			c.phone AS contractor_phone,
			p.title, p.description, p.category, p.priority, p.status,
			TO_CHAR(p.date_identified, 'YYYY-MM-DD') AS date_identified,
			TO_CHAR(p.date_started, 'YYYY-MM-DD') AS date_started,
			TO_CHAR(p.date_completed, 'YYYY-MM-DD') AS date_completed,
			p.estimated_cost::float8, p.actual_cost::float8,
			p.created_at, p.updated_at
		FROM projects p
		JOIN properties prop ON prop.id = p.property_id
		LEFT JOIN contractors c ON c.id = p.contractor_id
		WHERE 1=1
	`)

	args := []interface{}{}
	argIdx := 1

	if callerRole != "ADMIN" {
		query.WriteString(fmt.Sprintf(" AND p.user_id = $%d", argIdx))
		args = append(args, callerID)
		argIdx++
	}

	if propertyIDStr != "" {
		if propID, err := strconv.ParseInt(propertyIDStr, 10, 64); err == nil {
			query.WriteString(fmt.Sprintf(" AND p.property_id = $%d", argIdx))
			args = append(args, propID)
			argIdx++
		}
	}

	if statusFilter != "" && statusFilter != "ALL" {
		query.WriteString(fmt.Sprintf(" AND p.status = $%d", argIdx))
		args = append(args, strings.ToUpper(statusFilter))
		argIdx++
	}

	if priorityFilter != "" && priorityFilter != "ALL" {
		query.WriteString(fmt.Sprintf(" AND p.priority = $%d", argIdx))
		args = append(args, strings.ToUpper(priorityFilter))
		argIdx++
	}

	query.WriteString(" ORDER BY CASE p.priority WHEN 'URGENT' THEN 1 WHEN 'HIGH' THEN 2 WHEN 'MEDIUM' THEN 3 ELSE 4 END, p.date_identified DESC, p.id DESC")

	if s.DB == nil {
		writeJSON(w, http.StatusOK, []Project{})
		return
	}

	rows, err := s.DB.Query(r.Context(), query.String(), args...)
	if err != nil {
		log.Printf("[Projects] list error: %v", err)
		writeErr(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	projects := make([]Project, 0)
	projectIDs := make([]int64, 0)

	for rows.Next() {
		var prj Project
		var dStarted, dCompleted sql.NullString
		err := rows.Scan(
			&prj.ID, &prj.UserID, &prj.PropertyID,
			&prj.PropertyName, &prj.PropertyAddress,
			&prj.ContractorID, &prj.ContractorName, &prj.ContractorSpecialty, &prj.ContractorPhone,
			&prj.Title, &prj.Description, &prj.Category, &prj.Priority, &prj.Status,
			&prj.DateIdentified, &dStarted, &dCompleted,
			&prj.EstimatedCost, &prj.ActualCost,
			&prj.CreatedAt, &prj.UpdatedAt,
		)
		if err != nil {
			log.Printf("[Projects] scan error: %v", err)
			continue
		}
		if dStarted.Valid {
			prj.DateStarted = &dStarted.String
		}
		if dCompleted.Valid {
			prj.DateCompleted = &dCompleted.String
		}
		prj.Images = make([]ProjectImageSummary, 0)
		projects = append(projects, prj)
		projectIDs = append(projectIDs, prj.ID)
	}

	// Fetch associated images for all retrieved projects
	if len(projectIDs) > 0 {
		imgRows, err := s.DB.Query(r.Context(), `
			SELECT id, project_id, caption, file_name, file_size, mime_type, created_at
			FROM project_images
			WHERE project_id = ANY($1)
			ORDER BY id ASC
		`, projectIDs)
		if err == nil {
			defer imgRows.Close()
			imgMap := make(map[int64][]ProjectImageSummary)
			for imgRows.Next() {
				var img ProjectImageSummary
				if scanErr := imgRows.Scan(&img.ID, &img.ProjectID, &img.Caption, &img.FileName, &img.FileSize, &img.MimeType, &img.CreatedAt); scanErr == nil {
					img.ViewURL = fmt.Sprintf("/api/project-images/%d/view", img.ID)
					imgMap[img.ProjectID] = append(imgMap[img.ProjectID], img)
				}
			}
			for i := range projects {
				if imgs, ok := imgMap[projects[i].ID]; ok {
					projects[i].Images = imgs
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, projects)
}

type projectInput struct {
	PropertyID     int64    `json:"property_id"`
	ContractorID   *int64   `json:"contractor_id"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Category       string   `json:"category"`
	Priority       string   `json:"priority"`
	Status         string   `json:"status"`
	DateIdentified string   `json:"date_identified"`
	DateStarted    *string  `json:"date_started"`
	DateCompleted  *string  `json:"date_completed"`
	EstimatedCost  *float64 `json:"estimated_cost"`
	ActualCost     *float64 `json:"actual_cost"`
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserID(r.Context())
	if callerID == 0 {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var in projectInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	title := strings.TrimSpace(in.Title)
	if title == "" {
		writeErr(w, http.StatusBadRequest, "title is required")
		return
	}
	if in.PropertyID <= 0 {
		writeErr(w, http.StatusBadRequest, "valid property_id is required")
		return
	}

	// Validate property ownership
	var propOwnerID int64
	if s.DB != nil {
		err := s.DB.QueryRow(r.Context(), "SELECT user_id FROM properties WHERE id = $1", in.PropertyID).Scan(&propOwnerID)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "property not found")
			return
		}
		if auth.UserRole(r.Context()) != "ADMIN" && propOwnerID != callerID {
			writeErr(w, http.StatusForbidden, "forbidden: cannot add projects to a property you do not own")
			return
		}
	}

	priority := strings.ToUpper(strings.TrimSpace(in.Priority))
	if priority == "" {
		priority = "MEDIUM"
	}
	switch priority {
	case "LOW", "MEDIUM", "HIGH", "URGENT":
	default:
		priority = "MEDIUM"
	}

	status := strings.ToUpper(strings.TrimSpace(in.Status))
	if status == "" {
		status = "OPEN"
	}
	switch status {
	case "OPEN", "IN_PROGRESS", "COMPLETED", "ON_HOLD", "CANCELLED":
	default:
		status = "OPEN"
	}

	category := strings.ToUpper(strings.TrimSpace(in.Category))
	if category == "" {
		category = "MAINTENANCE"
	}

	dateIdentified := strings.TrimSpace(in.DateIdentified)
	if dateIdentified == "" {
		dateIdentified = time.Now().Format("2006-01-02")
	}

	estCost := 0.0
	if in.EstimatedCost != nil && *in.EstimatedCost >= 0 {
		estCost = *in.EstimatedCost
	}
	actCost := 0.0
	if in.ActualCost != nil && *in.ActualCost >= 0 {
		actCost = *in.ActualCost
	}

	var projectID int64
	err := s.DB.QueryRow(r.Context(), `
		INSERT INTO projects (
			user_id, property_id, contractor_id, title, description, category,
			priority, status, date_identified, date_started, date_completed,
			estimated_cost, actual_cost
		)
		VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9::date, NULLIF($10, '')::date, NULLIF($11, '')::date,
			$12, $13
		)
		RETURNING id
	`, callerID, in.PropertyID, in.ContractorID, title, strings.TrimSpace(in.Description), category,
		priority, status, dateIdentified, strPtrToString(in.DateStarted), strPtrToString(in.DateCompleted),
		estCost, actCost).Scan(&projectID)

	if err != nil {
		log.Printf("[Projects] create error: %v", err)
		writeErr(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	s.getProjectByID(w, r, projectID, http.StatusCreated)
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid project id")
		return
	}
	s.getProjectByID(w, r, id, http.StatusOK)
}

func (s *Server) getProjectByID(w http.ResponseWriter, r *http.Request, id int64, statusCode int) {
	callerID := auth.UserID(r.Context())
	callerRole := auth.UserRole(r.Context())

	if s.DB == nil {
		writeErr(w, http.StatusNotFound, "project not found")
		return
	}

	var prj Project
	var dStarted, dCompleted sql.NullString
	err := s.DB.QueryRow(r.Context(), `
		SELECT 
			p.id, p.user_id, p.property_id,
			COALESCE(prop.name, prop.street_address) AS property_name,
			prop.street_address AS property_address,
			p.contractor_id,
			c.name AS contractor_name,
			c.specialty AS contractor_specialty,
			c.phone AS contractor_phone,
			p.title, p.description, p.category, p.priority, p.status,
			TO_CHAR(p.date_identified, 'YYYY-MM-DD') AS date_identified,
			TO_CHAR(p.date_started, 'YYYY-MM-DD') AS date_started,
			TO_CHAR(p.date_completed, 'YYYY-MM-DD') AS date_completed,
			p.estimated_cost::float8, p.actual_cost::float8,
			p.created_at, p.updated_at
		FROM projects p
		JOIN properties prop ON prop.id = p.property_id
		LEFT JOIN contractors c ON c.id = p.contractor_id
		WHERE p.id = $1
	`, id).Scan(
		&prj.ID, &prj.UserID, &prj.PropertyID,
		&prj.PropertyName, &prj.PropertyAddress,
		&prj.ContractorID, &prj.ContractorName, &prj.ContractorSpecialty, &prj.ContractorPhone,
		&prj.Title, &prj.Description, &prj.Category, &prj.Priority, &prj.Status,
		&prj.DateIdentified, &dStarted, &dCompleted,
		&prj.EstimatedCost, &prj.ActualCost,
		&prj.CreatedAt, &prj.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "project not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "database error")
		return
	}

	if callerRole != "ADMIN" && prj.UserID != callerID {
		writeErr(w, http.StatusForbidden, "forbidden: you cannot access this project")
		return
	}

	if dStarted.Valid {
		prj.DateStarted = &dStarted.String
	}
	if dCompleted.Valid {
		prj.DateCompleted = &dCompleted.String
	}

	// Fetch photos
	prj.Images = make([]ProjectImageSummary, 0)
	imgRows, err := s.DB.Query(r.Context(), `
		SELECT id, project_id, caption, file_name, file_size, mime_type, created_at
		FROM project_images
		WHERE project_id = $1
		ORDER BY id ASC
	`, id)
	if err == nil {
		defer imgRows.Close()
		for imgRows.Next() {
			var img ProjectImageSummary
			if err := imgRows.Scan(&img.ID, &img.ProjectID, &img.Caption, &img.FileName, &img.FileSize, &img.MimeType, &img.CreatedAt); err == nil {
				img.ViewURL = fmt.Sprintf("/api/project-images/%d/view", img.ID)
				prj.Images = append(prj.Images, img)
			}
		}
	}

	writeJSON(w, statusCode, prj)
}

func (s *Server) updateProject(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserID(r.Context())
	callerRole := auth.UserRole(r.Context())
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid project id")
		return
	}

	var in projectInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Check ownership
	var existingUserID int64
	err = s.DB.QueryRow(r.Context(), "SELECT user_id FROM projects WHERE id = $1", id).Scan(&existingUserID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "project not found")
		return
	}
	if callerRole != "ADMIN" && existingUserID != callerID {
		writeErr(w, http.StatusForbidden, "forbidden: cannot modify projects owned by another user")
		return
	}

	title := strings.TrimSpace(in.Title)
	if title == "" {
		writeErr(w, http.StatusBadRequest, "title is required")
		return
	}

	priority := strings.ToUpper(strings.TrimSpace(in.Priority))
	switch priority {
	case "LOW", "MEDIUM", "HIGH", "URGENT":
	default:
		priority = "MEDIUM"
	}

	status := strings.ToUpper(strings.TrimSpace(in.Status))
	switch status {
	case "OPEN", "IN_PROGRESS", "COMPLETED", "ON_HOLD", "CANCELLED":
	default:
		status = "OPEN"
	}

	category := strings.ToUpper(strings.TrimSpace(in.Category))
	if category == "" {
		category = "MAINTENANCE"
	}

	dateIdentified := strings.TrimSpace(in.DateIdentified)
	if dateIdentified == "" {
		dateIdentified = time.Now().Format("2006-01-02")
	}

	estCost := 0.0
	if in.EstimatedCost != nil && *in.EstimatedCost >= 0 {
		estCost = *in.EstimatedCost
	}
	actCost := 0.0
	if in.ActualCost != nil && *in.ActualCost >= 0 {
		actCost = *in.ActualCost
	}

	// If marked completed and no date_completed, default to today
	dateCompletedStr := strPtrToString(in.DateCompleted)
	if status == "COMPLETED" && dateCompletedStr == "" {
		dateCompletedStr = time.Now().Format("2006-01-02")
	}

	_, err = s.DB.Exec(r.Context(), `
		UPDATE projects
		SET 
			property_id = CASE WHEN $2 > 0 THEN $2 ELSE property_id END,
			contractor_id = $3,
			title = $4,
			description = $5,
			category = $6,
			priority = $7,
			status = $8,
			date_identified = $9::date,
			date_started = NULLIF($10, '')::date,
			date_completed = NULLIF($11, '')::date,
			estimated_cost = $12,
			actual_cost = $13,
			updated_at = now()
		WHERE id = $1
	`, id, in.PropertyID, in.ContractorID, title, strings.TrimSpace(in.Description), category,
		priority, status, dateIdentified, strPtrToString(in.DateStarted), dateCompletedStr,
		estCost, actCost)

	if err != nil {
		log.Printf("[Projects] update error: %v", err)
		writeErr(w, http.StatusInternalServerError, "failed to update project")
		return
	}

	s.getProjectByID(w, r, id, http.StatusOK)
}

func (s *Server) deleteProject(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserID(r.Context())
	callerRole := auth.UserRole(r.Context())
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid project id")
		return
	}

	var existingUserID int64
	err = s.DB.QueryRow(r.Context(), "SELECT user_id FROM projects WHERE id = $1", id).Scan(&existingUserID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "project not found")
		return
	}
	if callerRole != "ADMIN" && existingUserID != callerID {
		writeErr(w, http.StatusForbidden, "forbidden: cannot delete projects owned by another user")
		return
	}

	_, err = s.DB.Exec(r.Context(), "DELETE FROM projects WHERE id = $1", id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "database error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- PROJECT IMAGES HANDLERS ---

func (s *Server) uploadProjectImage(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserID(r.Context())
	if callerID == 0 {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	idStr := r.PathValue("id")
	projectID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || projectID <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid project id")
		return
	}

	// Verify project exists & permissions
	var prjOwnerID int64
	if s.DB != nil {
		err := s.DB.QueryRow(r.Context(), "SELECT user_id FROM projects WHERE id = $1", projectID).Scan(&prjOwnerID)
		if err != nil {
			writeErr(w, http.StatusNotFound, "project not found")
			return
		}
		if auth.UserRole(r.Context()) != "ADMIN" && prjOwnerID != callerID {
			writeErr(w, http.StatusForbidden, "forbidden: cannot add photos to another user's project")
			return
		}
	}

	// 25MB max upload
	if err := r.ParseMultipartForm(25 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "unable to parse multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	caption := strings.TrimSpace(r.FormValue("caption"))
	fileName := filepath.Base(header.Filename)
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		ext := strings.ToLower(filepath.Ext(fileName))
		switch ext {
		case ".png":
			mimeType = "image/png"
		case ".webp":
			mimeType = "image/webp"
		case ".gif":
			mimeType = "image/gif"
		default:
			mimeType = "image/jpeg"
		}
	}

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to read uploaded file")
		return
	}

	var imageID int64
	err = s.DB.QueryRow(r.Context(), `
		INSERT INTO project_images (project_id, user_id, caption, file_name, file_size, mime_type, file_data)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, projectID, callerID, caption, fileName, len(fileBytes), mimeType, fileBytes).Scan(&imageID)

	if err != nil {
		log.Printf("[Projects] image upload error: %v", err)
		writeErr(w, http.StatusInternalServerError, "failed to store project image")
		return
	}

	img := ProjectImageSummary{
		ID:        imageID,
		ProjectID: projectID,
		Caption:   caption,
		FileName:  fileName,
		FileSize:  int64(len(fileBytes)),
		MimeType:  mimeType,
		CreatedAt: time.Now(),
		ViewURL:   fmt.Sprintf("/api/project-images/%d/view", imageID),
	}

	writeJSON(w, http.StatusCreated, img)
}

func (s *Server) listProjectImages(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	projectID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || projectID <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid project id")
		return
	}

	images := make([]ProjectImageSummary, 0)
	if s.DB == nil {
		writeJSON(w, http.StatusOK, images)
		return
	}

	rows, err := s.DB.Query(r.Context(), `
		SELECT id, project_id, caption, file_name, file_size, mime_type, created_at
		FROM project_images
		WHERE project_id = $1
		ORDER BY id ASC
	`, projectID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var img ProjectImageSummary
		if err := rows.Scan(&img.ID, &img.ProjectID, &img.Caption, &img.FileName, &img.FileSize, &img.MimeType, &img.CreatedAt); err == nil {
			img.ViewURL = fmt.Sprintf("/api/project-images/%d/view", img.ID)
			images = append(images, img)
		}
	}

	writeJSON(w, http.StatusOK, images)
}

func (s *Server) viewProjectImage(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	imageID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || imageID <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid image id")
		return
	}

	if s.DB == nil {
		writeErr(w, http.StatusNotFound, "image not found")
		return
	}

	var fileName, mimeType string
	var fileData []byte
	err = s.DB.QueryRow(r.Context(), `
		SELECT file_name, mime_type, file_data
		FROM project_images
		WHERE id = $1
	`, imageID).Scan(&fileName, &mimeType, &fileData)

	if err != nil {
		writeErr(w, http.StatusNotFound, "image not found")
		return
	}

	if len(fileData) == 0 {
		writeErr(w, http.StatusNotFound, "image data unavailable")
		return
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Length", strconv.Itoa(len(fileData)))
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filepath.Base(fileName)))
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(fileData)
}

func (s *Server) deleteProjectImage(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserID(r.Context())
	callerRole := auth.UserRole(r.Context())
	idStr := r.PathValue("id")
	imageID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || imageID <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid image id")
		return
	}

	var ownerID int64
	err = s.DB.QueryRow(r.Context(), "SELECT user_id FROM project_images WHERE id = $1", imageID).Scan(&ownerID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "image not found")
		return
	}
	if callerRole != "ADMIN" && ownerID != callerID {
		writeErr(w, http.StatusForbidden, "forbidden: cannot delete image owned by another user")
		return
	}

	_, err = s.DB.Exec(r.Context(), "DELETE FROM project_images WHERE id = $1", imageID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "database error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- CONTRACTORS HANDLERS ---

func (s *Server) listContractors(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserID(r.Context())
	callerRole := auth.UserRole(r.Context())
	if callerID == 0 {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if s.DB == nil {
		writeJSON(w, http.StatusOK, []Contractor{})
		return
	}

	var query strings.Builder
	query.WriteString(`
		SELECT 
			c.id, c.user_id, c.name, c.contact_person, c.phone, c.email,
			c.specialty, c.service_area, c.hourly_rate::float8, c.rating::float8,
			c.notes, c.is_active, c.created_at, c.updated_at
		FROM contractors c
		WHERE 1=1
	`)

	args := []interface{}{}
	if callerRole != "ADMIN" {
		query.WriteString(" AND c.user_id = $1")
		args = append(args, callerID)
	}
	query.WriteString(" ORDER BY c.name ASC")

	rows, err := s.DB.Query(r.Context(), query.String(), args...)
	if err != nil {
		log.Printf("[Contractors] list error: %v", err)
		writeErr(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	contractors := make([]Contractor, 0)
	contractorIDs := make([]int64, 0)

	for rows.Next() {
		var c Contractor
		var hRate sql.NullFloat64
		err := rows.Scan(
			&c.ID, &c.UserID, &c.Name, &c.ContactPerson, &c.Phone, &c.Email,
			&c.Specialty, &c.ServiceArea, &hRate, &c.Rating,
			&c.Notes, &c.IsActive, &c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			log.Printf("[Contractors] scan error: %v", err)
			continue
		}
		if hRate.Valid {
			c.HourlyRate = &hRate.Float64
		}
		c.PastProjects = make([]ContractorProjectSummary, 0)
		contractors = append(contractors, c)
		contractorIDs = append(contractorIDs, c.ID)
	}

	// Fetch past and active jobs worked on for these contractors
	if len(contractorIDs) > 0 {
		pRows, err := s.DB.Query(r.Context(), `
			SELECT 
				p.id, p.contractor_id, p.title,
				COALESCE(prop.name, prop.street_address) AS property_name,
				p.status, p.priority,
				p.estimated_cost::float8, p.actual_cost::float8,
				TO_CHAR(p.date_started, 'YYYY-MM-DD') AS date_started,
				TO_CHAR(p.date_completed, 'YYYY-MM-DD') AS date_completed
			FROM projects p
			JOIN properties prop ON prop.id = p.property_id
			WHERE p.contractor_id = ANY($1)
			ORDER BY p.date_identified DESC, p.id DESC
		`, contractorIDs)
		if err == nil {
			defer pRows.Close()
			projMap := make(map[int64][]ContractorProjectSummary)
			for pRows.Next() {
				var cp ContractorProjectSummary
				var cID int64
				var dStarted, dCompleted sql.NullString
				if err := pRows.Scan(&cp.ProjectID, &cID, &cp.Title, &cp.PropertyName, &cp.Status, &cp.Priority, &cp.EstimatedCost, &cp.ActualCost, &dStarted, &dCompleted); err == nil {
					if dStarted.Valid {
						cp.DateStarted = &dStarted.String
					}
					if dCompleted.Valid {
						cp.DateCompleted = &dCompleted.String
					}
					projMap[cID] = append(projMap[cID], cp)
				}
			}
			for i := range contractors {
				if projs, ok := projMap[contractors[i].ID]; ok {
					contractors[i].PastProjects = projs
					contractors[i].TotalProjects = len(projs)
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, contractors)
}

type contractorInput struct {
	Name          string   `json:"name"`
	ContactPerson string   `json:"contact_person"`
	Phone         string   `json:"phone"`
	Email         string   `json:"email"`
	Specialty     string   `json:"specialty"`
	ServiceArea   string   `json:"service_area"`
	HourlyRate    *float64 `json:"hourly_rate"`
	Rating        *float64 `json:"rating"`
	Notes         string   `json:"notes"`
	IsActive      *bool    `json:"is_active"`
}

func (s *Server) createContractor(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserID(r.Context())
	if callerID == 0 {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var in contractorInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := strings.TrimSpace(in.Name)
	if name == "" {
		writeErr(w, http.StatusBadRequest, "contractor name is required")
		return
	}

	specialty := strings.TrimSpace(in.Specialty)
	if specialty == "" {
		specialty = "General Contractor"
	}
	serviceArea := strings.TrimSpace(in.ServiceArea)
	if serviceArea == "" {
		serviceArea = "Greater Chicago Area"
	}

	rating := 5.0
	if in.Rating != nil && *in.Rating >= 0 && *in.Rating <= 5 {
		rating = *in.Rating
	}

	isActive := true
	if in.IsActive != nil {
		isActive = *in.IsActive
	}

	var contractorID int64
	err := s.DB.QueryRow(r.Context(), `
		INSERT INTO contractors (
			user_id, name, contact_person, phone, email, specialty,
			service_area, hourly_rate, rating, notes, is_active
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`, callerID, name, strings.TrimSpace(in.ContactPerson), strings.TrimSpace(in.Phone),
		strings.TrimSpace(in.Email), specialty, serviceArea, in.HourlyRate, rating,
		strings.TrimSpace(in.Notes), isActive).Scan(&contractorID)

	if err != nil {
		log.Printf("[Contractors] create error: %v", err)
		writeErr(w, http.StatusInternalServerError, "failed to create contractor")
		return
	}

	s.getContractorByID(w, r, contractorID, http.StatusCreated)
}

func (s *Server) getContractor(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid contractor id")
		return
	}
	s.getContractorByID(w, r, id, http.StatusOK)
}

func (s *Server) getContractorByID(w http.ResponseWriter, r *http.Request, id int64, statusCode int) {
	callerID := auth.UserID(r.Context())
	callerRole := auth.UserRole(r.Context())

	if s.DB == nil {
		writeErr(w, http.StatusNotFound, "contractor not found")
		return
	}

	var c Contractor
	var hRate sql.NullFloat64
	err := s.DB.QueryRow(r.Context(), `
		SELECT 
			c.id, c.user_id, c.name, c.contact_person, c.phone, c.email,
			c.specialty, c.service_area, c.hourly_rate::float8, c.rating::float8,
			c.notes, c.is_active, c.created_at, c.updated_at
		FROM contractors c
		WHERE c.id = $1
	`, id).Scan(
		&c.ID, &c.UserID, &c.Name, &c.ContactPerson, &c.Phone, &c.Email,
		&c.Specialty, &c.ServiceArea, &hRate, &c.Rating,
		&c.Notes, &c.IsActive, &c.CreatedAt, &c.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "contractor not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "database error")
		return
	}

	if callerRole != "ADMIN" && c.UserID != callerID {
		writeErr(w, http.StatusForbidden, "forbidden: cannot access this contractor")
		return
	}

	if hRate.Valid {
		c.HourlyRate = &hRate.Float64
	}

	// Fetch past jobs
	c.PastProjects = make([]ContractorProjectSummary, 0)
	pRows, err := s.DB.Query(r.Context(), `
		SELECT 
			p.id, p.contractor_id, p.title,
			COALESCE(prop.name, prop.street_address) AS property_name,
			p.status, p.priority,
			p.estimated_cost::float8, p.actual_cost::float8,
			TO_CHAR(p.date_started, 'YYYY-MM-DD') AS date_started,
			TO_CHAR(p.date_completed, 'YYYY-MM-DD') AS date_completed
		FROM projects p
		JOIN properties prop ON prop.id = p.property_id
		WHERE p.contractor_id = $1
		ORDER BY p.date_identified DESC, p.id DESC
	`, id)
	if err == nil {
		defer pRows.Close()
		for pRows.Next() {
			var cp ContractorProjectSummary
			var cID int64
			var dStarted, dCompleted sql.NullString
			if err := pRows.Scan(&cp.ProjectID, &cID, &cp.Title, &cp.PropertyName, &cp.Status, &cp.Priority, &cp.EstimatedCost, &cp.ActualCost, &dStarted, &dCompleted); err == nil {
				if dStarted.Valid {
					cp.DateStarted = &dStarted.String
				}
				if dCompleted.Valid {
					cp.DateCompleted = &dCompleted.String
				}
				c.PastProjects = append(c.PastProjects, cp)
			}
		}
		c.TotalProjects = len(c.PastProjects)
	}

	writeJSON(w, statusCode, c)
}

func (s *Server) updateContractor(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserID(r.Context())
	callerRole := auth.UserRole(r.Context())
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid contractor id")
		return
	}

	var in contractorInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := strings.TrimSpace(in.Name)
	if name == "" {
		writeErr(w, http.StatusBadRequest, "contractor name is required")
		return
	}

	var existingUserID int64
	err = s.DB.QueryRow(r.Context(), "SELECT user_id FROM contractors WHERE id = $1", id).Scan(&existingUserID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "contractor not found")
		return
	}
	if callerRole != "ADMIN" && existingUserID != callerID {
		writeErr(w, http.StatusForbidden, "forbidden: cannot modify contractors owned by another user")
		return
	}

	specialty := strings.TrimSpace(in.Specialty)
	if specialty == "" {
		specialty = "General Contractor"
	}
	serviceArea := strings.TrimSpace(in.ServiceArea)
	if serviceArea == "" {
		serviceArea = "Greater Chicago Area"
	}

	rating := 5.0
	if in.Rating != nil && *in.Rating >= 0 && *in.Rating <= 5 {
		rating = *in.Rating
	}

	isActive := true
	if in.IsActive != nil {
		isActive = *in.IsActive
	}

	_, err = s.DB.Exec(r.Context(), `
		UPDATE contractors
		SET 
			name = $2,
			contact_person = $3,
			phone = $4,
			email = $5,
			specialty = $6,
			service_area = $7,
			hourly_rate = $8,
			rating = $9,
			notes = $10,
			is_active = $11,
			updated_at = now()
		WHERE id = $1
	`, id, name, strings.TrimSpace(in.ContactPerson), strings.TrimSpace(in.Phone),
		strings.TrimSpace(in.Email), specialty, serviceArea, in.HourlyRate, rating,
		strings.TrimSpace(in.Notes), isActive)

	if err != nil {
		log.Printf("[Contractors] update error: %v", err)
		writeErr(w, http.StatusInternalServerError, "failed to update contractor")
		return
	}

	s.getContractorByID(w, r, id, http.StatusOK)
}

func (s *Server) deleteContractor(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserID(r.Context())
	callerRole := auth.UserRole(r.Context())
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "invalid contractor id")
		return
	}

	var existingUserID int64
	err = s.DB.QueryRow(r.Context(), "SELECT user_id FROM contractors WHERE id = $1", id).Scan(&existingUserID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "contractor not found")
		return
	}
	if callerRole != "ADMIN" && existingUserID != callerID {
		writeErr(w, http.StatusForbidden, "forbidden: cannot delete contractors owned by another user")
		return
	}

	_, err = s.DB.Exec(r.Context(), "DELETE FROM contractors WHERE id = $1", id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "database error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func strPtrToString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
