# MVP-003 Pengadaan & penerimaan bahan

Spesifikasi modul M3 (P0): master pemasok, rekap kebutuhan pengadaan dari menu yang sudah disetujui, purchase order (PO) ke pemasok, pemeriksaan penerimaan bahan (jumlah, kondisi, suhu, foto), serta retur dan potong nota otomatis. Dipecah menjadi 10 endpoint yang masing-masing bisa diuji terpisah. Semua endpoint berprefiks `/api/v1`, dilindungi middleware RBAC dari MVP-001.8, dan memakai tabel `pemasok`, `purchase_order`, `po_item`, `penerimaan_bahan`, `bahan`, `menu`, `menu_bahan`, `siklus_dana`, `notifikasi`, `audit_log`, ditambah dua tabel baru `po_menu` dan `berkas`.

## Main Feature

PO ke pemasok, cek penerimaan (jumlah, kondisi, suhu, foto), retur/potong nota

## Ringkasan endpoint

| ID | Endpoint | Fungsi | Akses |
| --- | --- | --- | --- |
| MVP-003.1 | GET/POST/PATCH /pemasok, GET /pemasok/{id} | Kelola master pemasok dan lihat kinerjanya | baca: admin, kepala\_sppg, akuntan, ahli\_gizi, pengawas; tulis: admin, kepala\_sppg, akuntan |
| MVP-003.2 | GET /pengadaan/kebutuhan | Rekap kebutuhan bahan dari menu disetujui, dikurangi yang sudah dipesan | akuntan, ahli\_gizi, kepala\_sppg |
| MVP-003.3 | POST /purchase-orders | Buat PO (draf) | akuntan |
| MVP-003.4 | GET /purchase-orders, GET /purchase-orders/{id} | Daftar dan detail PO beserta rekap penerimaan | semua peran SPPG kecuali petugas\_distribusi, pengawas, admin |
| MVP-003.5 | PATCH /purchase-orders/{id}, DELETE /purchase-orders/{id} | Ubah atau hapus PO draf | akuntan |
| MVP-003.6 | POST /purchase-orders/{id}/kirim, /batal, /tutup | Kirim PO ke pemasok, batalkan, atau tutup manual | akuntan; batal juga kepala\_sppg |
| MVP-003.7 | POST /uploads | Unggah foto bukti (dipakai ulang M4, M6, M7) | semua peran kecuali pengawas, sesuai konteks |
| MVP-003.8 | POST /purchase-orders/{id}/penerimaan | Catat kedatangan bahan per lot | akuntan, petugas\_dapur |
| MVP-003.9 | GET /penerimaan, GET /penerimaan/{id} | Daftar lot bahan dan sisa stoknya | akuntan, petugas\_dapur, ahli\_gizi, kepala\_sppg, pengawas, admin |
| MVP-003.10 | POST /penerimaan/{id}/koreksi | Koreksi hasil penerimaan dengan jejak audit | kepala\_sppg |

## Matriks peran

| Peran | Master pemasok | Buat/ubah/kirim PO | Catat penerimaan | Koreksi penerimaan | Lihat PO & lot |
| --- | --- | --- | --- | --- | --- |
| admin | Tulis, semua SPPG | Tidak | Tidak | Tidak | Semua SPPG |
| kepala\_sppg | Tulis, SPPG sendiri | Hanya batal | Tidak | Ya, SPPG sendiri | SPPG sendiri |
| akuntan | Tulis, SPPG sendiri | Ya, SPPG sendiri | Ya | Tidak | SPPG sendiri |
| ahli\_gizi | Baca | Tidak | Tidak | Tidak | SPPG sendiri |
| petugas\_dapur | Tidak | Tidak | Ya | Tidak | PO berstatus dikirim ke atas, tanpa harga; semua lot SPPG sendiri |
| petugas\_distribusi | Tidak | Tidak | Tidak | Tidak | Tidak |
| pic\_sekolah | Tidak | Tidak | Tidak | Tidak | Tidak |
| pengawas | Baca | Tidak | Tidak | Tidak | Semua SPPG, baca saja |

Petugas dapur (relawan) tidak melihat harga satuan, subtotal, total, atau nilai potong nota. Field tersebut dihapus dari response, bukan dikirim sebagai `null`.

## Alur status PO

```
draf ──kirim──▶ dikirim ──penerimaan──▶ diterima_sebagian ──penerimaan (semua item terpenuhi)──▶ selesai
  │               │  └────────penerimaan (semua item terpenuhi sekaligus)──────────────────────▶ selesai
  │               │                         └──tutup manual (sisa tidak akan dikirim)──────────▶ selesai
  └──batal──▶ batal ◀──batal (belum ada penerimaan)──┘
```

| Transisi | Pemicu | Pelaku |
| --- | --- | --- |
| draf → dikirim | MVP-003.6 kirim | akuntan |
| dikirim → diterima\_sebagian / selesai | otomatis setelah MVP-003.8 atau MVP-003.10 | sistem |
| diterima\_sebagian → selesai | otomatis bila semua item terpenuhi, atau MVP-003.6 tutup | sistem / akuntan |
| selesai → diterima\_sebagian | otomatis bila koreksi (MVP-003.10) membuat item tidak lagi terpenuhi, kecuali PO ditutup manual | sistem |
| draf / dikirim → batal | MVP-003.6 batal, hanya bila belum ada penerimaan | akuntan, kepala\_sppg |

Satu item dianggap **terpenuhi** bila total `qty_diterima` dari semua lotnya ≥ `qty_pesan`. Bahan yang ditolak tidak dihitung, sehingga pemasok masih bisa mengirim pengganti dalam PO yang sama.

## Tambahan skema

Tambahan ini menyelesaikan empat hal yang belum diatur di Rancangan Database:

- `purchase_order.total` sekarang dijaga trigger agar selalu sama dengan jumlah `po_item.subtotal`.
- `nilai_potong_nota` dihitung aplikasi saat penerimaan. Kolom ini tidak bisa dijadikan generated column karena `harga_satuan` ada di tabel lain.
- PO ditautkan ke menu lewat tabel baru `po_menu`.
- Foto bukti dicatat di tabel baru `berkas`.

Skrip sudah diuji di PostgreSQL 16 setelah DDL Rancangan Database, tambahan skema MVP-001, dan tambahan skema MVP-002. Kolom `penerimaan_bahan.sppg_id` dan `client_ref` bertipe NOT NULL tanpa default, jadi jalankan saat tabel tersebut masih kosong.
Read `/db/schema.sql` before work.

Yang sudah diverifikasi saat uji:

- total PO ikut berubah saat item ditambah, diubah, atau dihapus
- nama pemasok yang sama dengan huruf berbeda ditolak
- bahan duplikat dalam satu PO ditolak
- PO tidak bisa berstatus `dikirim` tanpa `dikirim_at`, dan tidak bisa `batal` tanpa alasan
- lot `rusak` atau lot yang gagal suhu tidak bisa punya `qty_diterima > 0`
- `Idempotency-Key` yang sama untuk item yang sama ditolak database
- berkas tidak bisa ditandai terpakai tanpa rujukan

## Aturan umum

- `sppg_id` selalu diambil dari token, kecuali admin yang wajib mengirimkannya di body atau query.
- Qty selalu dalam satuan bahan (`bahan.satuan`). Satuan `butir` dan `pcs` wajib bilangan bulat, satuan lain maksimal 2 desimal.
- Uang dalam rupiah `NUMERIC(14,2)` dan dikirim sebagai **string** (mis. `"38000.00"`) agar tidak ada galat floating point. Angka non-uang mengikuti MVP-002.
- "Hari ini" dihitung dengan zona Asia/Jakarta. Waktu dikirim dalam ISO 8601 dengan zona waktu (MVP-001).
- Nomor PO dibuat server dengan format `PO/{kode_sppg}/{YYYYMM}/{urut 4 digit}`. Urutannya per SPPG per bulan dan dibuat di dalam transaksi dengan `pg_advisory_xact_lock`, sehingga tidak ada nomor ganda saat dua akuntan membuat PO bersamaan.
- Kode lot dibuat server dengan format `LOT-{kode_sppg}-{YYYYMMDD}-{urut 3 digit}` (≤ 30 karakter), lalu dicetak atau ditulis di wadah bahan.
- Penerimaan bersifat append-only. Satu-satunya cara mengubahnya adalah koreksi lewat MVP-003.10.
- **Nilai tagihan dan potong nota.** Nilai nota satu kedatangan dihitung dari `(qty_diterima + qty_ditolak) × harga_satuan`. Potong nota = `qty_ditolak × harga_satuan`, sehingga yang ditagihkan hanya bahan yang diterima. Tagihan per item dibatasi `qty_pesan`: kelebihan kirim tidak ditagihkan.
- Semua foto berasal dari MVP-003.7. Response API mengembalikan URL presigned yang berlaku 15 menit, bukan object key mentah.
- Semua perubahan pemasok, PO, dan penerimaan dicatat di `audit_log`. Format error mengikuti MVP-001.

**Kode error khusus modul ini** (dipakai di `error.code`)

| Kode | HTTP | Arti |
| --- | --- | --- |
| STATUS\_TIDAK\_SESUAI | 409 | Aksi tidak berlaku untuk status PO saat ini |
| PO\_SUDAH\_ADA\_PENERIMAAN | 409 | PO tidak bisa dibatalkan karena sudah ada penerimaan |
| LOT\_SUDAH\_DIPAKAI | 409 | Lot sudah dipakai batch produksi sehingga tidak bisa dikoreksi |
| SUHU\_MELEBIHI\_BATAS | 422 | Bahan mudah rusak di atas `suhu_simpan_maks` tidak boleh diterima |
| BAHAN\_KEDALUWARSA | 422 | Tanggal kedaluwarsa sudah lewat tetapi ada qty diterima |
| MELEBIHI\_PESANAN | 422 | Total diterima melebihi toleransi 10% dari qty pesan |
| BERKAS\_TIDAK\_VALID | 422 | Foto tidak ditemukan, sudah dipakai, kedaluwarsa, atau beda konteks/SPPG |
| PEMASOK\_NONAKTIF | 422 | Pemasok nonaktif tidak bisa dipilih untuk PO baru atau dikirimi PO |

## Job terjadwal

| Job | Jadwal | Aksi |
| --- | --- | --- |
| PO terlambat | setiap hari 06.00 WIB | PO berstatus `dikirim` atau `diterima_sebagian` dengan `tgl_kirim_diminta` < hari ini dan item belum terpenuhi → notifikasi `po_terlambat` ke akuntan dan kepala\_sppg, maksimal sekali per PO per hari |
| Bersihkan berkas | setiap jam | Berkas yang tidak dipakai lebih dari 24 jam dihapus dari object storage dan dari tabel `berkas` |

---

## MVP-003.1 Master pemasok

### Goal

Akuntan dan kepala SPPG mengelola daftar pemasok dapur, menonaktifkan pemasok bermasalah, dan melihat kinerja pemasok berdasarkan riwayat penerimaan.

### Actor

baca: admin, kepala\_sppg, akuntan, ahli\_gizi, pengawas; tulis: admin, kepala\_sppg, akuntan

### Input

`POST /api/v1/pemasok` dan `PATCH /api/v1/pemasok/{id}`

- nama
- jenis\_usaha (UMKM, PT, CV, koperasi, petani, lainnya)
- kontak (nama orang yang bisa dihubungi, opsional)
- no\_hp
- alamat (opsional)
- aktif (hanya PATCH)
- alasan (wajib bila PATCH `aktif = false`)
- sppg\_id (hanya admin)

`GET /api/v1/pemasok?q=&jenis_usaha=&aktif=&page=&per_page=` untuk pencarian.

`GET /api/v1/pemasok/{id}` untuk detail dan kinerja.

### Rules

- nama wajib, 2–150 karakter, unik per SPPG (case-insensitive)
- jenis\_usaha wajib, dari daftar yang diizinkan
- no\_hp wajib, format Indonesia (`08…` atau `+628…`), 10–15 digit, disimpan dalam format `+628…`
- pemasok tidak bisa dihapus, hanya dinonaktifkan. Alasan penonaktifan dicatat di audit\_log.
- pemasok nonaktif tidak muncul saat membuat PO baru, tetapi PO yang sudah berjalan tetap bisa menerima kiriman
- detail menyertakan `kinerja` untuk 90 hari terakhir, dihitung dari PO berstatus selain draf dan batal:
  - `jumlah_po`, `total_nilai_po`, `total_potong_nota`
  - `persen_ditolak` = Σ nilai\_potong\_nota ÷ Σ nilai nota kedatangan × 100 (berbasis rupiah, sehingga bahan dengan satuan berbeda bisa dijumlahkan)
  - `persen_tepat_waktu` = PO dengan penerimaan pertama ≤ `tgl_kirim_diminta` ÷ PO yang sudah punya penerimaan × 100
- default urutan nama naik; `per_page` default 20, maks 100

### Success

POST: HTTP 201, returns created pemasok

PATCH: HTTP 200, returns updated pemasok

GET list: HTTP 200, returns `data[]` (id, nama, jenis\_usaha, kontak, no\_hp, aktif) dan `meta` (page, per\_page, total)

GET detail: HTTP 200, returns pemasok lengkap + `kinerja {periode_hari, jumlah_po, total_nilai_po, total_potong_nota, persen_ditolak, persen_tepat_waktu}`

### Failure

400: invalid input (termasuk alasan kosong saat menonaktifkan)

401: tidak login

403: peran tidak berhak menulis

404: pemasok not found atau milik SPPG lain

409: nama pemasok already exists di SPPG ini

500: unexpected internal error

### Acceptance Criteria

- [ ] Request is validated.
- [ ] Duplicate name within the same SPPG (any letter case) is rejected with 409; the same name in another SPPG is allowed.
- [ ] Deactivation requires a reason, and the supplier is hidden from new PO input.
- [ ] Performance metrics match the formulas over the last 90 days.
- [ ] Data is persisted to PostgreSQL and written to audit\_log.
- [ ] Unit tests exist.
- [ ] Integration test exists.

---

## MVP-003.2 Rekap kebutuhan pengadaan

### Goal

Akuntan melihat total bahan yang harus dipesan untuk satu rentang tanggal, dari semua menu yang sudah disetujui, dikurangi yang sudah dipesan, sebagai dasar menyusun PO per pemasok.

### Actor

akuntan, ahli\_gizi, kepala\_sppg

### Input

`GET /api/v1/pengadaan/kebutuhan`

- dari, sampai (wajib; maks 14 hari)
- cadangan\_persen (opsional, default 0, maks 20)
- kategori (opsional)

### Rules

- hanya menu berstatus `disetujui` di SPPG pengguna yang dihitung
- menu draf dalam rentang tidak dihitung, tetapi dilaporkan di `meta.menu_belum_disetujui[]` (tanggal, menu\_id, nama\_menu)
- qty per menu dihitung persis seperti MVP-002.6 (rumus, konversi satuan, pembulatan ke atas). `qty_kebutuhan` = jumlah qty per menu, sehingga angkanya sama dengan yang dilihat ahli gizi per menu.
- `qty_sudah_dipesan` = Σ `po_item.qty_pesan` untuk bahan tersebut dari PO berstatus selain `batal` yang tertaut (`po_menu`) ke minimal satu menu dalam rentang. PO yang juga tertaut ke menu di luar rentang tetap dihitung penuh, dan ditandai `meta.po_lintas_rentang[]` agar akuntan memeriksanya.
- `qty_sisa` = max(0, qty\_kebutuhan − qty\_sudah\_dipesan)
- `tanggal_pakai_pertama` disertakan per bahan agar bahan mudah rusak dipesan mendekati hari pakai
- endpoint hanya membaca, tidak menyimpan apa pun

### Success

HTTP 200

Returns `meta {dari, sampai, cadangan_persen, jumlah_menu, menu_belum_disetujui[], po_lintas_rentang[]}` dan `items[]` {bahan\_id, nama, kategori, satuan, mudah\_rusak, tanggal\_pakai\_pertama, total\_gram, qty\_kebutuhan, qty\_sudah\_dipesan, qty\_sisa, rincian[] {tanggal, menu\_id, qty}}

### Failure

400: invalid input (rentang terbalik atau lebih dari 14 hari, cadangan di luar 0–20)

401: tidak login

403: peran tidak berhak

500: unexpected internal error

### Acceptance Criteria

- [ ] Only approved menus are counted; draft menus are reported in meta.
- [ ] Per-menu quantities equal MVP-002.6 results for the same menu.
- [ ] Quantities already ordered in non-cancelled linked POs are subtracted.
- [ ] Endpoint does not change stored data.
- [ ] Unit tests cover aggregation and rounding.
- [ ] Integration test exists.

---

## MVP-003.3 Create purchase order

### Goal

Akuntan membuat PO untuk satu pemasok, berisi bahan, qty, dan harga, yang bisa ditautkan ke menu yang menjadi dasar pemesanan.

### Actor

akuntan

### Input

`POST /api/v1/purchase-orders`

- pemasok\_id
- tgl\_kirim\_diminta
- siklus\_id (opsional pada MVP)
- menu\_ids\[\] (opsional)
- catatan (opsional)
- items\[\]: { bahan\_id, qty\_pesan, harga\_satuan }

### Rules

- `sppg_id` diambil dari token; `tgl_po` = hari ini; `dibuat_oleh` = user dari token; status selalu `draf`
- pemasok harus ada, aktif, dan milik SPPG yang sama
- `tgl_kirim_diminta` wajib dan tidak boleh sebelum hari ini
- `siklus_id` bila dikirim harus milik SPPG yang sama dan tidak berstatus `ditutup`. Mulai M8 field ini menjadi wajib.
- setiap `menu_ids[]` harus berstatus `disetujui` dan milik SPPG yang sama; maks 14 menu
- items\[\] berisi 1–50 item; setiap bahan\_id harus ada, aktif, dan tidak duplikat
- qty\_pesan > 0, bilangan bulat untuk satuan butir dan pcs
- harga\_satuan > 0, maks 100.000.000 (cegah salah ketik)
- nomor\_po dibuat server; PO, item, dan tautan menu disimpan dalam satu transaksi; total dihitung trigger
- `warnings[]` (tetap diterima, tidak menolak):
  - qty\_pesan melebihi `qty_sisa` dari MVP-003.2 lebih dari 20% untuk menu yang ditautkan
  - bahan `mudah_rusak` dengan `tgl_kirim_diminta` lebih dari 1 hari sebelum tanggal menu pertama yang memakainya
  - bahan dipesan tetapi tidak dipakai di menu yang ditautkan

### Success

HTTP 201

Returns created PO: `id, nomor_po, status, pemasok {id, nama}, tgl_po, tgl_kirim_diminta, siklus_id, menu_ids[], catatan, items[] {id, bahan_id, nama, satuan, qty_pesan, harga_satuan, subtotal}, total, dibuat_oleh, created_at, warnings[]`

### Failure

400: invalid input

401: tidak login

403: bukan akuntan

404: pemasok\_id, bahan\_id, menu\_id, atau siklus\_id tidak ditemukan

422: pemasok nonaktif (PEMASOK\_NONAKTIF), bahan nonaktif, bahan duplikat, atau menu belum disetujui

500: unexpected internal error

### Acceptance Criteria

- [ ] Request is validated.
- [ ] PO number is unique and sequential per SPPG per month under concurrent requests.
- [ ] PO, items, and menu links are saved atomically.
- [ ] total equals the sum of item subtotals.
- [ ] Created PO has status draf.
- [ ] Warnings are returned without blocking creation.
- [ ] Data is persisted to PostgreSQL and written to audit\_log.
- [ ] Created entity is returned.
- [ ] Unit tests exist.
- [ ] Integration test exists.

---

## MVP-003.4 List & detail purchase order

### Goal

Akuntan dan kepala SPPG memantau status semua PO. Petugas dapur tahu bahan apa saja yang akan datang hari ini.

### Actor

admin, kepala\_sppg, akuntan, ahli\_gizi, petugas\_dapur, pengawas

### Input

`GET /api/v1/purchase-orders`

- status (opsional, bisa lebih dari satu: `status=dikirim,diterima_sebagian`)
- pemasok\_id (opsional)
- tgl\_kirim\_dari, tgl\_kirim\_sampai (opsional; maks 31 hari)
- q (opsional, cari nomor\_po)
- sppg\_id (hanya admin dan pengawas)
- page, per\_page (default 1 dan 20, maks 100)

`GET /api/v1/purchase-orders/{id}`

### Rules

- default urutan `tgl_kirim_diminta` naik, lalu `nomor_po`
- peran selain admin/pengawas otomatis difilter ke SPPG miliknya
- petugas\_dapur hanya melihat PO berstatus `dikirim`, `diterima_sebagian`, atau `selesai`, tanpa field harga (lihat Matriks peran)
- `terlambat = true` bila status `dikirim`/`diterima_sebagian` dan `tgl_kirim_diminta` < hari ini
- detail menyertakan rekap per item dari `v_po_item_rekap` dan daftar lot penerimaan

### Success

HTTP 200

List: `data[]` (id, nomor\_po, pemasok {id, nama}, tgl\_po, tgl\_kirim\_diminta, status, total, terlambat, jumlah\_item, jumlah\_item\_terpenuhi) dan `meta` (page, per\_page, total)

Detail: PO lengkap dengan:

- `items[]` {id, bahan {id, nama, satuan, mudah\_rusak}, qty\_pesan, harga\_satuan, subtotal, qty\_diterima, qty\_ditolak, qty\_kurang, terpenuhi, nilai\_potong\_nota, nilai\_tagihan}
- `rekap_nilai` {total\_pesanan, total\_nilai\_dikirim, total\_potong\_nota, total\_tagihan}
- `penerimaan[]` {id, kode\_lot, po\_item\_id, waktu\_terima, qty\_diterima, qty\_ditolak, kondisi, diperiksa\_oleh}
- `menu[]` {id, tanggal, nama\_menu}
- `dibuat_oleh`, `dikirim_oleh`, `dikirim_at`, `ditutup_oleh`, `ditutup_at`, `alasan_tutup`

### Failure

400: invalid input

401: tidak login

403: peran tidak berhak

404: PO not found, milik SPPG lain, atau (untuk petugas\_dapur) masih draf/batal

500: unexpected internal error

### Acceptance Criteria

- [ ] Filters and pagination work.
- [ ] Users never see POs from another SPPG.
- [ ] petugas\_dapur never receives price fields or draft/cancelled POs.
- [ ] Detail totals match v\_po\_item\_rekap (tagihan = accepted qty capped at ordered qty × price).
- [ ] Unit tests exist.
- [ ] Integration test exists.

---

## MVP-003.5 Update & delete PO draf

### Goal

Akuntan memperbaiki PO sebelum dikirim ke pemasok, atau menghapus PO draf yang batal disusun.

### Actor

akuntan

### Input

`PATCH /api/v1/purchase-orders/{id}` (semua field opsional)

- pemasok\_id
- tgl\_kirim\_diminta
- siklus\_id
- menu\_ids\[\] (bila dikirim, menggantikan seluruh tautan menu)
- catatan
- items\[\] (bila dikirim, menggantikan seluruh item)

`DELETE /api/v1/purchase-orders/{id}`

### Rules

- hanya PO berstatus `draf` yang bisa diubah atau dihapus
- nomor\_po dan tgl\_po tidak bisa diubah
- aturan validasi dan warnings sama dengan MVP-003.3
- penggantian items\[\] dan menu\_ids\[\] dilakukan dalam satu transaksi; total dihitung ulang oleh trigger
- DELETE menghapus PO beserta `po_item` dan `po_menu` (cascade); nilai lama disimpan di audit\_log. Nomor PO yang terhapus tidak dipakai ulang.

### Success

PATCH: HTTP 200, returns updated PO (+ `warnings[]`)

DELETE: HTTP 204

### Failure

400: invalid input

401: tidak login

403: bukan akuntan

404: PO not found atau milik SPPG lain

409: PO bukan draf (STATUS\_TIDAK\_SESUAI)

422: sama dengan MVP-003.3

500: unexpected internal error

### Acceptance Criteria

- [ ] Only draft POs can be changed or deleted.
- [ ] Sending items\[\] replaces all items atomically and total is recalculated.
- [ ] PO number and date cannot be changed.
- [ ] Old and new values are stored in audit\_log.
- [ ] Unit tests exist.
- [ ] Integration test exists.

---

## MVP-003.6 Kirim, batal & tutup PO

### Goal

Akuntan menandai PO sudah dikirim ke pemasok sehingga petugas dapur siap menerima. PO bisa dibatalkan sebelum ada kiriman, atau ditutup bila sisa pesanan tidak akan datang.

### Actor

kirim dan tutup: akuntan; batal: akuntan, kepala\_sppg

### Input

`POST /api/v1/purchase-orders/{id}/kirim`

`POST /api/v1/purchase-orders/{id}/batal`

- alasan (wajib, 5–500 karakter)

`POST /api/v1/purchase-orders/{id}/tutup`

- alasan (wajib, 5–500 karakter)

### Rules

- **kirim** hanya untuk PO `draf` yang punya minimal 1 item, pemasoknya masih aktif, dan `tgl_kirim_diminta` ≥ hari ini. Kirim mengisi `dikirim_oleh` dan `dikirim_at`, lalu status menjadi `dikirim`.
- setelah kirim, petugas\_dapur dan kepala\_sppg di SPPG tersebut menerima notifikasi `po_dikirim` (nomor PO, pemasok, tanggal kirim, daftar bahan)
- pengiriman dokumen PO ke pemasok (PDF/WhatsApp) belum termasuk MVP; akuntan menyampaikannya di luar sistem
- **batal** hanya untuk PO `draf` atau `dikirim` yang belum punya penerimaan sama sekali. Batal mengisi `ditutup_oleh`, `ditutup_at`, `alasan_tutup`, lalu status menjadi `batal`. PO batal tidak dihitung di MVP-003.2.
- **tutup** hanya untuk PO `diterima_sebagian`. Tutup mengisi kolom yang sama, status menjadi `selesai`, dan item yang belum terpenuhi dilaporkan di response sebagai `item_tidak_terpenuhi[]`.
- bila PO yang dibatalkan sudah berstatus `dikirim`, petugas\_dapur menerima notifikasi `po_batal`
- semua aksi dicatat di audit\_log (aksi `UPDATE`, dengan status lama dan baru)

### Success

HTTP 200

Returns PO dengan status terbaru (+ `item_tidak_terpenuhi[]` untuk tutup)

### Failure

400: alasan kosong atau terlalu pendek

401: tidak login

403: peran tidak berhak

404: PO not found atau milik SPPG lain

409: status tidak sesuai (STATUS\_TIDAK\_SESUAI) atau sudah ada penerimaan saat batal (PO\_SUDAH\_ADA\_PENERIMAAN)

422: kirim PO tanpa item, pemasok nonaktif (PEMASOK\_NONAKTIF), atau tgl\_kirim\_diminta sudah lewat

500: unexpected internal error

### Acceptance Criteria

- [ ] Only complete draft POs can be sent; sender and timestamp are stored.
- [ ] PO with any receipt cannot be cancelled.
- [ ] Cancel and close require a reason, stored on the PO and in audit\_log.
- [ ] Close lists the unfulfilled items.
- [ ] Notifications are created for petugas\_dapur and kepala\_sppg on send.
- [ ] Unit tests exist.
- [ ] Integration test exists.

---

## MVP-003.7 Upload foto bukti

### Goal

Petugas mengunggah foto bukti langsung dari kamera HP dengan timestamp server yang tidak bisa diubah. Endpoint ini dipakai penerimaan bahan sekarang, lalu checklist SOP (M4), distribusi (M6), dan portal sekolah (M7).

### Actor

Semua peran kecuali pengawas, dibatasi per konteks:

| Konteks | Peran yang boleh | Modul |
| --- | --- | --- |
| penerimaan\_bahan | akuntan, petugas\_dapur | M3 |
| checklist\_sop | petugas\_dapur, ahli\_gizi, kepala\_sppg | M4 (ditetapkan di MVP-004) |
| pengiriman | petugas\_distribusi | M6 (ditetapkan di MVP-006) |
| konfirmasi\_terima, pengaduan | pic\_sekolah | M7 (ditetapkan di MVP-007) |

### Input

`POST /api/v1/uploads` (multipart/form-data)

- file
- konteks

### Rules

- tipe file diperiksa dari magic bytes, bukan ekstensi: JPEG, PNG, atau WebP; maks 5 MB
- server menghapus metadata EXIF (termasuk GPS dan waktu kamera) lalu menyimpan foto ke object storage dengan key `{kode_sppg}/{konteks}/{YYYY}/{MM}/{DD}/{uuid}.{ext}`
- `sha256` dihitung dari berkas yang disimpan; `diunggah_at` = waktu server
- berkas belum terikat ke data apa pun sampai dipakai endpoint lain (mis. MVP-003.8). Endpoint pemakai wajib memastikan berkas:
  - milik SPPG yang sama
  - konteksnya cocok
  - belum pernah dipakai
  - diunggah ≤ 24 jam lalu
  
  Saat dipakai, endpoint tersebut mengisi `ref_tabel`, `ref_id`, `dipakai_at` dalam transaksi yang sama.
- berkas yang sudah dipakai tidak bisa diganti atau dihapus lewat API
- rate limit 30 unggahan per menit per user

### Success

HTTP 201

Returns `berkas_id, konteks, mime_type, ukuran_byte, sha256, diunggah_at, url` (presigned 15 menit, untuk pratinjau)

### Failure

400: file kosong atau konteks tidak dikenal

401: tidak login

403: peran tidak berhak untuk konteks tersebut

413: file lebih dari 5 MB

415: tipe file tidak didukung

429: terlalu banyak unggahan

500: unexpected internal error (termasuk object storage tidak tersedia)

### Acceptance Criteria

- [ ] File type is checked by content, not by extension.
- [ ] EXIF metadata is removed before storage.
- [ ] Server timestamp and SHA-256 are stored and returned.
- [ ] Role-to-context rules are enforced.
- [ ] Unused files older than 24 hours are removed by the scheduled job.
- [ ] Unit tests exist.
- [ ] Integration test exists (with MinIO or an S3 mock).

---

## MVP-003.8 Catat penerimaan bahan

### Goal

Saat pemasok datang, petugas memeriksa setiap bahan (jumlah, kondisi, suhu, kedaluwarsa, foto), menerima yang layak, dan menolak yang tidak. Bahan yang ditolak otomatis memotong nilai nota. Setiap bahan yang diterima menjadi satu lot yang bisa ditelusuri sampai batch (M5).

### Actor

akuntan, petugas\_dapur

### Input

`POST /api/v1/purchase-orders/{id}/penerimaan`

Header `Idempotency-Key: <UUID>` (wajib)

- items\[\]: { po\_item\_id, qty\_diterima, qty\_ditolak, alasan\_tolak, kondisi, suhu\_terima, tgl\_kedaluwarsa, foto\_berkas\_id, catatan }

### Rules

- PO harus berstatus `dikirim` atau `diterima_sebagian` dan milik SPPG pengguna
- items\[\] berisi 1–50 item; setiap po\_item\_id harus milik PO ini dan tidak duplikat dalam satu request
- qty\_diterima ≥ 0, qty\_ditolak ≥ 0, jumlah keduanya > 0; bilangan bulat untuk butir dan pcs
- qty\_ditolak > 0 → alasan\_tolak wajib (3–500 karakter)
- kondisi wajib (`baik`, `kurang`, `rusak`); `rusak` → qty\_diterima harus 0; `kurang` → catatan wajib
- bahan `mudah_rusak` → suhu\_terima wajib (−30 s.d. 60 °C)
  - `lolos_suhu` dihitung server: `suhu_terima ≤ bahan.suhu_simpan_maks`
  - bila tidak lolos dan qty\_diterima > 0 → ditolak dengan **SUHU\_MELEBIHI\_BATAS**; petugas harus mencatatnya sebagai ditolak
  - bahan yang tidak mudah rusak boleh tanpa suhu, `lolos_suhu = null`
- tgl\_kedaluwarsa opsional. Bila < hari ini dan qty\_diterima > 0 → ditolak dengan **BAHAN\_KEDALUWARSA**. Bila ≤ hari ini + 1, response menyertakan warning.
- total qty\_diterima item (semua lot sebelumnya + lot ini) maks `qty_pesan × 1,10`; lebih dari itu → **MELEBIHI\_PESANAN**
- foto\_berkas\_id wajib per item, harus lolos aturan pemakaian berkas di MVP-003.7 dengan konteks `penerimaan_bahan`
- server mengisi:
  - `kode_lot`
  - `waktu_terima` = waktu server
  - `diperiksa_oleh` = user dari token
  - `sppg_id`
  - `client_ref` = Idempotency-Key
  - `nilai_potong_nota` = round(qty\_ditolak × harga\_satuan, 2)
- semua lot, pemakaian berkas, dan perubahan status PO disimpan dalam satu transaksi. Bila satu item gagal, tidak ada yang tersimpan.
- status PO dihitung ulang dari `v_po_item_rekap`: semua item terpenuhi → `selesai`, selain itu → `diterima_sebagian`
- request ulang dengan Idempotency-Key yang sama dan body identik mengembalikan hasil pertama (HTTP 200) tanpa membuat lot baru. Key sama dengan body berbeda → 409. Ini menjaga data tidak ganda saat sinyal putus lalu PWA mengirim ulang.
- notifikasi:
  - ada qty\_ditolak > 0 → `penerimaan_ditolak` ke akuntan dan kepala\_sppg
  - ada `lolos_suhu = false` → `suhu_tidak_sesuai` ke kepala\_sppg
- catatan penerimaan ini adalah bukti titik kritis tahap `penerimaan`; MVP-004 akan merujuknya, bukan meminta isian ulang

### Success

HTTP 201 (atau 200 untuk request ulang idempoten)

Returns:

- `po` {id, nomor\_po, status, rekap\_nilai {total\_pesanan, total\_nilai\_dikirim, total\_potong\_nota, total\_tagihan}}
- `lots[]` {id, kode\_lot, po\_item\_id, bahan {id, nama, satuan}, qty\_diterima, qty\_ditolak, alasan\_tolak, kondisi, suhu\_terima, lolos\_suhu, tgl\_kedaluwarsa, nilai\_potong\_nota, foto\_url, waktu\_terima, diperiksa\_oleh}
- `warnings[]`

### Failure

400: invalid input atau header Idempotency-Key tidak ada/bukan UUID

401: tidak login

403: peran tidak berhak

404: PO not found, milik SPPG lain, atau po\_item\_id bukan milik PO ini

409: status PO tidak sesuai (STATUS\_TIDAK\_SESUAI), atau Idempotency-Key dipakai dengan body berbeda

422: SUHU\_MELEBIHI\_BATAS, BAHAN\_KEDALUWARSA, MELEBIHI\_PESANAN, BERKAS\_TIDAK\_VALID, atau barang rusak dengan qty diterima > 0 (sebutkan po\_item\_id di `error.fields`)

500: unexpected internal error

### Acceptance Criteria

- [ ] Request is validated.
- [ ] Perishable item above max storage temperature cannot be accepted.
- [ ] Expired or damaged goods cannot be accepted.
- [ ] Rejected quantity reduces the invoice: nilai\_potong\_nota = qty\_ditolak × harga\_satuan.
- [ ] Each accepted item gets a unique lot code.
- [ ] PO status moves to diterima\_sebagian or selesai automatically.
- [ ] Same Idempotency-Key does not create duplicate lots.
- [ ] All items, file usage, and status change are saved atomically.
- [ ] Notifications are created for rejections and temperature failures.
- [ ] Data is persisted to PostgreSQL and written to audit\_log.
- [ ] Unit tests cover temperature, expiry, tolerance, and potong nota cases.
- [ ] Integration test exists.

---

## MVP-003.9 Daftar & detail lot penerimaan

### Goal

Petugas dapur memilih lot bahan yang masih tersedia untuk dimasak (dasar `batch_bahan` di M5). Kepala SPPG dan pengawas menelusuri lot sampai ke pemasok dan batch yang memakainya.

### Actor

admin, kepala\_sppg, akuntan, ahli\_gizi, petugas\_dapur, pengawas

### Input

`GET /api/v1/penerimaan`

- dari, sampai (opsional, tanggal terima; maks 31 hari; default 7 hari terakhir)
- bahan\_id, pemasok\_id, po\_id (opsional)
- kondisi (opsional)
- kode\_lot (opsional, cocok persis; untuk scan label wadah)
- tersedia (opsional, boolean)
- sppg\_id (hanya admin dan pengawas)
- page, per\_page (default 1 dan 20, maks 100)

`GET /api/v1/penerimaan/{id}`

### Rules

- `qty_terpakai` = Σ `batch_bahan.qty_dipakai` untuk lot tersebut (0 sampai M5 berjalan)
- `qty_sisa` = qty\_diterima − qty\_terpakai
- `tersedia = true` hanya mengembalikan lot dengan qty\_sisa > 0, kondisi ≠ `rusak`, dan `tgl_kedaluwarsa` kosong atau ≥ hari ini
- urutan default: `tgl_kedaluwarsa` naik (kosong di akhir), lalu `waktu_terima` naik, agar lot yang lebih dulu kedaluwarsa dipakai lebih dulu (FEFO)
- peran selain admin/pengawas otomatis difilter ke SPPG miliknya; petugas\_dapur tidak menerima field harga atau nilai potong nota
- detail menyertakan rantai mundur (PO, pemasok) dan rantai maju (`batch[]` yang memakai lot ini, kosong sebelum M5)

### Success

HTTP 200

List: `data[]` (id, kode\_lot, bahan {id, nama, satuan, mudah\_rusak}, pemasok {id, nama}, nomor\_po, waktu\_terima, qty\_diterima, qty\_ditolak, qty\_terpakai, qty\_sisa, kondisi, suhu\_terima, lolos\_suhu, tgl\_kedaluwarsa) dan `meta` (page, per\_page, total)

Detail: lot lengkap + `foto_url`, `alasan_tolak`, `catatan`, `nilai_potong_nota`, `diperiksa_oleh`, `dikoreksi_oleh`, `dikoreksi_at`, `po {id, nomor_po, status}`, `pemasok {id, nama, no_hp}`, `batch[] {id, kode_batch, qty_dipakai}`

### Failure

400: invalid input (rentang terbalik atau lebih dari 31 hari)

401: tidak login

403: peran tidak berhak

404: lot not found atau milik SPPG lain

500: unexpected internal error

### Acceptance Criteria

- [ ] Remaining quantity equals accepted minus used in batches.
- [ ] tersedia filter excludes empty, damaged, and expired lots.
- [ ] Default order is first-expiry-first-out.
- [ ] Lookup by exact lot code works.
- [ ] Users never see lots from another SPPG; petugas\_dapur never sees prices.
- [ ] Unit tests exist.
- [ ] Integration test exists.

---

## MVP-003.10 Koreksi penerimaan

### Goal

Kepala SPPG memperbaiki salah catat pada penerimaan (mis. salah ketik qty) tanpa menghapus bukti asli, dengan alasan dan jejak audit lengkap.

### Actor

kepala\_sppg

### Input

`POST /api/v1/penerimaan/{id}/koreksi`

- alasan (wajib, 10–500 karakter)
- qty\_diterima, qty\_ditolak, alasan\_tolak, kondisi, tgl\_kedaluwarsa, catatan (opsional; minimal satu field dikirim)

### Rules

- `suhu_terima`, `lolos_suhu`, `foto_url`, `waktu_terima`, `diperiksa_oleh`, dan `kode_lot` tidak bisa dikoreksi karena merupakan bukti
- lot yang sudah dipakai di `batch_bahan` tidak bisa dikoreksi (**LOT\_SUDAH\_DIPAKAI**)
- kepala\_sppg tidak bisa mengoreksi lot yang ia periksa sendiri (`diperiksa_oleh` ≠ user saat ini)
- hasil koreksi wajib lolos semua aturan MVP-003.8:
  - lot yang `lolos_suhu = false` tetap tidak bisa punya qty\_diterima > 0
  - toleransi 10% tetap berlaku
  - barang rusak tetap tidak boleh diterima
- server menghitung ulang `nilai_potong_nota` dan status PO. PO `selesai` hasil otomatis bisa kembali ke `diterima_sebagian`; PO yang ditutup manual tetap `selesai`.
- pembaruan penerimaan, penghitungan status PO, dan pembacaan hasil respons dilakukan dalam satu transaksi. Hasil dibaca sebelum commit, termasuk ketika status PO tetap sama; kegagalan pembacaan hasil membatalkan perubahan.
- server mengisi `dikoreksi_oleh` dan `dikoreksi_at`; nilai lama, nilai baru, dan alasan dicatat di audit\_log
- akuntan menerima notifikasi `penerimaan_dikoreksi` karena nilai tagihan bisa berubah
- mulai M8: koreksi ditolak bila PO sudah masuk laporan siklus dana berstatus `dilaporkan` atau `ditutup`

### Success

HTTP 200

Returns lot terbaru dan `po {id, nomor_po, status, rekap_nilai}`

### Failure

400: invalid input atau alasan kosong

401: tidak login

403: bukan kepala\_sppg, atau mengoreksi pemeriksaan sendiri

404: lot not found atau milik SPPG lain

409: LOT\_SUDAH\_DIPAKAI

422: hasil koreksi melanggar aturan MVP-003.8 (SUHU\_MELEBIHI\_BATAS, MELEBIHI\_PESANAN, barang rusak diterima)

500: unexpected internal error

### Acceptance Criteria

- [ ] Evidence fields (temperature, photo, time, inspector, lot code) cannot be changed.
- [ ] Lot used by a production batch cannot be corrected.
- [ ] Inspector cannot correct their own receipt.
- [ ] potong nota and PO status are recalculated.
- [ ] Reason, old values, and new values are stored in audit\_log.
- [ ] Unit tests exist.
- [ ] Integration test exists.

---

## Pertanyaan terbuka

- [ ] Apakah PO perlu persetujuan kepala SPPG sebelum dikirim (mis. di atas nilai tertentu)? Spesifikasi ini mengizinkan akuntan langsung mengirim; maker–checker baru diterapkan di pencairan dana (M8).
- [ ] Toleransi kelebihan kirim 10%: sesuai praktik lapangan, atau perlu berbeda per kategori (mis. sayur berbasis berat vs telur per butir)?
- [ ] Apakah dokumen PO perlu diekspor ke PDF atau dikirim otomatis ke WhatsApp pemasok pada R1, atau cukup di R2?
