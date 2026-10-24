package web

import (
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"Trove/internal/service"
)

// maxMultipartMemory is how much of a multipart body is buffered in memory
// before parts spill to temporary files. Uploads are not size-capped (N-1), so
// the limit stays small and large files never sit in memory.
const maxMultipartMemory = 1 << 20

// maxUploadCount bounds how many files a single request may add at once. It
// protects against a form with a pathological number of parts, not against
// large files.
const maxUploadCount = 1000

// itemInput is the parsed add/edit form, with any opened files that must be
// closed by the caller.
type itemInput struct {
	input service.ItemInput
	// files are the opened uploads backing input.Files.
	files []multipart.File
}

// Close releases every opened upload.
func (in *itemInput) Close() {
	for _, f := range in.files {
		_ = f.Close()
	}
	in.files = nil
}

// parseItemInput reads the add/edit form. It supports both ordinary
// urlencoded forms and multipart forms with file uploads.
func (s *Server) parseItemInput(w http.ResponseWriter, r *http.Request) (*itemInput, bool) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		return s.parseMultipartItemInput(w, r)
	}
	if !s.parseForm(w, r) {
		return nil, false
	}
	in := &itemInput{input: service.ItemInput{
		Title:       r.PostFormValue("title"),
		Description: r.PostFormValue("description"),
		Note:        r.PostFormValue("note"),
		Tags:        splitTags(r.PostFormValue("tags")),
		Favorite:    r.PostFormValue("favorite") == "1",
		Links:       linksFromForm(r),
	}}
	return in, true
}

// parseMultipartItemInput streams a multipart form. Non-file fields are read
// from the buffered form; file parts are opened and streamed by the service.
func (s *Server) parseMultipartItemInput(w http.ResponseWriter, r *http.Request) (*itemInput, bool) {
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		http.Error(w, "form too large or malformed", http.StatusBadRequest)
		return nil, false
	}
	in := &itemInput{input: service.ItemInput{
		Title:       r.PostFormValue("title"),
		Description: r.PostFormValue("description"),
		Note:        r.PostFormValue("note"),
		Tags:        splitTags(r.PostFormValue("tags")),
		Favorite:    r.PostFormValue("favorite") == "1",
		Links:       linksFromForm(r),
	}}

	files := r.MultipartForm.File["files"]
	if len(files) > maxUploadCount {
		http.Error(w, "too many files in one request", http.StatusRequestEntityTooLarge)
		return nil, false
	}
	labels := r.MultipartForm.Value["file_label"]
	for i, header := range files {
		f, err := header.Open()
		if err != nil {
			in.Close()
			s.serverError(w, r, "opening upload", err)
			return nil, false
		}
		in.files = append(in.files, f)
		in.input.Files = append(in.input.Files, service.FileInput{
			FileName:  header.Filename,
			MediaType: header.Header.Get("Content-Type"),
			Label:     valueAt(labels, i),
			Content:   f,
		})
	}
	return in, true
}

// linksFromForm pairs repeated link_url and link_label values (A-1, A-6).
// Blank URL rows are ignored so an empty extra row does not fail the save.
func linksFromForm(r *http.Request) []service.LinkInput {
	urls := r.PostForm["link_url"]
	labels := r.PostForm["link_label"]
	links := make([]service.LinkInput, 0, len(urls))
	for i, url := range urls {
		if strings.TrimSpace(url) == "" {
			continue
		}
		links = append(links, service.LinkInput{URL: url, Label: valueAt(labels, i)})
	}
	return links
}

func valueAt(values []string, i int) string {
	if i < 0 || i >= len(values) {
		return ""
	}
	return values[i]
}

// handleDownload streams a stored file under its original name (A-7). The
// Content-Disposition and nosniff headers stop an uploaded HTML or SVG file
// from ever running in the app's origin.
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	content, err := s.items.Download(r.Context(), r.PathValue("id"))
	if err != nil {
		s.itemError(w, r, "downloading attachment", err)
		return
	}
	defer func() {
		if err := content.Content.Close(); err != nil {
			s.logger.WarnContext(r.Context(), "closing download", "error", err)
		}
	}()

	a := content.Attachment
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", attachmentDisposition(a.FileName))
	if a.SizeBytes > 0 {
		w.Header().Set("Content-Length", itoa(a.SizeBytes))
	}
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, content.Content); err != nil {
		s.logger.WarnContext(r.Context(), "streaming download", "error", err)
	}
}

// attachmentDisposition builds a safe Content-Disposition header. The file
// name is metadata only; mime.FormatMediaType encodes it, and it never touches
// a filesystem path.
func attachmentDisposition(name string) string {
	if strings.TrimSpace(name) == "" {
		name = "download"
	}
	if value := mime.FormatMediaType("attachment", map[string]string{"filename": name}); value != "" {
		return value
	}
	return "attachment"
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}

// handleLinkRow returns one blank link row for the add/edit form. It is used
// by the "add another link" button and works as a plain link (it returns an
// HTML fragment either way).
func (s *Server) handleLinkRow(w http.ResponseWriter, r *http.Request) {
	s.renderFragment(w, "link-row", nil)
}
