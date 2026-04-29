package social

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"social-network/backend/internal/auth"
)

const (
	maxGroupTitleLength       = 120
	maxGroupDescriptionLength = 2000
)

type GroupUser struct {
	ID        string `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Nickname  string `json:"nickname,omitempty"`
	AvatarURL string `json:"avatarUrl,omitempty"`
}

type Group struct {
	ID                   string    `json:"id"`
	Title                string    `json:"title"`
	Description          string    `json:"description"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
	MembersCount         int       `json:"membersCount"`
	PostsCount           int       `json:"postsCount"`
	EventsCount          int       `json:"eventsCount"`
	PendingRequestsCount int       `json:"pendingRequestsCount"`
	IsMember             bool      `json:"isMember"`
	Role                 string    `json:"role,omitempty"`
	JoinRequestID        string    `json:"joinRequestId,omitempty"`
	JoinRequestStatus    string    `json:"joinRequestStatus,omitempty"`
	Creator              GroupUser `json:"creator"`
}

type CreateGroupInput struct {
	Title       string
	Description string
}

func (s Service) Groups(ctx context.Context, viewerID string) ([]Group, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`
			SELECT
				g.id,
				g.title,
				g.description,
				g.created_at,
				g.updated_at,
				u.id,
				u.first_name,
				u.last_name,
				u.nickname,
				u.avatar_url,
				gm.role,
				viewer_request.id,
				viewer_request.status,
				COALESCE(member_counts.members_count, 0) AS members_count,
				COALESCE(post_counts.posts_count, 0) AS posts_count,
				COALESCE(event_counts.events_count, 0) AS events_count,
				COALESCE(request_counts.pending_requests_count, 0) AS pending_requests_count
			FROM groups g
			INNER JOIN users u ON u.id = g.creator_id
			LEFT JOIN group_memberships gm
				ON gm.group_id = g.id
				AND gm.user_id = $1
			LEFT JOIN LATERAL (
				SELECT gjr.id, gjr.status
				FROM group_join_requests gjr
				WHERE
					gjr.group_id = g.id
					AND gjr.requester_id = $1
					AND gjr.status = 'pending'
				ORDER BY gjr.created_at DESC, gjr.id DESC
				LIMIT 1
			) viewer_request ON TRUE
			LEFT JOIN (
				SELECT group_id, COUNT(*)::INT AS members_count
				FROM group_memberships
				GROUP BY group_id
			) member_counts ON member_counts.group_id = g.id
			LEFT JOIN (
				SELECT group_id, COUNT(*)::INT AS posts_count
				FROM group_posts
				GROUP BY group_id
			) post_counts ON post_counts.group_id = g.id
			LEFT JOIN (
				SELECT group_id, COUNT(*)::INT AS events_count
				FROM group_events
				GROUP BY group_id
			) event_counts ON event_counts.group_id = g.id
			LEFT JOIN (
				SELECT group_id, COUNT(*)::INT AS pending_requests_count
				FROM group_join_requests
				WHERE status = 'pending'
				GROUP BY group_id
			) request_counts ON request_counts.group_id = g.id
			ORDER BY
				CASE WHEN gm.user_id IS NULL THEN 1 ELSE 0 END,
				g.created_at DESC,
				g.id DESC
			LIMIT 50
		`,
		viewerID,
	)
	if err != nil {
		return nil, fmt.Errorf("query groups: %w", err)
	}
	defer rows.Close()

	groups := make([]Group, 0)
	for rows.Next() {
		group, scanErr := s.scanGroup(rows)
		if scanErr != nil {
			return nil, scanErr
		}

		groups = append(groups, group)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate groups: %w", err)
	}

	return groups, nil
}

func (s Service) CreateGroup(ctx context.Context, creator auth.User, input CreateGroupInput) (Group, error) {
	normalizedInput, err := normalizeCreateGroupInput(input)
	if err != nil {
		return Group{}, err
	}

	groupID, err := newToken(16)
	if err != nil {
		return Group{}, fmt.Errorf("generate group id: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Group{}, fmt.Errorf("begin create group transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(
		ctx,
		`
			INSERT INTO groups (id, creator_id, title, description)
			VALUES ($1, $2, $3, $4)
		`,
		groupID,
		creator.ID,
		normalizedInput.Title,
		normalizedInput.Description,
	); err != nil {
		return Group{}, fmt.Errorf("insert group: %w", err)
	}

	if _, err = tx.ExecContext(
		ctx,
		`
			INSERT INTO group_memberships (group_id, user_id, role)
			VALUES ($1, $2, 'creator')
			ON CONFLICT (group_id, user_id) DO NOTHING
		`,
		groupID,
		creator.ID,
	); err != nil {
		return Group{}, fmt.Errorf("insert group creator membership: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return Group{}, fmt.Errorf("commit create group transaction: %w", err)
	}

	return s.loadGroupByIDWithReader(ctx, s.db, creator.ID, groupID)
}

func (s Service) GroupByID(ctx context.Context, viewerID, groupID string) (Group, error) {
	return s.loadGroupByIDWithReader(ctx, s.db, viewerID, groupID)
}

func (s Service) JoinGroup(ctx context.Context, userID, groupID string) (Group, error) {
	normalizedGroupID := strings.TrimSpace(groupID)
	if normalizedGroupID == "" {
		return Group{}, &ValidationError{
			Message: "Choose a group to join.",
			Fields: map[string]string{
				"groupId": "Choose a group to join.",
			},
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Group{}, fmt.Errorf("begin join group transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var exists bool
	var creatorID string
	var groupTitle string
	if err = tx.QueryRowContext(
		ctx,
		`
			SELECT
				EXISTS (SELECT 1 FROM groups WHERE id = $1),
				COALESCE((SELECT creator_id FROM groups WHERE id = $1), ''),
				COALESCE((SELECT title FROM groups WHERE id = $1), '')
		`,
		normalizedGroupID,
	).Scan(&exists, &creatorID, &groupTitle); err != nil {
		return Group{}, fmt.Errorf("check group existence: %w", err)
	}
	if !exists {
		return Group{}, ErrNotFound
	}

	var existingRole sql.NullString
	if err = tx.QueryRowContext(
		ctx,
		`
			SELECT role
			FROM group_memberships
			WHERE group_id = $1 AND user_id = $2
		`,
		normalizedGroupID,
		userID,
	).Scan(&existingRole); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Group{}, fmt.Errorf("load existing group membership: %w", err)
	}

	if existingRole.Valid {
		if err = tx.Commit(); err != nil {
			return Group{}, fmt.Errorf("commit join group transaction: %w", err)
		}

		return s.loadGroupByIDWithReader(ctx, s.db, userID, normalizedGroupID)
	}

	var pendingRequestID string
	if err = tx.QueryRowContext(
		ctx,
		`
			SELECT id
			FROM group_join_requests
			WHERE group_id = $1 AND requester_id = $2 AND status = 'pending'
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		`,
		normalizedGroupID,
		userID,
	).Scan(&pendingRequestID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Group{}, fmt.Errorf("load existing group join request: %w", err)
	}

	var createdNotification Notification
	if strings.TrimSpace(pendingRequestID) == "" {
		requestID, tokenErr := newToken(16)
		if tokenErr != nil {
			return Group{}, fmt.Errorf("generate group join request id: %w", tokenErr)
		}

		if _, err = tx.ExecContext(
			ctx,
			`
				INSERT INTO group_join_requests (id, group_id, requester_id, status)
				VALUES ($1, $2, $3, 'pending')
			`,
			requestID,
			normalizedGroupID,
			userID,
		); err != nil {
			return Group{}, fmt.Errorf("insert group join request: %w", err)
		}

		requesterIdentity, identityErr := s.loadUserIdentity(ctx, tx, userID)
		if identityErr != nil {
			return Group{}, identityErr
		}

		createdNotification, err = s.insertNotification(
			ctx,
			tx,
			creatorID,
			"group_join_request_received",
			"New group join request",
			fmt.Sprintf("%s requested to join \"%s\".", requesterIdentity.DisplayName(), groupTitle),
			"group",
			normalizedGroupID,
		)
		if err != nil {
			return Group{}, err
		}
	}

	if err = tx.Commit(); err != nil {
		return Group{}, fmt.Errorf("commit join group transaction: %w", err)
	}

	if createdNotification.ID != "" {
		s.publisher.PublishToUser(creatorID, "notification.created", NotificationEvent{
			Notification: createdNotification,
		})
	}

	return s.loadGroupByIDWithReader(ctx, s.db, userID, normalizedGroupID)
}

func (s Service) loadGroupByIDWithReader(ctx context.Context, reader sqlReader, viewerID, groupID string) (Group, error) {
	normalizedGroupID := strings.TrimSpace(groupID)
	if normalizedGroupID == "" {
		return Group{}, &ValidationError{
			Message: "Choose a group to open.",
			Fields: map[string]string{
				"groupId": "Choose a group to open.",
			},
		}
	}

	row := reader.QueryRowContext(
		ctx,
		`
			SELECT
				g.id,
				g.title,
				g.description,
				g.created_at,
				g.updated_at,
				u.id,
				u.first_name,
				u.last_name,
				u.nickname,
				u.avatar_url,
				gm.role,
				viewer_request.id,
				viewer_request.status,
				COALESCE(member_counts.members_count, 0) AS members_count,
				COALESCE(post_counts.posts_count, 0) AS posts_count,
				COALESCE(event_counts.events_count, 0) AS events_count,
				COALESCE(request_counts.pending_requests_count, 0) AS pending_requests_count
			FROM groups g
			INNER JOIN users u ON u.id = g.creator_id
			LEFT JOIN group_memberships gm
				ON gm.group_id = g.id
				AND gm.user_id = $1
			LEFT JOIN LATERAL (
				SELECT gjr.id, gjr.status
				FROM group_join_requests gjr
				WHERE
					gjr.group_id = g.id
					AND gjr.requester_id = $1
					AND gjr.status = 'pending'
				ORDER BY gjr.created_at DESC, gjr.id DESC
				LIMIT 1
			) viewer_request ON TRUE
			LEFT JOIN (
				SELECT group_id, COUNT(*)::INT AS members_count
				FROM group_memberships
				GROUP BY group_id
			) member_counts ON member_counts.group_id = g.id
			LEFT JOIN (
				SELECT group_id, COUNT(*)::INT AS posts_count
				FROM group_posts
				GROUP BY group_id
			) post_counts ON post_counts.group_id = g.id
			LEFT JOIN (
				SELECT group_id, COUNT(*)::INT AS events_count
				FROM group_events
				GROUP BY group_id
			) event_counts ON event_counts.group_id = g.id
			LEFT JOIN (
				SELECT group_id, COUNT(*)::INT AS pending_requests_count
				FROM group_join_requests
				WHERE status = 'pending'
				GROUP BY group_id
			) request_counts ON request_counts.group_id = g.id
			WHERE g.id = $2
		`,
		viewerID,
		normalizedGroupID,
	)

	group, err := s.scanGroup(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Group{}, ErrNotFound
		}

		return Group{}, fmt.Errorf("load group: %w", err)
	}

	return group, nil
}

type groupScanner interface {
	Scan(dest ...any) error
}

func (s Service) scanGroup(scanner groupScanner) (Group, error) {
	var (
		group             Group
		creatorNickname   sql.NullString
		creatorAvatarURL  sql.NullString
		membershipRole    sql.NullString
		joinRequestID     sql.NullString
		joinRequestStatus sql.NullString
	)
	if err := scanner.Scan(
		&group.ID,
		&group.Title,
		&group.Description,
		&group.CreatedAt,
		&group.UpdatedAt,
		&group.Creator.ID,
		&group.Creator.FirstName,
		&group.Creator.LastName,
		&creatorNickname,
		&creatorAvatarURL,
		&membershipRole,
		&joinRequestID,
		&joinRequestStatus,
		&group.MembersCount,
		&group.PostsCount,
		&group.EventsCount,
		&group.PendingRequestsCount,
	); err != nil {
		return Group{}, err
	}

	group.Creator.Nickname = nullStringValue(creatorNickname)
	if creatorAvatarURL.Valid {
		group.Creator.AvatarURL = s.publicURL(creatorAvatarURL.String)
	}

	group.Role = nullStringValue(membershipRole)
	group.JoinRequestID = nullStringValue(joinRequestID)
	group.JoinRequestStatus = nullStringValue(joinRequestStatus)
	group.IsMember = group.Role != ""
	return group, nil
}

func normalizeCreateGroupInput(input CreateGroupInput) (CreateGroupInput, error) {
	normalized := CreateGroupInput{
		Title:       strings.TrimSpace(input.Title),
		Description: strings.TrimSpace(input.Description),
	}

	fieldErrors := make(map[string]string)
	if normalized.Title == "" {
		fieldErrors["title"] = "Group title is required."
	} else if len(normalized.Title) > maxGroupTitleLength {
		fieldErrors["title"] = "Group title must be 120 characters or fewer."
	}

	if normalized.Description == "" {
		fieldErrors["description"] = "Group description is required."
	} else if len(normalized.Description) > maxGroupDescriptionLength {
		fieldErrors["description"] = "Group description must be 2000 characters or fewer."
	}

	if len(fieldErrors) > 0 {
		return CreateGroupInput{}, &ValidationError{
			Message: "Please correct the group details.",
			Fields:  fieldErrors,
		}
	}

	return normalized, nil
}
