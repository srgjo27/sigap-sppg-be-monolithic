# MVP-001 Authentication & roles

The specifications for the M1 (P0) module—login, role-based access, and activity logs—are broken down into 8 endpoints, each of which can be tested separately. All endpoints are prefixed with `/api/v1` and use the `users` and `audit_log` tables, plus a new table called `refresh_token`.

## Main Feature

Login, role-based access, log aktivitas

## Endpoint Summary

| ID | Endpoint | Function | Access |
|---|---|---|---|
| MVP-001.1 | POST /users | Create a user account | admin, kepala_sppg |
| MVP-001.2 | POST /auth/login | Login, dapat token | publik |
| MVP-001.3 | POST /auth/refresh, POST /auth/logout |Refresh and revoke the token | user login |
| MVP-001.4 | GET /auth/me | Active User Profiles and Permissions | user login |
| MVP-001.5 | PATCH /users/{id} | Edit data, roles, or deactivate an account | admin, kepala_sppg |
| MVP-001.6 | PUT /auth/password | Change Your Password Yourself | user login |
| MVP-001.7 | GET /audit-logs | View the activity log | admin, kepala_sppg, pengawas |
| MVP-001.8 | Middleware RBAC | Deny access outside of roles and SPPG | All endpoints are protected |

## Role Matrix

This rule is applied by the RBAC middleware (MVP-001.8) to all subsequent modules.

| Role | Manage users | View the audit log | Data scope |
|---|---|---|---|
| admin | All roles, all SPPG | All | All SPPG |
| kepala_sppg | Roles other than administrator and supervisor; the SPPG itself | the SPPG itself | the SPPG itself |
| ahli_gizi, akuntan, petugas_dapur, petugas_distribusi | No | No | the SPPG itself |
| pic_sekolah | No | No | the Sekolah itself |
| pengawas | No | All (read-only) | the SPPG itself, read-only |

## Additional schemes

Refresh tokens are stored in the database so they can be revoked when a user logs out or when an account is deactivated.
Read `/db/schema.sql` before work.

## General Rules 

- JWT access token (HS256 or RS256), valid for 15 minutes; contains sub (user id), `role, sppg_id, sekolah_id`.
- Random 256-bit refresh token, valid for 7 days, rotated each time it is used.
- Passwords are hashed using argon2id (or bcrypt with a cost of ≥ 12); must be at least 8 characters long and contain both letters and numbers.
- All changes to user data and every login attempt (successful or failed) are recorded in the `audit_log`.
- Standardized error format: `{ “error”: { “code”: “VALIDATION_ERROR”, “message”: “...”, “fields”: { ‘email’: “already registered” } } }`.

## MVP-001.1 Create user

### Goal
The admin or SPPG director can create new accounts for pegawai, relawan, PIC sekolah, or pengawas. There is no self-registration.

### Actor
admin, kepala_sppg

### Input

POST `/api/v1/users`

- nama
- email
- no_hp (optional)
- peran
- sppg_id (required, except for pengawas or admin roles)
- sekolah_id (required when the role pic_sekolah)
- password_awal (optional; if left blank, the system generates a random password)

### Rules

- email must be unique (case-insensitive, disimpan lowercase)
- nama is required, 2–120 karakter
- peran must be one of the values enum `user_role`
- kepala_sppg may only create user in `sppg_id` belongs to him and he must not create a role admin or pengawas
- sekolah_id must belong to sppg_id the same
- no_hp format Indonesia (08… atau +628…), 10–15 digit
- new accounts always `wajib_ganti_password` = `true`
- password is never returned in the response, except for a newly generated random password (displayed once)

### Success

HTTP 201
Returns created user: `id, nama, email, no_hp, peran, sppg_id, sekolah_id, aktif, created_at` (+ `password_hash` when a system is created)

### Failure

400: invalid input
401: tidak login atau token kedaluwarsa
403: peran tidak berhak, atau membuat user di SPPG lain
404: sppg_id atau sekolah_id tidak ditemukan
409: email already exists
500: unexpected internal error

### Acceptance Criteria

[ ] Request is validated.
[ ] Duplicate email is rejected with 409.
[ ] kepala_sppg cannot create users outside their SPPG or with role admin/pengawas.
[ ] Password is stored hashed, never returned.
[ ] Data is persisted to PostgreSQL.
[ ] Action is written to audit_log.
[ ] Created entity is returned.
[ ] Unit tests exist.
[ ] Integration test exists.

## MVP-001.2 Login

### Goal

Users log in with their email and password and receive an access token and a refresh token.

### Actor

All role

### Input 

POST `/api/v1/auth/login`

- email
- password

### Rules

- email and password are required
- account with `aktif` set to `false` cannot log in
- after 5 consecutive failed attempts, the account is locked for 15 minutes (`terkunci_sampai`)
- a successful login resets `gagal_login` to 0 and populates `last_login_at`
- the failure message does not distinguish between an incorrect email and an incorrect password
- rate limit: 10 attempts per minute per IP
- the refresh token is stored as a hash in `refresh_token`

### Success

HTTP 200
Returns `access_token, refresh_token, expires_in, wajib_ganti_password`, and object user (id, nama, peran, sppg_id, sekolah_id)

### Failure

400: invalid input
401: email atau password salah
403: akun nonaktif
423: akun terkunci sementara (sertakan `terkunci_sampai`)
429: terlalu banyak percobaan dari IP yang sama
500: unexpected internal error

### Acceptance Criteria

[ ] Valid credentials return both tokens.
[ ] Wrong email and wrong password return the same 401 message.
[ ] Inactive account is rejected.
[ ] 5 failed attempts lock the account for 15 minutes.
[ ] Successful and failed logins are written to audit_log with IP.
[ ] Unit tests exist.
[ ] Integration test exists.

## MVP-001.3 Refresh token & logout

### Goal

Users remain logged in without entering a password every 15 minutes and can log out securely.

### Actor

Logged-in users

### Input

POST `/api/v1/auth/refresh` 
POST `/api/v1/auth/logout`

- refresh_token

### Rules

- the refresh token must exist, not have been revoked, and not have expired
- each refresh revokes the old token and issues a new token pair (rotation)
- if a revoked token is used again, all of that user’s refresh tokens are revoked (token theft detection)
- disabled users cannot refresh
- logout revokes the refresh token sent; `?all=true` revokes all of the user’s sessions

### Success

Refresh: HTTP 200, returns `access_token`, `refresh_token`, `expires_in`
Logout: HTTP 204, tanpa body

### Failure

400: invalid input
401: refresh token tidak valid, dicabut, atau kedaluwarsa
403: akun nonaktif
500: unexpected internal error

### Acceptance Criteria

[ ] Valid refresh returns a new token pair and revokes the old one.
[ ] Reused revoked token revokes every session of that user.
[ ] Logout makes the token unusable.
[ ] Unit tests exist.
[ ] Integration test exists.

## MVP-001.4 Current user profile

### Goal

The frontend knows who is logged in, their role, and which menu items should be displayed.

### Actor

Logged-in users

### Input 

GET `/api/v1/auth/me` (header Authorization: Bearer <access_token>)

### Rules

- data is retrieved directly from the database—not just from the token’s contents—so that changes to roles or statuses take effect immediately
- the list of `permissions` is derived from the role matrix

### Success

HTTP 200
Returns `id, nama, email, no_hp, peran, sppg (id, nama), sekolah (id, nama), wajib_ganti_password, permissions[]`

### Failure

401: token tidak ada, tidak valid, atau kedaluwarsa
403: akun sudah dinonaktifkan
500: unexpected internal error

Acceptance Criteria
[ ] Returns the profile of the token owner only.
[ ] Deactivated user receives 403 even with a valid token.
[ ] Unit tests exist.
[ ] Integration test exists.