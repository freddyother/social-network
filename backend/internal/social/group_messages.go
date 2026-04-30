package social

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"social-network/backend/internal/auth"
)

const maxGroupMessageBodyLength = 2000

type GroupMessage struct {
	ID       string    `json:"id"`
	GroupID  string    `json:"groupId"`
	SenderID string    `json:"senderId"`
	Body     string    `json:"body"`
	SentAt   time.Time `json:"sentAt"`
	Sender   GroupUser `json:"sender"`
}

type SendGroupMessageInput struct {
	Body string
}

type GroupMessageEvent struct {
	GroupID string       `json:"groupId"`
	Message GroupMessage `json:"message"`
}

func (s Service) GroupMessages(ctx context.Context, viewerID, groupID string) ([]GroupMessage, error) {
	normalizedGroupID, err := s.requireGroupMembership(ctx, s.db, viewerID, groupID)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(
		ctx,
		`
			WITH recent AS (
				SELECT
					gm.id,
					gm.group_id,
					gm.sender_id,
					gm.body,
					gm.sent_at,
					u.id AS user_id,
					u.first_name,
					u.last_name,
					u.nickname,
					u.avatar_url
				FROM group_messages gm
				INNER JOIN users u ON u.id = gm.sender_id
				WHERE gm.group_id = $1
				ORDER BY gm.sent_at DESC, gm.id DESC
				LIMIT 100
			)
			SELECT *
			FROM recent
			ORDER BY sent_at ASC, id ASC
		`,
		normalizedGroupID,
	)
	if err != nil {
		return nil, fmt.Errorf("query group messages: %w", err)
	}
	defer rows.Close()

	return s.loadGroupMessagesFromRows(rows, "group messages")
}

func (s Service) SendGroupMessage(ctx context.Context, sender auth.User, groupID string, input SendGroupMessageInput) (GroupMessage, error) {
	normalizedInput, err := normalizeSendGroupMessageInput(input)
	if err != nil {
		return GroupMessage{}, err
	}

	messageID, err := newToken(16)
	if err != nil {
		return GroupMessage{}, fmt.Errorf("generate group message id: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return GroupMessage{}, fmt.Errorf("begin group message transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	normalizedGroupID, err := s.requireGroupMembership(ctx, tx, sender.ID, groupID)
	if err != nil {
		return GroupMessage{}, err
	}

	var sentAt time.Time
	if err = tx.QueryRowContext(
		ctx,
		`
			INSERT INTO group_messages (id, group_id, sender_id, body)
			VALUES ($1, $2, $3, $4)
			RETURNING sent_at
		`,
		messageID,
		normalizedGroupID,
		sender.ID,
		normalizedInput.Body,
	).Scan(&sentAt); err != nil {
		return GroupMessage{}, fmt.Errorf("insert group message: %w", err)
	}

	memberIDs, err := s.groupMemberIDs(ctx, tx, normalizedGroupID)
	if err != nil {
		return GroupMessage{}, err
	}

	if err = tx.Commit(); err != nil {
		return GroupMessage{}, fmt.Errorf("commit group message transaction: %w", err)
	}

	message := GroupMessage{
		ID:       messageID,
		GroupID:  normalizedGroupID,
		SenderID: sender.ID,
		Body:     normalizedInput.Body,
		SentAt:   sentAt,
		Sender: GroupUser{
			ID:        sender.ID,
			FirstName: sender.FirstName,
			LastName:  sender.LastName,
			Nickname:  sender.Nickname,
			AvatarURL: sender.AvatarURL,
		},
	}

	event := GroupMessageEvent{
		GroupID: normalizedGroupID,
		Message: message,
	}
	for _, memberID := range memberIDs {
		s.publisher.PublishToUser(memberID, "group.message.created", event)
	}

	return message, nil
}

func (s Service) groupMemberIDs(ctx context.Context, reader sqlReader, groupID string) ([]string, error) {
	rows, err := reader.QueryContext(
		ctx,
		`
			SELECT user_id
			FROM group_memberships
			WHERE group_id = $1
		`,
		groupID,
	)
	if err != nil {
		return nil, fmt.Errorf("query group members: %w", err)
	}
	defer rows.Close()

	memberIDs := make([]string, 0)
	for rows.Next() {
		var memberID string
		if err := rows.Scan(&memberID); err != nil {
			return nil, fmt.Errorf("scan group member: %w", err)
		}

		memberIDs = append(memberIDs, memberID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate group members: %w", err)
	}

	return memberIDs, nil
}

func (s Service) loadGroupMessagesFromRows(rows *sql.Rows, operation string) ([]GroupMessage, error) {
	messages := make([]GroupMessage, 0)

	for rows.Next() {
		message, err := s.scanGroupMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan %s group message: %w", operation, err)
		}

		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s: %w", operation, err)
	}

	return messages, nil
}

func (s Service) scanGroupMessage(scanner interface{ Scan(dest ...any) error }) (GroupMessage, error) {
	var (
		message         GroupMessage
		senderNickname  sql.NullString
		senderAvatarURL sql.NullString
	)
	if err := scanner.Scan(
		&message.ID,
		&message.GroupID,
		&message.SenderID,
		&message.Body,
		&message.SentAt,
		&message.Sender.ID,
		&message.Sender.FirstName,
		&message.Sender.LastName,
		&senderNickname,
		&senderAvatarURL,
	); err != nil {
		return GroupMessage{}, err
	}

	message.Sender.Nickname = nullStringValue(senderNickname)
	if senderAvatarURL.Valid {
		message.Sender.AvatarURL = s.publicURL(senderAvatarURL.String)
	}

	return message, nil
}

func normalizeSendGroupMessageInput(input SendGroupMessageInput) (SendGroupMessageInput, error) {
	normalized := SendGroupMessageInput{
		Body: strings.TrimSpace(input.Body),
	}

	fieldErrors := make(map[string]string)
	if normalized.Body == "" {
		fieldErrors["body"] = "Write a message before sending."
	} else if len(normalized.Body) > maxGroupMessageBodyLength {
		fieldErrors["body"] = "Messages must be 2000 characters or fewer."
	}

	if len(fieldErrors) > 0 {
		return SendGroupMessageInput{}, &ValidationError{
			Message: "Please correct the message details.",
			Fields:  fieldErrors,
		}
	}

	return normalized, nil
}
