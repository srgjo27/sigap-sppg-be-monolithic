package domain

import (
	"errors"
	"time"
)

// Sentinel domain errors for the berkas module.
var (
	ErrKonteksUnknown   = errors.New("konteks tidak dikenal")
	ErrKonteksForbidden = errors.New("peran tidak berhak untuk konteks ini")
	ErrFileEmpty        = errors.New("file kosong")
	ErrFileTooLarge     = errors.New("file lebih dari 5 MB")
	ErrUnsupportedImage = errors.New("tipe file tidak didukung")
	ErrSPPGNotFound     = errors.New("sppg not found")
	ErrUnauthorized     = errors.New("unauthorized")
)

// MaxUploadBytes is the hard limit for one uploaded image: 5 MB.
const MaxUploadBytes = 5 << 20 // 5 * 1024 * 1024

// PresignTTL is how long a returned presigned URL stays valid.
const PresignTTL = 15 * time.Minute
