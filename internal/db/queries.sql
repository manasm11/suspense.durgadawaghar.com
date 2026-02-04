-- name: CreateParty :one
INSERT INTO parties (name, location)
VALUES (?, ?)
RETURNING *;

-- name: GetPartyByID :one
SELECT * FROM parties WHERE id = ?;

-- name: GetPartyByName :one
SELECT * FROM parties WHERE name = ? LIMIT 1;

-- name: ListParties :many
SELECT * FROM parties ORDER BY name;

-- name: CreateIdentifier :one
INSERT INTO identifiers (party_id, type, value)
VALUES (?, ?, ?)
ON CONFLICT (type, value) DO UPDATE SET party_id = excluded.party_id
RETURNING *;

-- name: GetIdentifierByTypeValue :one
SELECT * FROM identifiers WHERE type = ? AND value = ? LIMIT 1;

-- name: GetIdentifiersByPartyID :many
SELECT * FROM identifiers WHERE party_id = ?;

-- name: FindPartiesByIdentifierValue :many
SELECT DISTINCT p.*, i.type as match_type, i.value as match_value
FROM parties p
JOIN identifiers i ON p.id = i.party_id
WHERE i.value = ?;

-- name: FindPartiesByIdentifierValues :many
SELECT DISTINCT p.*, i.type as match_type, i.value as match_value
FROM parties p
JOIN identifiers i ON p.id = i.party_id
WHERE i.value IN (sqlc.slice('values'));

-- name: CreateTransaction :one
INSERT INTO transactions (party_id, amount, transaction_date, payment_mode, narration, bank)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetTransactionsByPartyID :many
SELECT * FROM transactions
WHERE party_id = ?
ORDER BY transaction_date DESC;

-- name: CountTransactionsByPartyID :one
SELECT COUNT(*) as count FROM transactions WHERE party_id = ?;

-- name: GetRecentTransactionsByPartyID :many
SELECT * FROM transactions
WHERE party_id = ?
ORDER BY transaction_date DESC
LIMIT ?;

-- name: GetPartyWithTransactionCount :one
SELECT p.*, COUNT(t.id) as transaction_count, SUM(t.amount) as total_amount
FROM parties p
LEFT JOIN transactions t ON p.id = t.party_id
WHERE p.id = ?
GROUP BY p.id;

-- name: GetAllPartiesWithStats :many
SELECT p.*, COUNT(t.id) as transaction_count, COALESCE(SUM(t.amount), 0) as total_amount
FROM parties p
LEFT JOIN transactions t ON p.id = t.party_id
GROUP BY p.id
ORDER BY transaction_count DESC;

-- name: FindPartiesByNarrationPattern :many
SELECT DISTINCT p.*, t.narration as match_narration
FROM parties p
JOIN transactions t ON p.id = t.party_id
WHERE t.narration LIKE ?
LIMIT 50;

-- name: FindPartiesByNarrationPatternAndBank :many
SELECT DISTINCT p.*, t.narration as match_narration
FROM parties p
JOIN transactions t ON p.id = t.party_id
WHERE t.narration LIKE ? AND t.bank = ?
LIMIT 50;

-- name: FindPartiesByIdentifierValuesAndBank :many
SELECT DISTINCT p.*, i.type as match_type, i.value as match_value
FROM parties p
JOIN identifiers i ON p.id = i.party_id
JOIN transactions t ON p.id = t.party_id
WHERE i.value IN (sqlc.slice('values')) AND t.bank = ?;

-- name: GetPartyWithTransactionCountByBank :one
SELECT p.*, COUNT(t.id) as transaction_count, COALESCE(SUM(t.amount), 0) as total_amount
FROM parties p
LEFT JOIN transactions t ON p.id = t.party_id AND t.bank = ?
WHERE p.id = ?
GROUP BY p.id;

-- name: GetRecentTransactionsByPartyIDAndBank :many
SELECT * FROM transactions
WHERE party_id = ? AND bank = ?
ORDER BY transaction_date DESC
LIMIT ?;
