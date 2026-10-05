-- ===== ENUM =====
CREATE TYPE user_role AS ENUM ('admin','kepala_sppg','ahli_gizi','akuntan','petugas_dapur','petugas_distribusi','pic_sekolah','pengawas');
CREATE TYPE slhs_status AS ENUM ('belum','proses','terbit','kedaluwarsa');
CREATE TYPE penerima_jenis AS ENUM ('sekolah','posyandu','pesantren');
CREATE TYPE menu_status AS ENUM ('draf','disetujui');
CREATE TYPE po_status AS ENUM ('draf','dikirim','diterima_sebagian','selesai','batal');
CREATE TYPE kondisi_bahan AS ENUM ('baik','kurang','rusak');
CREATE TYPE tahap_produksi AS ENUM ('penerimaan','penyimpanan','persiapan','pemasakan','pemorsian','pengemasan');
CREATE TYPE batch_status AS ENUM ('dimasak','menunggu_persetujuan','siap_kirim','dikirim','selesai','kedaluwarsa','investigasi','ditarik');
CREATE TYPE kirim_status AS ENUM ('dijadwalkan','berangkat','tiba','diterima','ditolak','terlambat');
CREATE TYPE kondisi_makanan AS ENUM ('baik','kurang','tidak_layak');
CREATE TYPE aduan_kategori AS ENUM ('gejala_kesehatan','mutu_makanan','benda_asing','keterlambatan','jumlah_kurang','lainnya');
CREATE TYPE aduan_prioritas AS ENUM ('kritis','tinggi','normal');
CREATE TYPE aduan_status AS ENUM ('baru','diproses','selesai','ditolak');
CREATE TYPE siklus_status AS ENUM ('draf','diajukan','dicairkan','dilaporkan','ditutup');
CREATE TYPE dana_kategori AS ENUM ('bahan_baku','operasional','insentif');

-- ===== MASTER & AKSES =====
CREATE TABLE sppg (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  kode VARCHAR(20) NOT NULL UNIQUE,
  nama VARCHAR(150) NOT NULL,
  alamat TEXT,
  latitude NUMERIC(9,6), longitude NUMERIC(9,6),
  nama_yayasan VARCHAR(150),
  status_slhs slhs_status NOT NULL DEFAULT 'belum',
  tgl_slhs_berakhir DATE,
  kapasitas_porsi INT CHECK (kapasitas_porsi > 0),
  batas_konsumsi_jam NUMERIC(3,1) NOT NULL DEFAULT 4,
  aktif BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sekolah (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  sppg_id BIGINT NOT NULL REFERENCES sppg(id),
  npsn VARCHAR(20) UNIQUE,
  nama VARCHAR(150) NOT NULL,
  jenis penerima_jenis NOT NULL DEFAULT 'sekolah',
  jenjang VARCHAR(20),
  jumlah_penerima INT NOT NULL CHECK (jumlah_penerima >= 0),
  alamat TEXT,
  latitude NUMERIC(9,6), longitude NUMERIC(9,6),
  urutan_rute INT,
  waktu_makan TIME,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  sppg_id BIGINT REFERENCES sppg(id),
  sekolah_id BIGINT REFERENCES sekolah(id),
  nama VARCHAR(120) NOT NULL,
  email VARCHAR(150) NOT NULL UNIQUE,
  no_hp VARCHAR(20),
  password_hash VARCHAR(255) NOT NULL,
  peran user_role NOT NULL,
  aktif BOOLEAN NOT NULL DEFAULT true,
  last_login_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (peran <> 'pic_sekolah' OR sekolah_id IS NOT NULL)
);

CREATE TABLE pemasok (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  sppg_id BIGINT NOT NULL REFERENCES sppg(id),
  nama VARCHAR(150) NOT NULL,
  jenis_usaha VARCHAR(30),
  kontak VARCHAR(100), no_hp VARCHAR(20),
  alamat TEXT,
  aktif BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE bahan (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  nama VARCHAR(100) NOT NULL UNIQUE,
  kategori VARCHAR(30) NOT NULL,
  satuan VARCHAR(10) NOT NULL,
  mudah_rusak BOOLEAN NOT NULL DEFAULT false,
  suhu_simpan_maks NUMERIC(4,1),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ===== KEUANGAN (dibuat lebih dulu karena dirujuk PO) =====
CREATE TABLE siklus_dana (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  sppg_id BIGINT NOT NULL REFERENCES sppg(id),
  periode_mulai DATE NOT NULL,
  periode_selesai DATE NOT NULL,
  dana_diajukan NUMERIC(14,2) NOT NULL DEFAULT 0,
  dana_diterima NUMERIC(14,2),
  sisa_siklus_lalu NUMERIC(14,2) NOT NULL DEFAULT 0,
  status siklus_status NOT NULL DEFAULT 'draf',
  diajukan_at TIMESTAMPTZ, dicairkan_at TIMESTAMPTZ, dilaporkan_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (periode_selesai >= periode_mulai),
  UNIQUE (sppg_id, periode_mulai)
);

-- ===== MENU & PENGADAAN =====
CREATE TABLE menu (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  sppg_id BIGINT NOT NULL REFERENCES sppg(id),
  tanggal DATE NOT NULL,
  nama_menu VARCHAR(200) NOT NULL,
  energi_kkal NUMERIC(6,1), protein_g NUMERIC(6,1),
  karbohidrat_g NUMERIC(6,1), lemak_g NUMERIC(6,1),
  target_porsi INT NOT NULL CHECK (target_porsi > 0),
  dibuat_oleh BIGINT NOT NULL REFERENCES users(id),
  status menu_status NOT NULL DEFAULT 'draf',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (sppg_id, tanggal)
);

CREATE TABLE menu_bahan (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  menu_id BIGINT NOT NULL REFERENCES menu(id) ON DELETE CASCADE,
  bahan_id BIGINT NOT NULL REFERENCES bahan(id),
  gram_per_porsi NUMERIC(8,2) NOT NULL CHECK (gram_per_porsi > 0),
  UNIQUE (menu_id, bahan_id)
);

CREATE TABLE purchase_order (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  sppg_id BIGINT NOT NULL REFERENCES sppg(id),
  nomor_po VARCHAR(30) NOT NULL UNIQUE,
  pemasok_id BIGINT NOT NULL REFERENCES pemasok(id),
  siklus_id BIGINT REFERENCES siklus_dana(id),
  tgl_po DATE NOT NULL,
  tgl_kirim_diminta DATE,
  total NUMERIC(14,2) NOT NULL DEFAULT 0,
  status po_status NOT NULL DEFAULT 'draf',
  dibuat_oleh BIGINT NOT NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE po_item (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  po_id BIGINT NOT NULL REFERENCES purchase_order(id) ON DELETE CASCADE,
  bahan_id BIGINT NOT NULL REFERENCES bahan(id),
  qty_pesan NUMERIC(10,2) NOT NULL CHECK (qty_pesan > 0),
  harga_satuan NUMERIC(14,2) NOT NULL CHECK (harga_satuan >= 0),
  subtotal NUMERIC(14,2) GENERATED ALWAYS AS (qty_pesan * harga_satuan) STORED
);

CREATE TABLE penerimaan_bahan (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  po_item_id BIGINT NOT NULL REFERENCES po_item(id),
  kode_lot VARCHAR(30) NOT NULL UNIQUE,
  waktu_terima TIMESTAMPTZ NOT NULL DEFAULT now(),
  qty_diterima NUMERIC(10,2) NOT NULL CHECK (qty_diterima >= 0),
  qty_ditolak NUMERIC(10,2) NOT NULL DEFAULT 0 CHECK (qty_ditolak >= 0),
  alasan_tolak TEXT,
  suhu_terima NUMERIC(4,1),
  kondisi kondisi_bahan NOT NULL,
  tgl_kedaluwarsa DATE,
  foto_url TEXT NOT NULL,
  diperiksa_oleh BIGINT NOT NULL REFERENCES users(id),
  nilai_potong_nota NUMERIC(14,2) NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (qty_ditolak = 0 OR alasan_tolak IS NOT NULL)
);

-- ===== PRODUKSI & TRACEABILITY =====
CREATE TABLE batch_produksi (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  sppg_id BIGINT NOT NULL REFERENCES sppg(id),
  kode_batch VARCHAR(30) NOT NULL UNIQUE,
  menu_id BIGINT NOT NULL REFERENCES menu(id),
  jumlah_porsi INT NOT NULL CHECK (jumlah_porsi > 0),
  mulai_masak TIMESTAMPTZ,
  selesai_masak TIMESTAMPTZ,
  batas_konsumsi TIMESTAMPTZ,
  status batch_status NOT NULL DEFAULT 'dimasak',
  disetujui_oleh BIGINT REFERENCES users(id),
  disetujui_at TIMESTAMPTZ,
  catatan TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (status NOT IN ('siap_kirim','dikirim','selesai') OR disetujui_at IS NOT NULL),
  CHECK (selesai_masak IS NULL OR mulai_masak IS NULL OR selesai_masak >= mulai_masak)
);

CREATE TABLE batch_bahan (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  batch_id BIGINT NOT NULL REFERENCES batch_produksi(id) ON DELETE CASCADE,
  penerimaan_id BIGINT NOT NULL REFERENCES penerimaan_bahan(id),
  qty_dipakai NUMERIC(10,2) NOT NULL CHECK (qty_dipakai > 0),
  UNIQUE (batch_id, penerimaan_id)
);

CREATE TABLE titik_kritis (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  kode VARCHAR(20) NOT NULL UNIQUE,
  tahap tahap_produksi NOT NULL,
  pertanyaan TEXT NOT NULL,
  tipe_nilai VARCHAR(10) NOT NULL CHECK (tipe_nilai IN ('ya_tidak','angka','foto')),
  nilai_min NUMERIC(6,2), nilai_maks NUMERIC(6,2),
  wajib_foto BOOLEAN NOT NULL DEFAULT false,
  urutan INT NOT NULL,
  aktif BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE checklist_sop (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  batch_id BIGINT NOT NULL REFERENCES batch_produksi(id) ON DELETE CASCADE,
  titik_kritis_id BIGINT NOT NULL REFERENCES titik_kritis(id),
  nilai_angka NUMERIC(6,2),
  nilai_ya BOOLEAN,
  lolos BOOLEAN NOT NULL,
  foto_url TEXT,
  diisi_oleh BIGINT NOT NULL REFERENCES users(id),
  diisi_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  tindakan_koreksi TEXT,
  UNIQUE (batch_id, titik_kritis_id),
  CHECK (lolos OR tindakan_koreksi IS NOT NULL)
);

CREATE TABLE sampel_retensi (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  batch_id BIGINT NOT NULL UNIQUE REFERENCES batch_produksi(id),
  lokasi_simpan VARCHAR(50) NOT NULL,
  disimpan_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  dibuang_at TIMESTAMPTZ,
  dikirim_lab_at TIMESTAMPTZ,
  hasil_lab TEXT
);

-- ===== DISTRIBUSI & PENGADUAN =====
CREATE TABLE pengiriman (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  batch_id BIGINT NOT NULL REFERENCES batch_produksi(id),
  sekolah_id BIGINT NOT NULL REFERENCES sekolah(id),
  jumlah_porsi INT NOT NULL CHECK (jumlah_porsi > 0),
  petugas_id BIGINT REFERENCES users(id),
  kendaraan VARCHAR(50),
  kendaraan_tertutup BOOLEAN NOT NULL DEFAULT true,
  foto_muat_url TEXT,
  berangkat_at TIMESTAMPTZ,
  tiba_at TIMESTAMPTZ,
  lat_tiba NUMERIC(9,6), lng_tiba NUMERIC(9,6),
  status kirim_status NOT NULL DEFAULT 'dijadwalkan',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (batch_id, sekolah_id),
  CHECK (tiba_at IS NULL OR berangkat_at IS NULL OR tiba_at >= berangkat_at)
);

CREATE TABLE konfirmasi_terima (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  pengiriman_id BIGINT NOT NULL UNIQUE REFERENCES pengiriman(id),
  jumlah_diterima INT NOT NULL CHECK (jumlah_diterima >= 0),
  kondisi kondisi_makanan NOT NULL,
  suhu_saat_terima NUMERIC(4,1),
  rating_rasa SMALLINT CHECK (rating_rasa BETWEEN 1 AND 5),
  catatan TEXT,
  foto_url TEXT,
  dikonfirmasi_oleh BIGINT NOT NULL REFERENCES users(id),
  dikonfirmasi_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE pengaduan (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  nomor_tiket VARCHAR(30) NOT NULL UNIQUE,
  sppg_id BIGINT NOT NULL REFERENCES sppg(id),
  sekolah_id BIGINT NOT NULL REFERENCES sekolah(id),
  batch_id BIGINT REFERENCES batch_produksi(id),
  kategori aduan_kategori NOT NULL,
  prioritas aduan_prioritas NOT NULL DEFAULT 'normal',
  jumlah_terdampak INT CHECK (jumlah_terdampak >= 0),
  deskripsi TEXT NOT NULL,
  foto_url TEXT,
  status aduan_status NOT NULL DEFAULT 'baru',
  dilaporkan_oleh BIGINT NOT NULL REFERENCES users(id),
  batas_respons TIMESTAMPTZ NOT NULL,
  selesai_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (kategori <> 'gejala_kesehatan' OR prioritas = 'kritis')
);

CREATE TABLE tindak_lanjut (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  pengaduan_id BIGINT NOT NULL REFERENCES pengaduan(id) ON DELETE CASCADE,
  oleh BIGINT NOT NULL REFERENCES users(id),
  tindakan TEXT NOT NULL,
  status_baru aduan_status NOT NULL,
  lampiran_url TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ===== KEUANGAN & SISTEM =====
CREATE TABLE transaksi_dana (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  siklus_id BIGINT NOT NULL REFERENCES siklus_dana(id),
  tanggal DATE NOT NULL,
  kategori dana_kategori NOT NULL,
  po_id BIGINT REFERENCES purchase_order(id),
  uraian TEXT NOT NULL,
  nominal NUMERIC(14,2) NOT NULL CHECK (nominal > 0),
  bukti_url TEXT NOT NULL,
  diajukan_oleh BIGINT NOT NULL REFERENCES users(id),
  disetujui_oleh BIGINT REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (disetujui_oleh IS NULL OR disetujui_oleh <> diajukan_oleh),
  CHECK (kategori <> 'bahan_baku' OR po_id IS NOT NULL)
);

CREATE TABLE notifikasi (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  jenis VARCHAR(40) NOT NULL,
  judul TEXT NOT NULL,
  isi TEXT,
  ref_tabel VARCHAR(40), ref_id BIGINT,
  kanal VARCHAR(10) NOT NULL DEFAULT 'app',
  dibaca_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE audit_log (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id BIGINT REFERENCES users(id),
  aksi VARCHAR(10) NOT NULL,
  tabel VARCHAR(40) NOT NULL,
  record_id BIGINT,
  data_lama JSONB, data_baru JSONB,
  ip_address INET,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ===== INDEKS =====
CREATE INDEX idx_batch_sppg_status ON batch_produksi (sppg_id, status);
CREATE INDEX idx_batch_menu ON batch_produksi (menu_id);
CREATE INDEX idx_checklist_batch ON checklist_sop (batch_id);
CREATE INDEX idx_batch_bahan_lot ON batch_bahan (penerimaan_id);
CREATE INDEX idx_kirim_batch ON pengiriman (batch_id);
CREATE INDEX idx_kirim_sekolah_status ON pengiriman (sekolah_id, status);
CREATE INDEX idx_aduan_batch_status ON pengaduan (batch_id, status);
CREATE INDEX idx_aduan_sppg_status ON pengaduan (sppg_id, status);
CREATE INDEX idx_trx_siklus_kat ON transaksi_dana (siklus_id, kategori);
CREATE INDEX idx_notif_user_unread ON notifikasi (user_id) WHERE dibaca_at IS NULL;
CREATE INDEX idx_audit_ref ON audit_log (tabel, record_id);

-- ===== Additional schemes =====
CREATE TABLE refresh_token (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash CHAR(64) NOT NULL UNIQUE,   -- SHA-256, token asli tidak disimpan
  user_agent TEXT,
  ip_address INET,
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_refresh_user ON refresh_token (user_id) WHERE revoked_at IS NULL;

ALTER TABLE users ADD COLUMN gagal_login SMALLINT NOT NULL DEFAULT 0,
                  ADD COLUMN terkunci_sampai TIMESTAMPTZ,
                  ADD COLUMN wajib_ganti_password BOOLEAN NOT NULL DEFAULT true;