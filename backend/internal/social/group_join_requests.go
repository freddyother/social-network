package social

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type GroupJoinRequest struct {
	ID          string     `json:"id"`
	GroupID     string     `json:"groupId"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"createdAt"`
	RespondedAt *time.Time `json:"respondedAt,omitempty"`
	Requester   GroupUser  `json:"requester"`
}

func (s Service) GroupJoinRequests(ctx context.Context, viewerID, groupID string) ([]GroupJoinRequest, error) {
	normalizedGroupID, _, err := s.requireGroupCreator(ctx, s.db, viewerID, groupID)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(
		ctx,
		`
			SELECT
				gjr.id,
				gjr.group_id,
				gjr.status,
				gjr.created_at,
				gjr.responded_at,
				u.id,
				u.first_name,
				u.last_name,
				u.nickname,
				u.avatar_url
			FROM group_join_requests gjr
			INNER JOIN users u ON u.id = gjr.requester_id
			WHERE gjr.group_id = $1 AND gjr.status = 'pending'
			ORDER BY gjr.created_at DESC, gjr.id DESC
		`,
		normalizedGroupID,
	)
	if err != nil {
		return nil, fmt.Errorf("query group join requests: %w", err)
	}
	defer rows.Close()

	requests := make([]GroupJoinRequest, 0)
	for rows.Next() {
		request, scanErr := s.scanGroupJoinRequest(rows)
		if scanErr != nil {
			return nil, scanErr
		}

		requests = append(requests, request)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate group join requests: %w", err)
	}

	return requests, nil
}

func (s Service) RespondToGroupJoinRequest(ctx context.Context, viewerID, groupID, requestID string, accept bool) (GroupJoinRequest, error) {
	normalizedRequestID := strings.TrimSpace(requestID)
	if normalizedRequestID == "" {
		return GroupJoinRequest{}, &ValidationError{
			Message: "Choose a join request first.",
			Fields: map[string]string{
				"requestId": "Choose a join request first.",
			},
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return GroupJoinRequest{}, fmt.Errorf("begin group join request response transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	normalizedGroupID, groupTitle, err := s.requireGroupCreator(ctx, tx, viewerID, groupID)
	if err != nil {
		return GroupJoinRequest{}, err
	}

	var request GroupJoinRequest
	var requesterNickname sql.NullString
	var requesterAvatarURL sql.NullString
	var respondedAt sql.NullTime
	if err = tx.QueryRowContext(
		ctx,
		`
			SELECT
				gjr.id,
				gjr.group_id,
				gjr.status,
				gjr.created_at,
				gjr.responded_at,
				u.id,
				u.first_name,
				u.last_name,
				u.nickname,
				u.avatar_url
			FROM group_join_requests gjr
			INNER JOIN users u ON u.id = gjr.requester_id
			WHERE gjr.id = $1 AND gjr.group_id = $2
		`,
		normalizedRequestID,
		normalizedGroupID,
	).Scan(
		&request.ID,
		&request.GroupID,
		&request.Status,
		&request.CreatedAt,
		&respondedAt,
		&request.Requester.ID,
		&request.Requester.FirstName,
		&request.Requester.LastName,
		&requesterNickname,
		&requesterAvatarURL,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return GroupJoinRequest{}, ErrNotFound
		}

		return GroupJoinRequest{}, fmt.Errorf("load group join request: %w", err)
	}

	request.Requester.Nickname = nullStringValue(requesterNickname)
	if requesterAvatarURL.Valid {
		request.Requester.AvatarURL = s.publicURL(requesterAvatarURL.String)
	}
	if respondedAt.Valid {
		value := respondedAt.Time
		request.RespondedAt = &value
	}

	if request.Status != "pending" {
		return GroupJoinRequest{}, ErrAlreadyHandled
	}

	newStatus := "declined"
	var createdNotification Notification
	if accept {
		newStatus = "accepted"
		if _, err = tx.ExecContext(
			ctx,
			`
				INSERT INTO group_memberships (group_id, user_id, role)
				VALUES ($1, $2, 'member')
				ON CONFLICT (group_id, user_id) DO NOTHING
			`,
			normalizedGroupID,
			request.Requester.ID,
		); err != nil {
			return GroupJoinRequest{}, fmt.Errorf("insert accepted group membership: %w", err)
		}

		createdNotification, err = s.insertNotification(
			ctx,
			tx,
			request.Requester.ID,
			"group_join_request_accepted",
			"Group join request accepted",
			fmt.Sprintf("Your request to join \"%s\" was accepted.", groupTitle),
			"group",
			normalizedGroupID,
		)
		if err != nil {
			return GroupJoinRequest{}, err
		}
	}

	var updatedRespondedAt time.Time
	if err = tx.QueryRowContext(
		ctx,
		`
			UPDATE group_join_requests
			SET status = $1, responded_at = NOW()
			WHERE id = $2
			RETURNING responded_at
		`,
		newStatus,
		normalizedRequestID,
	).Scan(&updatedRespondedAt); err != nil {
		return GroupJoinRequest{}, fmt.Errorf("update group join request: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return GroupJoinRequest{}, fmt.Errorf("commit group join request response transaction: %w", err)
	}

	request.Status = newStatus
	request.RespondedAt = &updatedRespondedAt

	if accept && createdNotification.ID != "" {
		s.publisher.PublishToUser(request.Requester.ID, "notification.created", NotificationEvent{
			Notification: createdNotification,
		})
	}

	return request, nil
}

func (s Service) requireGroupCreator(ctx context.Context, reader sqlReader, userID, groupID string) (string, string, error) {
	normalizedGroupID := strings.TrimSpace(groupID)
	if normalizedGroupID == "" {
		return "", "", &ValidationError{
			Message: "Choose a group first.",
			Fields: map[string]string{
				"groupId": "Choose a group first.",
			},
		}
	}

	var (
		exists bool
		title  string
		role   sql.NullString
	)
	if err := reader.QueryRowContext(
		ctx,
		`
			SELECT
				EXISTS (SELECT 1 FROM groups WHERE id = $1) AS group_exists,
				COALESCE((SELECT title FROM groups WHERE id = $1), ''),
				(
					SELECT gm.role
					FROM group_memberships gm
					WHERE gm.group_id = $1 AND gm.user_id = $2
					LIMIT 1
				) AS membership_role
		`,
		normalizedGroupID,
		userID,
	).Scan(&exists, &title, &role); err != nil {
		return "", "", fmt.Errorf("load group creator access: %w", err)
	}

	if !exists {
		return "", "", ErrNotFound
	}

	if strings.TrimSpace(role.String) != "creator" {
		return "", "", ErrForbidden
	}

	return normalizedGroupID, title, nil
}

type groupJoinRequestScanner interface {
	Scan(dest ...any) error
}

func (s Service) scanGroupJoinRequest(scanner groupJoinRequestScanner) (GroupJoinRequest, error) {
	var (
		request            GroupJoinRequest
		respondedAt        sql.NullTime
		requesterNickname  sql.NullString
		requesterAvatarURL sql.NullString
	)
	if err := scanner.Scan(
		&request.ID,
		&request.GroupID,
		&request.Status,
		&request.CreatedAt,
		&respondedAt,
		&request.Requester.ID,
		&request.Requester.FirstName,
		&request.Requester.LastName,
		&requesterNickname,
		&requesterAvatarURL,
	); err != nil {
		return GroupJoinRequest{}, fmt.Errorf("scan group join request: %w", err)
	}

	request.Requester.Nickname = nullStringValue(requesterNickname)
	if requesterAvatarURL.Valid {
		request.Requester.AvatarURL = s.publicURL(requesterAvatarURL.String)
	}
	if respondedAt.Valid {
		value := respondedAt.Time
		request.RespondedAt = &value
	}

	return request, nil
}
