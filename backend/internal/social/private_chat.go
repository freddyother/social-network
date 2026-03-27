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

const maxPrivateMessageLength = 2000

type ChatUser struct {
	ID                string `json:"id"`
	FirstName         string `json:"firstName"`
	LastName          string `json:"lastName"`
	Nickname          string `json:"nickname,omitempty"`
	AvatarURL         string `json:"avatarUrl,omitempty"`
	ProfileVisibility string `json:"profileVisibility"`
}

type PrivateMessage struct {
	ID          string     `json:"id"`
	SenderID    string     `json:"senderId"`
	RecipientID string     `json:"recipientId"`
	Body        string     `json:"body"`
	SentAt      time.Time  `json:"sentAt"`
	ReadAt      *time.Time `json:"readAt,omitempty"`
}

type ConversationSummary struct {
	User        ChatUser       `json:"user"`
	LastMessage PrivateMessage `json:"lastMessage"`
	UnreadCount int            `json:"unreadCount"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

type ConversationThread struct {
	User     ChatUser         `json:"user"`
	Messages []PrivateMessage `json:"messages"`
}

type SendPrivateMessageInput struct {
	Body string
}

type ConversationReadResult struct {
	ConversationUserID string     `json:"conversationUserId"`
	MessageIDs         []string   `json:"messageIds"`
	ReadAt             *time.Time `json:"readAt,omitempty"`
}

type ChatMessageEvent struct {
	ConversationUserID string         `json:"conversationUserId"`
	Message            PrivateMessage `json:"message"`
}

type ConversationReadEvent struct {
	ConversationUserID string    `json:"conversationUserId"`
	ReaderID           string    `json:"readerId"`
	MessageIDs         []string  `json:"messageIds"`
	ReadAt             time.Time `json:"readAt"`
}

func (s Service) Conversations(ctx context.Context, userID string) ([]ConversationSummary, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`
			WITH ranked AS (
				SELECT
					CASE
						WHEN pm.sender_id = $1 THEN pm.recipient_id
						ELSE pm.sender_id
					END AS partner_id,
					pm.id,
					pm.sender_id,
					pm.recipient_id,
					pm.body,
					pm.sent_at,
					pm.read_at,
					ROW_NUMBER() OVER (
						PARTITION BY CASE
							WHEN pm.sender_id = $1 THEN pm.recipient_id
							ELSE pm.sender_id
						END
						ORDER BY pm.sent_at DESC, pm.id DESC
					) AS row_number
				FROM private_messages pm
				WHERE pm.sender_id = $1 OR pm.recipient_id = $1
			),
			unread AS (
				SELECT sender_id AS partner_id, COUNT(*) AS unread_count
				FROM private_messages
				WHERE recipient_id = $1 AND read_at IS NULL
				GROUP BY sender_id
			)
			SELECT
				u.id,
				u.first_name,
				u.last_name,
				u.nickname,
				u.avatar_url,
				u.profile_visibility,
				ranked.id,
				ranked.sender_id,
				ranked.recipient_id,
				ranked.body,
				ranked.sent_at,
				ranked.read_at,
				COALESCE(unread.unread_count, 0) AS unread_count
			FROM ranked
			INNER JOIN users u ON u.id = ranked.partner_id
			LEFT JOIN unread ON unread.partner_id = ranked.partner_id
			WHERE ranked.row_number = 1
			ORDER BY ranked.sent_at DESC, ranked.id DESC
		`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query conversations: %w", err)
	}
	defer rows.Close()

	conversations := make([]ConversationSummary, 0)
	for rows.Next() {
		conversation, err := s.scanConversationSummary(rows)
		if err != nil {
			return nil, err
		}

		conversations = append(conversations, conversation)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate conversations: %w", err)
	}

	return conversations, nil
}

func (s Service) Conversation(ctx context.Context, viewerID, partnerID string) (ConversationThread, error) {
	normalizedPartnerID, err := normalizeConversationPartner(viewerID, partnerID)
	if err != nil {
		return ConversationThread{}, err
	}

	user, err := s.loadChatUser(ctx, normalizedPartnerID)
	if err != nil {
		return ConversationThread{}, err
	}

	rows, err := s.db.QueryContext(
		ctx,
		`
			SELECT id, sender_id, recipient_id, body, sent_at, read_at
			FROM private_messages
			WHERE (sender_id = $1 AND recipient_id = $2)
			   OR (sender_id = $2 AND recipient_id = $1)
			ORDER BY sent_at ASC, id ASC
		`,
		viewerID,
		normalizedPartnerID,
	)
	if err != nil {
		return ConversationThread{}, fmt.Errorf("query conversation messages: %w", err)
	}
	defer rows.Close()

	messages := make([]PrivateMessage, 0)
	for rows.Next() {
		message, err := scanPrivateMessage(rows)
		if err != nil {
			return ConversationThread{}, err
		}

		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return ConversationThread{}, fmt.Errorf("iterate conversation messages: %w", err)
	}

	return ConversationThread{
		User:     user,
		Messages: messages,
	}, nil
}

func (s Service) SendPrivateMessage(ctx context.Context, sender auth.User, partnerID string, input SendPrivateMessageInput) (PrivateMessage, error) {
	normalizedPartnerID, err := normalizeConversationPartner(sender.ID, partnerID)
	if err != nil {
		return PrivateMessage{}, err
	}

	normalizedInput, err := normalizeSendPrivateMessageInput(input)
	if err != nil {
		return PrivateMessage{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PrivateMessage{}, fmt.Errorf("begin private message transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = s.loadChatUserWithReader(ctx, tx, normalizedPartnerID); err != nil {
		return PrivateMessage{}, err
	}

	messageID, err := newToken(16)
	if err != nil {
		return PrivateMessage{}, fmt.Errorf("generate private message id: %w", err)
	}

	var sentAt time.Time
	if err = tx.QueryRowContext(
		ctx,
		`
			INSERT INTO private_messages (id, sender_id, recipient_id, body)
			VALUES ($1, $2, $3, $4)
			RETURNING sent_at
		`,
		messageID,
		sender.ID,
		normalizedPartnerID,
		normalizedInput.Body,
	).Scan(&sentAt); err != nil {
		return PrivateMessage{}, fmt.Errorf("insert private message: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return PrivateMessage{}, fmt.Errorf("commit private message transaction: %w", err)
	}

	message := PrivateMessage{
		ID:          messageID,
		SenderID:    sender.ID,
		RecipientID: normalizedPartnerID,
		Body:        normalizedInput.Body,
		SentAt:      sentAt,
	}

	s.publisher.PublishToUser(sender.ID, "chat.message.created", ChatMessageEvent{
		ConversationUserID: normalizedPartnerID,
		Message:            message,
	})
	s.publisher.PublishToUser(normalizedPartnerID, "chat.message.created", ChatMessageEvent{
		ConversationUserID: sender.ID,
		Message:            message,
	})

	return message, nil
}

func (s Service) MarkConversationRead(ctx context.Context, viewerID, partnerID string) (ConversationReadResult, error) {
	normalizedPartnerID, err := normalizeConversationPartner(viewerID, partnerID)
	if err != nil {
		return ConversationReadResult{}, err
	}

	if _, err := s.loadChatUser(ctx, normalizedPartnerID); err != nil {
		return ConversationReadResult{}, err
	}

	readAt := time.Now().UTC()
	rows, err := s.db.QueryContext(
		ctx,
		`
			UPDATE private_messages
			SET read_at = $3
			WHERE sender_id = $1
			  AND recipient_id = $2
			  AND read_at IS NULL
			RETURNING id
		`,
		normalizedPartnerID,
		viewerID,
		readAt,
	)
	if err != nil {
		return ConversationReadResult{}, fmt.Errorf("mark conversation read: %w", err)
	}
	defer rows.Close()

	messageIDs := make([]string, 0)
	for rows.Next() {
		var messageID string
		if err := rows.Scan(&messageID); err != nil {
			return ConversationReadResult{}, fmt.Errorf("scan read private message: %w", err)
		}

		messageIDs = append(messageIDs, messageID)
	}

	if err := rows.Err(); err != nil {
		return ConversationReadResult{}, fmt.Errorf("iterate read private messages: %w", err)
	}

	result := ConversationReadResult{
		ConversationUserID: normalizedPartnerID,
		MessageIDs:         messageIDs,
	}

	if len(messageIDs) == 0 {
		return result, nil
	}

	result.ReadAt = &readAt

	s.publisher.PublishToUser(viewerID, "chat.conversation.read", ConversationReadEvent{
		ConversationUserID: normalizedPartnerID,
		ReaderID:           viewerID,
		MessageIDs:         messageIDs,
		ReadAt:             readAt,
	})
	s.publisher.PublishToUser(normalizedPartnerID, "chat.conversation.read", ConversationReadEvent{
		ConversationUserID: viewerID,
		ReaderID:           viewerID,
		MessageIDs:         messageIDs,
		ReadAt:             readAt,
	})

	return result, nil
}

func (s Service) loadChatUser(ctx context.Context, userID string) (ChatUser, error) {
	return s.loadChatUserWithReader(ctx, s.db, userID)
}

func (s Service) loadChatUserWithReader(ctx context.Context, reader sqlReader, userID string) (ChatUser, error) {
	row := reader.QueryRowContext(
		ctx,
		`
			SELECT id, first_name, last_name, nickname, avatar_url, profile_visibility
			FROM users
			WHERE id = $1
		`,
		userID,
	)

	var user ChatUser
	var nickname sql.NullString
	var avatarURL sql.NullString
	if err := row.Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&nickname,
		&avatarURL,
		&user.ProfileVisibility,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ChatUser{}, ErrNotFound
		}

		return ChatUser{}, fmt.Errorf("load chat user: %w", err)
	}

	user.Nickname = nullStringValue(nickname)
	if avatarURL.Valid {
		user.AvatarURL = s.publicURL(avatarURL.String)
	}

	return user, nil
}

func (s Service) scanConversationSummary(scanner interface{ Scan(dest ...any) error }) (ConversationSummary, error) {
	var summary ConversationSummary
	var nickname sql.NullString
	var avatarURL sql.NullString
	var readAt sql.NullTime
	if err := scanner.Scan(
		&summary.User.ID,
		&summary.User.FirstName,
		&summary.User.LastName,
		&nickname,
		&avatarURL,
		&summary.User.ProfileVisibility,
		&summary.LastMessage.ID,
		&summary.LastMessage.SenderID,
		&summary.LastMessage.RecipientID,
		&summary.LastMessage.Body,
		&summary.LastMessage.SentAt,
		&readAt,
		&summary.UnreadCount,
	); err != nil {
		return ConversationSummary{}, fmt.Errorf("scan conversation summary: %w", err)
	}

	summary.User.Nickname = nullStringValue(nickname)
	if avatarURL.Valid {
		summary.User.AvatarURL = s.publicURL(avatarURL.String)
	}
	summary.LastMessage.ReadAt = nullTimePointer(readAt)
	summary.UpdatedAt = summary.LastMessage.SentAt

	return summary, nil
}

func scanPrivateMessage(scanner interface{ Scan(dest ...any) error }) (PrivateMessage, error) {
	var message PrivateMessage
	var readAt sql.NullTime
	if err := scanner.Scan(
		&message.ID,
		&message.SenderID,
		&message.RecipientID,
		&message.Body,
		&message.SentAt,
		&readAt,
	); err != nil {
		return PrivateMessage{}, fmt.Errorf("scan private message: %w", err)
	}

	message.ReadAt = nullTimePointer(readAt)
	return message, nil
}

func normalizeConversationPartner(viewerID, partnerID string) (string, error) {
	normalizedPartnerID := strings.TrimSpace(partnerID)
	if normalizedPartnerID == "" {
		return "", &ValidationError{
			Message: "Choose an account to open the conversation.",
			Fields: map[string]string{
				"userId": "Choose an account to open the conversation.",
			},
		}
	}

	if strings.TrimSpace(viewerID) == normalizedPartnerID {
		return "", &ValidationError{
			Message: "You cannot open a private chat with yourself.",
			Fields: map[string]string{
				"userId": "You cannot open a private chat with yourself.",
			},
		}
	}

	return normalizedPartnerID, nil
}

func normalizeSendPrivateMessageInput(input SendPrivateMessageInput) (SendPrivateMessageInput, error) {
	normalized := SendPrivateMessageInput{
		Body: strings.TrimSpace(input.Body),
	}

	fieldErrors := make(map[string]string)
	if normalized.Body == "" {
		fieldErrors["body"] = "Write a message before sending."
	} else if len(normalized.Body) > maxPrivateMessageLength {
		fieldErrors["body"] = "Messages must be 2000 characters or fewer."
	}

	if len(fieldErrors) > 0 {
		return SendPrivateMessageInput{}, &ValidationError{
			Message: "Please correct the message details.",
			Fields:  fieldErrors,
		}
	}

	return normalized, nil
}

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}

	parsed := value.Time
	return &parsed
}
