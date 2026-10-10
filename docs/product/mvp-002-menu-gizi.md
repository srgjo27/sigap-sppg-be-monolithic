# MVP-002 Menu & gizi
 
Spesifikasi modul M2 (P0): master bahan, penyusunan menu harian beserta komposisi dan nilai gizi per porsi, persetujuan menu, dan perhitungan kebutuhan bahan otomatis. Dipecah menjadi 7 endpoint yang masing-masing bisa diuji terpisah. Semua endpoint berprefiks `/api/v1`, dilindungi middleware RBAC dari MVP-001.8, dan memakai tabel `bahan`, `menu`, `menu_bahan`, `sekolah`, `audit_log`.
 
## Main Feature
 
Menu harian/mingguan, komposisi gizi per porsi, kebutuhan bahan otomatis dari jumlah penerima
 
## Ringkasan endpoint
 
| ID | Endpoint | Fungsi | Akses |
| --- | --- | --- | --- |
| MVP-002.1 | GET/POST/PATCH /bahan | Kelola master bahan | baca: semua peran SPPG; tulis: admin, ahli\_gizi |
| MVP-002.2 | POST /menus | Buat menu harian (draf) beserta komposisi | ahli\_gizi |
| MVP-002.3 | GET /menus, GET /menus/{id} | Daftar menu per rentang tanggal dan detailnya | semua peran SPPG, pengawas |
| MVP-002.4 | PATCH /menus/{id}, DELETE /menus/{id} | Ubah atau hapus menu draf | ahli\_gizi |
| MVP-002.5 | POST /menus/{id}/approve, POST /menus/{id}/revert | Setujui menu atau kembalikan ke draf | kepala\_sppg |
| MVP-002.6 | GET /menus/{id}/kebutuhan-bahan | Hitung total kebutuhan bahan | ahli\_gizi, akuntan, kepala\_sppg |
| MVP-002.7 | POST /menus/{id}/copy | Salin menu ke tanggal lain (susun menu mingguan) | ahli\_gizi |
 
## Matriks peran
 
| Peran | Master bahan | Buat/ubah menu | Setujui menu | Lihat menu & kebutuhan bahan |
| --- | --- | --- | --- | --- |
| admin | Tulis | Tidak | Tidak | Semua SPPG |
| ahli\_gizi | Tulis | Ya, SPPG sendiri | Tidak | SPPG sendiri |
| kepala\_sppg | Baca | Tidak | Ya, SPPG sendiri | SPPG sendiri |
| akuntan | Baca | Tidak | Tidak | SPPG sendiri (untuk menyusun PO) |
| petugas\_dapur, petugas\_distribusi | Baca | Tidak | Tidak | Menu saja, SPPG sendiri |
| pic\_sekolah | Tidak | Tidak | Tidak | Menu yang sudah disetujui untuk SPPG yang melayani sekolahnya |
| pengawas | Baca | Tidak | Tidak | Semua SPPG, baca saja |

## Tambahan skema
 
Perhitungan kebutuhan bahan butuh konversi gram ke satuan bahan (mis. telur dalam butir), dan persetujuan menu perlu dicatat pelakunya.
Read `/db/schema.sql` before work.

## Aturan umum
 
- Satu menu per SPPG per tanggal (UNIQUE `sppg_id, tanggal`).
- Menu berstatus `disetujui` tidak bisa diubah atau dihapus; harus dikembalikan ke draf dulu (MVP-002.5).
- Menu yang sudah dipakai di `batch_produksi` tidak bisa dikembalikan ke draf, diubah, atau dihapus.
- `target_porsi` default = jumlah `sekolah.jumlah_penerima` milik SPPG tersebut; ahli gizi boleh mengubahnya.
- Nilai gizi (`energi_kkal`, `protein_g`, `karbohidrat_g`, `lemak_g`) diisi manual oleh ahli gizi per porsi pada MVP; perhitungan otomatis dari tabel komposisi pangan masuk rilis berikutnya.
- Semua perubahan menu dan bahan dicatat di `audit_log`; format error mengikuti MVP-001.
- Angka desimal dikirim sebagai string atau number dengan maksimal 2 angka di belakang koma.

 
## MVP-002.1 Master bahan
 
### Goal
 
Ahli gizi dan admin mengelola daftar bahan pangan yang dipakai di menu dan pengadaan, lengkap dengan satuan dan sifat mudah rusak.
 
### Actor
 
baca: semua peran kecuali pic\_sekolah; tulis: admin, ahli\_gizi
 
### Input
 
`POST /api/v1/bahan` dan `PATCH /api/v1/bahan/{id}`
 
- nama
- kategori (karbohidrat, protein\_hewani, protein\_nabati, sayur, buah, bumbu, lainnya)
- satuan (kg, liter, butir, ikat, pcs)
- gram\_per\_satuan
- mudah\_rusak (boolean)
- suhu\_simpan\_maks (opsional, °C)
- aktif (hanya PATCH)
`GET /api/v1/bahan?q=&kategori=&aktif=` untuk pencarian.
 
### Rules
 
- nama must be unique (case-insensitive) dan wajib, 2–100 karakter
- kategori dan satuan wajib, dari daftar yang diizinkan
- gram\_per\_satuan > 0; untuk satuan kg dan liter otomatis 1000 bila tidak diisi
- suhu\_simpan\_maks wajib bila mudah\_rusak = true
- bahan tidak bisa dihapus, hanya dinonaktifkan; bahan nonaktif tidak muncul saat menyusun menu baru, tetapi menu lama tetap utuh
### Success
 
POST: HTTP 201, returns created bahan
 
PATCH: HTTP 200, returns updated bahan
 
GET: HTTP 200, returns `data[]` dan `meta`
 
### Failure
 
400: invalid input
 
401: tidak login
 
403: peran tidak berhak menulis
 
404: bahan not found
 
409: nama bahan already exists
 
500: unexpected internal error
 
### Acceptance Criteria
 
- [ ] Request is validated.
- [ ] Duplicate name (any letter case) is rejected with 409.
- [ ] Perishable item without max storage temperature is rejected.
- [ ] Deactivated item is hidden from new menu input but kept in old menus.
- [ ] Data is persisted to PostgreSQL and written to audit\_log.
- [ ] Unit tests exist.
- [ ] Integration test exists.

## MVP-002.2 Create menu
 
### Goal
 
Ahli gizi menyusun menu untuk satu tanggal, lengkap dengan komposisi bahan per porsi dan nilai gizinya.
 
### Actor
 
ahli\_gizi
 
### Input
 
`POST /api/v1/menus`
 
- tanggal
- nama\_menu
- energi\_kkal, protein\_g, karbohidrat\_g, lemak\_g (per porsi)
- target\_porsi (opsional; default dari jumlah penerima)
- catatan (opsional)
- bahan\[\]: { bahan\_id, gram\_per\_porsi }
### Rules
 
- (sppg\_id, tanggal) must be unique; sppg\_id diambil dari token, bukan dari body
- tanggal tidak boleh di masa lalu
- nama\_menu is required, 3–200 karakter
- nilai gizi wajib dan > 0; energi\_kkal maks 2000, protein/karbohidrat/lemak maks 300 g (cegah salah ketik)
- bahan\[\] minimal 1 item, setiap bahan\_id harus ada dan aktif, tidak boleh duplikat
- gram\_per\_porsi > 0 dan maks 1000
- target\_porsi > 0; bila melebihi `sppg.kapasitas_porsi` tetap diterima tetapi response menyertakan `warnings[]`
- menu baru selalu berstatus `draf`; `dibuat_oleh` = user dari token
- menu dan semua `menu_bahan` disimpan dalam satu transaksi
### Success
 
HTTP 201
 
Returns created menu: `id, tanggal, nama_menu, gizi {energi_kkal, protein_g, karbohidrat_g, lemak_g}, target_porsi, status, bahan[] {bahan_id, nama, gram_per_porsi}, dibuat_oleh, created_at, warnings[]`
 
### Failure
 
400: invalid input
 
401: tidak login
 
403: bukan ahli\_gizi
 
404: bahan\_id tidak ditemukan
 
409: menu untuk tanggal tersebut already exists
 
422: bahan nonaktif atau duplikat dalam komposisi
 
500: unexpected internal error
 
### Acceptance Criteria
 
- [ ] Request is validated.
- [ ] Second menu on the same date for the same SPPG is rejected with 409.
- [ ] Menu and its ingredients are saved atomically (no partial data on failure).
- [ ] target\_porsi defaults to total recipients of the SPPG's schools.
- [ ] Created menu has status draf.
- [ ] Data is persisted to PostgreSQL and written to audit\_log.
- [ ] Created entity is returned.
- [ ] Unit tests exist.
- [ ] Integration test exists.

## MVP-002.3 List & detail menu
 
### Goal
 
Semua pihak di SPPG melihat menu harian dan mingguan; pengawas melihat lintas SPPG.
 
### Actor
 
Semua peran SPPG, pengawas, admin; pic\_sekolah hanya menu disetujui
 
### Input
 
`GET /api/v1/menus`
 
- dari, sampai (wajib; maks 31 hari)
- status (opsional)
- sppg\_id (hanya admin dan pengawas)
`GET /api/v1/menus/{id}`
 
### Rules
 
- default urutan tanggal naik
- peran selain admin/pengawas otomatis difilter ke sppg\_id miliknya
- pic\_sekolah hanya melihat menu `disetujui` dari SPPG yang melayani sekolahnya, tanpa komposisi gram (cukup nama menu dan nilai gizi)
- tanggal tanpa menu dikembalikan di `meta.tanggal_kosong[]` agar frontend bisa menandai hari yang belum direncanakan
### Success
 
HTTP 200
 
List: `data[]` (id, tanggal, nama\_menu, gizi, target\_porsi, status) dan `meta` (dari, sampai, total, tanggal\_kosong\[\])
 
Detail: menu lengkap dengan `bahan[]`, `dibuat_oleh`, `disetujui_oleh`, `disetujui_at`
 
### Failure
 
400: invalid input (rentang terbalik atau lebih dari 31 hari)
 
401: tidak login
 
404: menu not found atau milik SPPG lain
 
500: unexpected internal error
 
### Acceptance Criteria
 
- [ ] Date range filter works and is limited to 31 days.
- [ ] Users never see menus from another SPPG.
- [ ] pic\_sekolah sees approved menus only, without ingredient grams.
- [ ] Days without a menu are listed in meta.
- [ ] Unit tests exist.
- [ ] Integration test exists.

## MVP-002.4 Update & delete menu draf
 
### Goal
 
Ahli gizi memperbaiki menu sebelum disetujui, atau menghapus menu yang batal.
 
### Actor
 
ahli\_gizi
 
### Input
 
`PATCH /api/v1/menus/{id}` (semua field opsional)
 
- nama\_menu
- energi\_kkal, protein\_g, karbohidrat\_g, lemak\_g
- target\_porsi
- catatan
- bahan\[\] (bila dikirim, menggantikan seluruh komposisi)
`DELETE /api/v1/menus/{id}`
 
### Rules
 
- hanya menu berstatus `draf` yang bisa diubah atau dihapus
- tanggal tidak bisa diubah; untuk pindah tanggal gunakan copy (MVP-002.7) lalu hapus
- aturan validasi sama dengan MVP-002.2
- penggantian bahan\[\] dilakukan dalam satu transaksi
- DELETE menghapus menu beserta `menu_bahan` (cascade); nilai lama disimpan di audit\_log
### Success
 
PATCH: HTTP 200, returns updated menu
 
DELETE: HTTP 204
 
### Failure
 
400: invalid input
 
401: tidak login
 
403: bukan ahli\_gizi
 
404: menu not found atau milik SPPG lain
 
409: menu sudah disetujui atau sudah dipakai di batch produksi
 
500: unexpected internal error
 
### Acceptance Criteria
 
- [ ] Only draft menus can be changed or deleted.
- [ ] Sending bahan\[\] replaces the whole composition atomically.
- [ ] Date cannot be changed.
- [ ] Old and new values are stored in audit\_log.
- [ ] Unit tests exist.
- [ ] Integration test exists.

## MVP-002.5 Approve & revert menu
 
### Goal
 
Kepala SPPG menyetujui menu sebelum bahan dipesan dan dimasak, atau mengembalikannya ke draf untuk diperbaiki.
 
### Actor
 
kepala\_sppg
 
### Input
 
`POST /api/v1/menus/{id}/approve`
 
`POST /api/v1/menus/{id}/revert`
 
- alasan (wajib untuk revert)
### Rules
 
- approve hanya untuk menu `draf` yang punya minimal 1 bahan dan nilai gizi lengkap
- approve mengisi `disetujui_oleh`, `disetujui_at`, dan mengubah status ke `disetujui`
- revert hanya untuk menu `disetujui` yang belum dipakai di `batch_produksi`
- revert mengosongkan `disetujui_oleh` dan `disetujui_at`, status kembali `draf`, alasan dicatat di audit\_log
- setelah approve, akuntan dan ahli gizi menerima notifikasi bahwa kebutuhan bahan siap dipesan
### Success
 
HTTP 200
 
Returns menu dengan status terbaru
 
### Failure
 
400: alasan revert kosong
 
401: tidak login
 
403: bukan kepala\_sppg
 
404: menu not found atau milik SPPG lain
 
409: status tidak sesuai (approve menu yang sudah disetujui, revert menu draf) atau menu sudah dipakai batch
 
422: menu belum lengkap (tanpa bahan atau nilai gizi)
 
500: unexpected internal error
 
### Acceptance Criteria
 
- [ ] Only complete draft menus can be approved.
- [ ] Approved menu stores approver and timestamp.
- [ ] Menu used by a production batch cannot be reverted.
- [ ] Revert requires a reason, written to audit\_log.
- [ ] Notification is created for akuntan and ahli\_gizi on approval.
- [ ] Unit tests exist.
- [ ] Integration test exists.
 
## MVP-002.6 Kebutuhan bahan
 
### Goal
 
Akuntan dan ahli gizi langsung tahu berapa banyak setiap bahan yang harus dipesan untuk satu menu, sebagai dasar purchase order di M3.
 
### Actor
 
ahli\_gizi, akuntan, kepala\_sppg
 
### Input
 
`GET /api/v1/menus/{id}/kebutuhan-bahan`
 
- target\_porsi (opsional, untuk simulasi tanpa mengubah menu)
- cadangan\_persen (opsional, default 0, maks 20; tambahan untuk susut/rusak)
### Rules
 
- rumus per bahan: `total_gram = gram_per_porsi × target_porsi × (1 + cadangan_persen/100)`
- dikonversi ke satuan bahan: `qty = total_gram ÷ bahan.gram_per_satuan`, dibulatkan ke atas 2 desimal (butir dan pcs dibulatkan ke atas ke bilangan bulat)
- endpoint hanya membaca, tidak menyimpan apa pun
- bisa dipanggil untuk menu draf (simulasi) maupun disetujui; response menyertakan status menu
### Success
 
HTTP 200
 
Returns `menu {id, tanggal, nama_menu, status}`, `target_porsi`, `cadangan_persen`, dan `items[]` {bahan\_id, nama, kategori, satuan, gram\_per\_porsi, total\_gram, qty, mudah\_rusak}
 
### Failure
 
400: invalid input (target\_porsi ≤ 0, cadangan di luar 0–20)
 
401: tidak login
 
403: peran tidak berhak
 
404: menu not found atau milik SPPG lain
 
500: unexpected internal error
 
### Acceptance Criteria
 
- [ ] Quantities match the formula, including unit conversion and rounding.
- [ ] Countable units (butir, pcs) are rounded up to whole numbers.
- [ ] Simulation parameters do not change stored data.
- [ ] Unit tests cover conversion and rounding cases.
- [ ] Integration test exists.

## MVP-002.7 Copy menu
 
### Goal
 
Ahli gizi menyusun menu mingguan dengan cepat dengan menyalin menu yang sudah ada ke satu atau beberapa tanggal lain.
 
### Actor
 
ahli\_gizi
 
### Input
 
`POST /api/v1/menus/{id}/copy`
 
- tanggal\_tujuan\[\] (1–7 tanggal)
### Rules
 
- sumber boleh berstatus draf atau disetujui, dari SPPG yang sama
- setiap tanggal tujuan tidak boleh di masa lalu dan belum punya menu
- salinan selalu berstatus `draf`, dengan `dibuat_oleh` = user saat ini dan target\_porsi dihitung ulang dari jumlah penerima terkini
- bahan yang sudah nonaktif tidak ikut disalin dan dilaporkan di `warnings[]`
- semua salinan dibuat dalam satu transaksi: bila satu tanggal gagal, tidak ada yang dibuat
### Success
 
HTTP 201
 
Returns `data[]` menu baru dan `warnings[]`
 
### Failure
 
400: invalid input (lebih dari 7 tanggal, tanggal duplikat, atau di masa lalu)
 
401: tidak login
 
403: bukan ahli\_gizi
 
404: menu sumber not found atau milik SPPG lain
 
409: salah satu tanggal tujuan sudah punya menu (sebutkan tanggalnya)
 
500: unexpected internal error
 
### Acceptance Criteria
 
- [ ] Copies are created as drafts for every target date in one transaction.
- [ ] Conflict on any date rejects the whole request with 409.
- [ ] Inactive ingredients are skipped and reported.
- [ ] Data is persisted to PostgreSQL and written to audit\_log.
- [ ] Unit tests exist.
- [ ] Integration test exists.