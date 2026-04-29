package social

import (
	"context"
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
