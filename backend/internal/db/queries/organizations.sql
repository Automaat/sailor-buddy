-- name: CreateOrganization :one
INSERT INTO organizations (name, slug, description, logo_url, pzz_club_number, city, website)
VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING *;

-- name: GetOrganizationBySlug :one
SELECT * FROM organizations WHERE slug = $1;

-- name: GetOrganizationByID :one
SELECT * FROM organizations WHERE id = $1;

-- name: ListUserOrganizations :many
SELECT o.*, om.role
FROM organizations o
JOIN org_members om ON om.org_id = o.id
WHERE om.user_id = $1
ORDER BY o.name;

-- name: UpdateOrganization :exec
UPDATE organizations SET
    name = $1, description = $2, logo_url = $3,
    pzz_club_number = $4, city = $5, website = $6,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $7;

-- name: DeleteOrganization :exec
DELETE FROM organizations WHERE id = $1;

-- name: AddOrgMember :one
INSERT INTO org_members (org_id, user_id, role)
VALUES ($1, $2, $3) RETURNING *;

-- name: GetOrgMembership :one
SELECT om.*, o.slug
FROM org_members om
JOIN organizations o ON o.id = om.org_id
WHERE om.org_id = $1 AND om.user_id = $2;

-- name: GetOrgMembershipBySlug :one
SELECT om.id, om.org_id, om.user_id, om.role, om.joined_at
FROM org_members om
JOIN organizations o ON o.id = om.org_id
WHERE o.slug = $1 AND om.user_id = $2;

-- name: ListOrgMembers :many
SELECT om.id, om.org_id, om.user_id, om.role, om.joined_at,
       u.name AS user_name, u.email AS user_email, u.avatar_url AS user_avatar_url
FROM org_members om
JOIN users u ON u.id = om.user_id
WHERE om.org_id = $1
ORDER BY om.role, u.name;

-- name: UpdateOrgMemberRole :exec
UPDATE org_members SET role = $1 WHERE id = $2 AND org_id = $3;

-- name: RemoveOrgMember :exec
DELETE FROM org_members WHERE id = $1 AND org_id = $2;

-- name: CountOrgAdmins :one
SELECT COUNT(*)::BIGINT FROM org_members WHERE org_id = $1 AND role = 'admin';

-- name: CreateOrgInvite :one
INSERT INTO org_invites (org_id, token, role, created_by, expires_at, max_uses)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;

-- name: GetOrgInviteByToken :one
SELECT oi.*, o.name AS org_name, o.slug AS org_slug
FROM org_invites oi
JOIN organizations o ON o.id = oi.org_id
WHERE oi.token = $1;

-- name: ListOrgInvites :many
SELECT oi.*, u.name AS creator_name
FROM org_invites oi
JOIN users u ON u.id = oi.created_by
WHERE oi.org_id = $1
ORDER BY oi.created_at DESC;

-- name: IncrementInviteUseCount :execrows
UPDATE org_invites SET use_count = use_count + 1 WHERE id = $1 AND (max_uses IS NULL OR use_count < max_uses);

-- name: DeleteOrgInvite :exec
DELETE FROM org_invites WHERE id = $1 AND org_id = $2;
