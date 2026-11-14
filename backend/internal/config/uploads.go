package config

import (
	"errors"
	"os"
	"strconv"
)

// UploadLimits applies to the shared storage owned by one account.
type UploadLimits struct {
	Bytes    int64
	Files    int64
	Requests int64
}

func DefaultUploadLimits() UploadLimits {
	return UploadLimits{Bytes: 100 << 20, Files: 100, Requests: 20}
}

func (l UploadLimits) Validate() error {
	if l.Bytes < 1 || l.Bytes > 1<<40 || l.Files < 1 || l.Files > 100_000 || l.Requests < 1 || l.Requests > 1000 {
		return errors.New("upload limits must be finite: bytes 1..1099511627776, files 1..100000, requests per minute 1..1000")
	}
	return nil
}

func loadUploadLimits() *UploadLimits {
	defaults := DefaultUploadLimits()
	return &UploadLimits{
		Bytes:    uploadLimitEnv("SAILOR_UPLOAD_MAX_BYTES", defaults.Bytes),
		Files:    uploadLimitEnv("SAILOR_UPLOAD_MAX_FILES", defaults.Files),
		Requests: uploadLimitEnv("SAILOR_UPLOAD_REQUESTS_PER_MINUTE", defaults.Requests),
	}
}

func uploadLimitEnv(name string, fallback int64) int64 {
	value, set := os.LookupEnv(name)
	if !set {
		return fallback
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return -1
	}
	return n
}
