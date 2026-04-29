package social

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"social-network/backend/internal/auth"
)

const (
	groupInviteMessagePrefix = "[nexo-group-invite]?"
	maxGroupInviteNoteLength = 500
)

type InviteUserToGroupInput struct {
	RecipientID string
	Note        string
}

func (s Service) GroupInviteCandidates(ctx context.Context, viewerID, groupID string) ([]SuggestedUser, error) {
	normalizedGroupID, err := s.requireGroupMembership(ctx, s.db, viewerID, groupID)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(
		ctx,
		`
			SELECT
				u.id,
				u.first_name,
				u.last_name,
				u.nickname,
				u.about_me,
				u.profile_visibility,
				CASE
					WHEN f.follower_id IS NOT NULL THEN 'following'
					WHEN fr.id IS NOT NULL THEN 'requested'
					ELSE 'not_following'
				END AS relationship_status
			FROM users u
			LEFT JOIN followers f
				ON f.follower_id = $1
				AND f.followee_id = u.id
			LEFT JOIN follow_requests fr
				ON fr.sender_id = $1
				AND fr.recipient_id = u.id
				AND fr.status = 'pending'
			WHERE
				u.id <> $1
				AND NOT EXISTS (
					SELECT 1
					FROM group_memberships gm
					WHERE gm.group_id = $2 AND gm.user_id = u.id
				)
				AND NOT EXISTS (
					SELECT 1
					FROM group_invitations gi
					WHERE
						gi.group_id = $2
						AND gi.invitee_id = u.id
						AND gi.status = 'pending'
				)
				AND NOT EXISTS (
					SELECT 1
					FROM group_join_requests gjr
					WHERE
						gjr.group_id = $2
						AND gjr.requester_id = u.id
						AND gjr.status = 'pending'
				)
			ORDER BY u.created_at DESC
			LIMIT 30
		`,
		viewerID,
		normalizedGroupID,
	)
	if err != nil {
		return nil, fmt.Errorf("query group invite candidates: %w", err)
	}
	defer rows.Close()

	users := make([]SuggestedUser, 0)
	for rows.Next() {
		var user SuggestedUser
		var nickname sql.NullString
		var aboutMe sql.NullString
		if err := rows.Scan(
			&user.ID,
			&user.FirstName,
			&user.LastName,
			&nickname,
			&aboutMe,
			&user.ProfileVisibility,
			&user.RelationshipStatus,
		); err != nil {
			return nil, fmt.Errorf("scan group invite candidate: %w", err)
		}

		user.Nickname = nullStringValue(nickname)
		user.AboutMe = nullStringValue(aboutMe)
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate group invite candidates: %w", err)
	}

	return users, nil
}

func (s Service) InviteUserToGroup(ctx context.Context, sender auth.User, groupID string, input InviteUserToGroupInput) (PrivateMessage, error) {
	normalizedInput, err := normalizeInviteUserToGroupInput(input)
	if err != nil {
		return PrivateMessage{}, err
	}

	group, err := s.loadGroupByIDWithReader(ctx, s.db, sender.ID, groupID)
	if err != nil {
		return PrivateMessage{}, err
	}
	if !group.IsMember {
		return PrivateMessage{}, ErrForbidden
	}

	var alreadyMember bool
	if err := s.db.QueryRowContext(
		ctx,
		`
			SELECT EXISTS (
				SELECT 1
				FROM group_memberships
				WHERE group_id = $1 AND user_id = $2
			)
		`,
		group.ID,
		normalizedInput.RecipientID,
	).Scan(&alreadyMember); err != nil {
		return PrivateMessage{}, fmt.Errorf("check invited user membership: %w", err)
	}

	if alreadyMember {
		return PrivateMessage{}, &ValidationError{
			Message: "That person already belongs to this group.",
			Fields: map[string]string{
				"recipientId": "That person already belongs to this group.",
			},
		}
	}

	var hasPendingInvitation bool
	if err := s.db.QueryRowContext(
		ctx,
		`
			SELECT EXISTS (
				SELECT 1
				FROM group_invitations
				WHERE
					group_id = $1
					AND invitee_id = $2
					AND status = 'pending'
			)
		`,
		group.ID,
		normalizedInput.RecipientID,
	).Scan(&hasPendingInvitation); err != nil {
		return PrivateMessage{}, fmt.Errorf("check pending group invitation: %w", err)
	}

	if hasPendingInvitation {
		return PrivateMessage{}, &ValidationError{
			Message: "That person already has a pending invitation to this group.",
			Fields: map[string]string{
				"recipientId": "That person already has a pending invitation to this group.",
			},
		}
	}

	var hasPendingJoinRequest bool
	if err := s.db.QueryRowContext(
		ctx,
		`
			SELECT EXISTS (
				SELECT 1
				FROM group_join_requests
				WHERE
					group_id = $1
					AND requester_id = $2
					AND status = 'pending'
			)
		`,
		group.ID,
		normalizedInput.RecipientID,
	).Scan(&hasPendingJoinRequest); err != nil {
		return PrivateMessage{}, fmt.Errorf("check pending group join request: %w", err)
	}

	if hasPendingJoinRequest {
		return PrivateMessage{}, &ValidationError{
			Message: "That person already requested to join this group.",
			Fields: map[string]string{
				"recipientId": "That person already requested to join this group.",
			},
		}
	}

	body := buildGroupInviteMessage(group, normalizedInput.Note)
	return s.SendPrivateMessage(ctx, sender, normalizedInput.RecipientID, SendPrivateMessageInput{
		Body: body,
	})
}

func buildGroupInviteMessage(group Group, note string) string {
	values := url.Values{}
	values.Set("groupId", group.ID)
	values.Set("title", group.Title)
	if trimmedNote := strings.TrimSpace(note); trimmedNote != "" {
		values.Set("note", trimmedNote)
	}

	return groupInviteMessagePrefix + values.Encode()
}

func normalizeInviteUserToGroupInput(input InviteUserToGroupInput) (InviteUserToGroupInput, error) {
	normalized := InviteUserToGroupInput{
		RecipientID: strings.TrimSpace(input.RecipientID),
		Note:        strings.TrimSpace(input.Note),
	}

	fieldErrors := make(map[string]string)
	if normalized.RecipientID == "" {
		fieldErrors["recipientId"] = "Choose who should receive the invite."
	}
	if len(normalized.Note) > maxGroupInviteNoteLength {
		fieldErrors["note"] = "Invite notes must be 500 characters or fewer."
	}

	if len(fieldErrors) > 0 {
		return InviteUserToGroupInput{}, &ValidationError{
			Message: "Please correct the invitation details.",
			Fields:  fieldErrors,
		}
	}

	return normalized, nil
}
