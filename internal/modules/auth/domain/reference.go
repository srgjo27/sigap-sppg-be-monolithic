package domain

// SPPGInfo is the minimal SPPG reference for profile responses (MVP-001.4).
type SPPGInfo struct {
	ID   int64
	Nama string
}

// SekolahInfo is the minimal sekolah reference for profile responses.
type SekolahInfo struct {
	ID   int64
	Nama string
}
