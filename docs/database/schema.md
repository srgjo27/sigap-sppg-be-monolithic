# Database Schema

## Purpose

PostgreSQL database for SIGAP SPPG backend.

## Main Domains

- users
- sppg

  
## Important Relationships

| Relasi | Kardinalitas | Foreign key | Arti bisnis |
|---|---|---|---|
| sppg has many users | 1 : N | users.sppg_id | Setiap pegawai dan relawan terikat pada satu dapur; pengawas boleh null (lintas SPPG) |
| sppg has many sekolah | 1 : N | sekolah.sppg_id | Satu dapur melayani banyak sekolah/posyandu; satu sekolah hanya dilayani satu dapur |
| sppg has many pemasok | 1 : N | pemasok.sppg_id | Daftar pemasok dikelola per dapur |
| sppg has many menu | 1 : N | menu.sppg_id | Satu menu per hari per dapur (UNIQUE sppg_id, tanggal) |
| sppg has many batch_produksi | 1 : N | batch_produksi.sppg_id | Semua produksi tercatat per dapur |
| sppg has many siklus_dana | 1 : N | siklus_dana.sppg_id | Satu proposal dana per periode 10 hari |
| sekolah has many users (pic_sekolah) | 1 : N | users.sekolah_id | Akun PIC yang mengonfirmasi penerimaan dan mengadu |
| menu belongs to many bahan | N : M | menu_bahan (menu_id, bahan_id) | Komposisi gram per porsi; tabel penghubung |
| pemasok has many purchase_order | 1 : N | purchase_order.pemasok_id | Riwayat pesanan per pemasok |
| purchase_order has many po_item | 1 : N | po_item.po_id | Rincian bahan yang dipesan |
| po_item has many penerimaan_bahan | 1 : N | penerimaan_bahan.po_item_id | Satu item bisa datang bertahap; tiap kedatangan = satu lot |
| menu has many batch_produksi | 1 : N | batch_produksi.menu_id | Satu menu bisa dimasak dalam beberapa batch |
| batch_produksi belongs to many penerimaan_bahan | N : M | batch_bahan (batch_id, penerimaan_id) | Lot bahan yang dipakai batch; kunci traceability ke pemasok |
| batch_produksi has many checklist_sop | 1 : N | checklist_sop.batch_id | Satu isian per titik kritis (UNIQUE batch_id, titik_kritis_id) |
| titik_kritis has many checklist_sop | 1 : N | checklist_sop.titik_kritis_id | Template SOP dipakai ulang di setiap batch |
| batch_produksi has one sampel_retensi | 1 : 1 | sampel_retensi.batch_id (UNIQUE) | Sampel untuk uji lab saat insiden |
| batch_produksi belongs to many sekolah | N : M | pengiriman (batch_id, sekolah_id) | Satu batch dikirim ke banyak sekolah; pengiriman sebagai tabel penghubung beratribut |
| pengiriman has one konfirmasi_terima | 1 : 1 | konfirmasi_terima.pengiriman_id (UNIQUE) | Bukti makanan sampai dan kondisinya |
| batch_produksi has many pengaduan | 1 : N (opsional) | pengaduan.batch_id | Aduan menempel ke batch asal agar sumber masalah langsung terlacak |
| sekolah has many pengaduan | 1 : N | pengaduan.sekolah_id | Riwayat aduan per sekolah |
| pengaduan has many tindak_lanjut | 1 : N | tindak_lanjut.pengaduan_id | Jejak penanganan tiket |
| siklus_dana has many transaksi_dana | 1 : N | transaksi_dana.siklus_id | Pengeluaran per siklus untuk laporan 10 harian |
| siklus_dana has many purchase_order | 1 : N | purchase_order.siklus_id | PO dibiayai dari siklus dana tertentu |
| purchase_order has many transaksi_dana | 1 : N | transaksi_dana.po_id | Pembayaran bahan baku wajib merujuk PO |
| users has many audit_log & notifikasi | 1 : N | audit_log.user_id, notifikasi.user_id | Siapa mengubah apa, dan siapa menerima peringatan |

## Rules

- All tables use a BIGINT identity primary key (PK).
- TIMESTAMPTZ time (Asia/Jakarta time zone in the app),
- uang NUMERIC(14,2) in rupiah, 
- the created_at/updated_at column.
- Operational data always includes the sppg_id to ensure compatibility with multiple SPPGs.