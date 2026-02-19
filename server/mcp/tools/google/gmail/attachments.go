package gmail

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/api/gmail/v1"
)

// AttachmentInfo represents attachment metadata without content
type AttachmentInfo struct {
	AttachmentID string  `json:"attachment_id"`
	Filename     string  `json:"filename"`
	MimeType     string  `json:"mime_type"`
	Size         int64   `json:"size"`
	LocalPath    *string `json:"local_path,omitempty"`
	IsDownloaded bool    `json:"is_downloaded"`
}

// GetAttachmentInfo extracts attachment info from a message
func (s *Service) GetAttachmentInfo(messageID string) ([]AttachmentInfo, error) {
	// Check cache first
	if s.cache != nil {
		cached, err := s.cache.GetAttachments(messageID)
		if err == nil && len(cached) > 0 {
			result := make([]AttachmentInfo, len(cached))
			for i, a := range cached {
				result[i] = AttachmentInfo{
					AttachmentID: a.AttachmentID,
					Filename:     a.Filename,
					MimeType:     a.MimeType,
					Size:         a.Size,
					LocalPath:    a.LocalPath,
					IsDownloaded: a.LocalPath != nil,
				}
			}
			return result, nil
		}
	}

	// Fetch message to get attachment info
	msg, err := s.client.GetMessage(messageID, "full")
	if err != nil {
		return nil, err
	}

	// Extract attachment info from message
	attachments := extractAttachmentInfo(msg.Payload)

	// Cache the attachment metadata
	if s.cache != nil {
		for _, a := range attachments {
			att := &Attachment{
				GmailID:      messageID,
				AttachmentID: a.AttachmentID,
				Filename:     a.Filename,
				MimeType:     a.MimeType,
				Size:         a.Size,
			}
			s.cache.SaveAttachment(att)
		}
	}

	return attachments, nil
}

// DownloadAttachment downloads an attachment and stores it locally
// Returns the local file path
func (s *Service) DownloadAttachment(messageID, attachmentID string) (string, error) {
	// Check if already downloaded
	if s.cache != nil {
		attachments, err := s.cache.GetAttachments(messageID)
		if err == nil {
			for _, a := range attachments {
				if a.AttachmentID == attachmentID && a.LocalPath != nil {
					// Verify file still exists
					if _, err := os.Stat(*a.LocalPath); err == nil {
						return *a.LocalPath, nil
					}
				}
			}
		}
	}

	// Get attachment info to determine filename
	attachments, err := s.GetAttachmentInfo(messageID)
	if err != nil {
		return "", err
	}

	var info *AttachmentInfo
	for _, a := range attachments {
		if a.AttachmentID == attachmentID {
			info = &a
			break
		}
	}
	if info == nil {
		return "", fmt.Errorf("attachment not found: %s", attachmentID)
	}

	// Download the attachment
	data, err := s.client.GetAttachment(messageID, attachmentID)
	if err != nil {
		return "", err
	}

	// Create directory structure
	localPath, err := s.saveAttachmentFile(messageID, info.Filename, data)
	if err != nil {
		return "", err
	}

	// Update cache with local path
	if s.cache != nil {
		s.cache.UpdateAttachmentLocalPath(messageID, attachmentID, localPath)
	}

	return localPath, nil
}

// saveAttachmentFile saves attachment data to the local filesystem
func (s *Service) saveAttachmentFile(messageID, filename string, data []byte) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Create directory: ~/.diane/attachments/{messageID}/
	dir := filepath.Join(home, ".diane", "attachments", messageID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create attachment directory: %w", err)
	}

	// Sanitize filename
	safeFilename := sanitizeFilename(filename)
	if safeFilename == "" {
		safeFilename = fmt.Sprintf("attachment_%d", time.Now().Unix())
	}

	// Handle duplicate filenames
	localPath := filepath.Join(dir, safeFilename)
	if _, err := os.Stat(localPath); err == nil {
		// File exists, add timestamp
		ext := filepath.Ext(safeFilename)
		base := strings.TrimSuffix(safeFilename, ext)
		safeFilename = fmt.Sprintf("%s_%d%s", base, time.Now().UnixNano(), ext)
		localPath = filepath.Join(dir, safeFilename)
	}

	// Write file
	if err := os.WriteFile(localPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write attachment file: %w", err)
	}

	return localPath, nil
}

// GetAttachmentPath returns the local path for an attachment if downloaded
func (s *Service) GetAttachmentPath(messageID, attachmentID string) (*string, error) {
	if s.cache == nil {
		return nil, nil
	}

	attachments, err := s.cache.GetAttachments(messageID)
	if err != nil {
		return nil, err
	}

	for _, a := range attachments {
		if a.AttachmentID == attachmentID {
			return a.LocalPath, nil
		}
	}

	return nil, nil
}

// ListDownloadedAttachments returns all locally downloaded attachments
func (s *Service) ListDownloadedAttachments() ([]Attachment, error) {
	if s.cache == nil {
		return nil, nil
	}

	return s.cache.ListDownloadedAttachments()
}

// CleanupOldAttachments removes attachment files older than the specified duration
func (s *Service) CleanupOldAttachments(maxAge time.Duration) (int, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, err
	}

	attachmentsDir := filepath.Join(home, ".diane", "attachments")
	if _, err := os.Stat(attachmentsDir); os.IsNotExist(err) {
		return 0, nil // No attachments directory
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	// Walk the attachments directory
	err = filepath.Walk(attachmentsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			return nil
		}

		// Check file age
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err == nil {
				removed++
			}
		}
		return nil
	})

	// Clean up empty directories
	filepath.Walk(attachmentsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() || path == attachmentsDir {
			return nil
		}
		// Try to remove (will fail if not empty)
		os.Remove(path)
		return nil
	})

	return removed, err
}

// Helper functions

// extractAttachmentInfo extracts attachment metadata from message parts
func extractAttachmentInfo(part *gmail.MessagePart) []AttachmentInfo {
	var result []AttachmentInfo

	if part == nil {
		return result
	}

	// Check if this part is an attachment
	if part.Filename != "" && part.Body != nil && part.Body.AttachmentId != "" {
		result = append(result, AttachmentInfo{
			AttachmentID: part.Body.AttachmentId,
			Filename:     part.Filename,
			MimeType:     part.MimeType,
			Size:         part.Body.Size,
			IsDownloaded: false,
		})
	}

	// Recurse into sub-parts
	for _, subPart := range part.Parts {
		result = append(result, extractAttachmentInfo(subPart)...)
	}

	return result
}

// sanitizeFilename removes dangerous characters from filenames
func sanitizeFilename(filename string) string {
	// Remove path separators and other dangerous characters
	dangerous := []string{"/", "\\", "..", ":", "*", "?", "\"", "<", ">", "|", "\x00"}
	result := filename
	for _, d := range dangerous {
		result = strings.ReplaceAll(result, d, "_")
	}

	// Trim spaces and dots from edges
	result = strings.TrimSpace(result)
	result = strings.Trim(result, ".")

	// Limit length
	if len(result) > 200 {
		ext := filepath.Ext(result)
		base := result[:200-len(ext)]
		result = base + ext
	}

	return result
}
