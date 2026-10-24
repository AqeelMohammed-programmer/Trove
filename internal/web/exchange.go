package web

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"Trove/internal/app"
)

// maxArchiveMemory is how much of the import upload is buffered in memory
// before spilling to a temporary file. Archives stream to disk, not memory.
const maxArchiveMemory = 1 << 20

// handleExportImportPage shows the export and import screen (spec 4.8).
func (s *Server) handleExportImportPage(w http.ResponseWriter, r *http.Request) {
	data, err := s.itemPageData(r.Context(), r)
	if err != nil {
		s.serverError(w, r, "loading tags", err)
		return
	}
	data["Title"] = "Export and import"
	data["Nav"] = "export"
	switch r.URL.Query().Get("imported") {
	case "1":
		data["Notice"] = "Import finished."
	case "error":
		data["Error"] = "Import failed. Nothing was changed."
	}
	s.render(w, http.StatusOK, "export-import", data)
}

// handleExport streams the whole repository as a ZIP archive (E-1).
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	filename := app.Name + "-export-" + time.Now().UTC().Format("20060102-150405") + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", attachmentDisposition(filename))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	if err := s.items.Export(r.Context(), w); err != nil {
		// Headers are already sent; log and stop. The truncated archive is
		// obviously invalid and will fail import validation.
		s.logger.ErrorContext(r.Context(), "exporting repository", "error", err)
	}
}

// handleImport loads an uploaded archive into an empty repository (E-5). The
// upload streams to a temporary file; validation happens before any change.
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	sess, ok := sessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseMultipartForm(maxArchiveMemory); err != nil {
		http.Error(w, "upload too large or malformed", http.StatusBadRequest)
		return
	}
	if !s.checkCSRF(w, r, sess) {
		return
	}

	header, err := archiveHeader(r)
	if err != nil {
		s.renderImportError(w, r, "Choose an archive file to import.")
		return
	}

	path, err := stageArchive(header)
	if err != nil {
		s.serverError(w, r, "staging upload", err)
		return
	}
	defer func() {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			s.logger.WarnContext(r.Context(), "removing staged archive", "error", err)
		}
	}()

	if err := s.items.Import(r.Context(), path); err != nil {
		if isValidationError(err) {
			s.renderImportError(w, r, validationMessage(err))
			return
		}
		s.serverError(w, r, "importing archive", err)
		return
	}
	http.Redirect(w, r, "/export-import?imported=1", http.StatusSeeOther)
}

// renderImportError re-renders the screen with an import message.
func (s *Server) renderImportError(w http.ResponseWriter, r *http.Request, message string) {
	data, err := s.itemPageData(r.Context(), r)
	if err != nil {
		s.serverError(w, r, "loading tags", err)
		return
	}
	data["Title"] = "Export and import"
	data["Nav"] = "export"
	data["Error"] = message
	s.render(w, http.StatusBadRequest, "export-import", data)
}

func archiveHeader(r *http.Request) (*multipart.FileHeader, error) {
	if r.MultipartForm == nil {
		return nil, errors.New("no multipart form")
	}
	files := r.MultipartForm.File["archive"]
	if len(files) == 0 {
		return nil, errors.New("no archive uploaded")
	}
	return files[0], nil
}

// stageArchive streams the uploaded archive to a temporary file and returns its
// path. The caller removes it.
func stageArchive(header *multipart.FileHeader) (string, error) {
	src, err := header.Open()
	if err != nil {
		return "", err
	}
	defer src.Close() //nolint:errcheck // read-only

	tmp, err := os.CreateTemp("", "trove-import-*.zip")
	if err != nil {
		return "", err
	}
	path := tmp.Name()
	if _, err := io.Copy(tmp, src); err != nil {
		_ = tmp.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}
