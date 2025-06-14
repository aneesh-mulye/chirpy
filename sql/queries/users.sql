-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, hashed_password)
VALUES (
	gen_random_uuid(),
	CURRENT_TIMESTAMP,
	CURRENT_TIMESTAMP,
	$1,
	$2
)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: Reset :exec
TRUNCATE users CASCADE;
