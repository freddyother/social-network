package social

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"social-network/backend/internal/auth"
)

const maxCommentBodyLength = 1000

type CreateCommentInput struct {
	Body            string
	ParentCommentID string
}

type Comment struct {
	ID              string     `json:"id"`
	Body            string     `json:"body"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	ParentCommentID string     `json:"parentCommentId,omitempty"`
	Depth           int        `json:"depth"`
	Author          PostAuthor `json:"author"`
	Replies         []Comment  `json:"replies"`
}

type Notification struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	EntityType string    `json:"entityType,omitempty"`
	EntityID   string    `json:"entityId,omitempty"`
	IsRead     bool      `json:"isRead"`
	CreatedAt  time.Time `json:"createdAt"`
}

type sqlReader interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type sqlExecutor interface {
	sqlReader
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type visiblePost struct {
	ID                      string
	Title                   string
	AuthorID                string
	AuthorProfileVisibility string
	Privacy                 string
	ViewerFollowsAuthor     bool
}

type userIdentity struct {
	ID        string
	FirstName string
	LastName  string
	Nickname  string
	Email     string
}

func (u userIdentity) DisplayName() string {
	return displayName(u.FirstName, u.LastName, u.Nickname, u.Email)
}

func (s Service) Comments(ctx context.Context, viewerID, postID string) ([]Comment, error) {
	if _, err := s.loadVisiblePost(ctx, s.db, viewerID, postID); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(
		ctx,
		`
			SELECT
				c.id,
				c.parent_comment_id,
				c.body,
				c.created_at,
				c.updated_at,
				u.id,
				u.first_name,
				u.last_name,
				u.nickname,
				u.profile_visibility
			FROM comments c
			INNER JOIN users u ON u.id = c.author_id
			WHERE c.post_id = $1
			ORDER BY c.created_at ASC, c.id ASC
		`,
		postID,
	)
	if err != nil {
		return nil, fmt.Errorf("query comments: %w", err)
	}
	defer rows.Close()

	ordered := make([]*Comment, 0)
	commentsByID := make(map[string]*Comment)

	for rows.Next() {
		var item Comment
		var parentCommentID sql.NullString
		var nickname sql.NullString
		if err := rows.Scan(
			&item.ID,
			&parentCommentID,
			&item.Body,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.Author.ID,
			&item.Author.FirstName,
			&item.Author.LastName,
			&nickname,
			&item.Author.ProfileVisibility,
		); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}

		item.ParentCommentID = nullStringValue(parentCommentID)
		item.Author.Nickname = nullStringValue(nickname)
		item.Replies = []Comment{}

		comment := item
		ordered = append(ordered, &comment)
		commentsByID[comment.ID] = &comment
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate comments: %w", err)
	}

	topLevel := make([]*Comment, 0)
	for _, comment := range ordered {
		if comment.ParentCommentID == "" {
			comment.Depth = 0
			topLevel = append(topLevel, comment)
			continue
		}

		parent := commentsByID[comment.ParentCommentID]
		if parent == nil {
			continue
		}

		comment.Depth = parent.Depth + 1
		if comment.Depth > 1 {
			continue
		}

		parent.Replies = append(parent.Replies, *comment)
	}

	result := make([]Comment, 0, len(topLevel))
	for _, comment := range topLevel {
		result = append(result, *comment)
	}

	return result, nil
}

func (s Service) CreateComment(ctx context.Context, author auth.User, postID string, input CreateCommentInput) (Comment, error) {
	normalizedInput, err := normalizeCreateCommentInput(input)
	if err != nil {
		return Comment{}, err
	}

	deliveries := make([]notificationDelivery, 0, 2)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Comment{}, fmt.Errorf("begin create comment transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	post, err := s.loadVisiblePost(ctx, tx, author.ID, postID)
	if err != nil {
		return Comment{}, err
	}

	commentID, err := newToken(16)
	if err != nil {
		return Comment{}, fmt.Errorf("generate comment id: %w", err)
	}

	parentCommentID := any(nil)
	depth := 0
	replyTargetAuthorID := ""

	if normalizedInput.ParentCommentID != "" {
		parentCommentID = normalizedInput.ParentCommentID
		depth = 1

		var existingParentCommentID sql.NullString
		if err = tx.QueryRowContext(
			ctx,
			`
				SELECT author_id, parent_comment_id
				FROM comments
				WHERE id = $1 AND post_id = $2
			`,
			normalizedInput.ParentCommentID,
			postID,
		).Scan(&replyTargetAuthorID, &existingParentCommentID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return Comment{}, ErrNotFound
			}

			return Comment{}, fmt.Errorf("load parent comment: %w", err)
		}

		if existingParentCommentID.Valid {
			return Comment{}, &ValidationError{
				Message: "Replies are currently limited to two levels.",
				Fields: map[string]string{
					"parentCommentId": "Replies are currently limited to two levels.",
				},
			}
		}
	}

	var createdAt time.Time
	var updatedAt time.Time
	if err = tx.QueryRowContext(
		ctx,
		`
			INSERT INTO comments (id, post_id, author_id, parent_comment_id, body, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
			RETURNING created_at, updated_at
		`,
		commentID,
		postID,
		author.ID,
		parentCommentID,
		normalizedInput.Body,
	).Scan(&createdAt, &updatedAt); err != nil {
		return Comment{}, fmt.Errorf("insert comment: %w", err)
	}

	actorName := displayNameFromAuthUser(author)
	if normalizedInput.ParentCommentID == "" {
		if post.AuthorID != author.ID {
			createdNotification, notificationErr := s.insertNotification(
				ctx,
				tx,
				post.AuthorID,
				"post_comment",
				"New comment on your post",
				fmt.Sprintf("%s commented on \"%s\".", actorName, post.Title),
				"post",
				postID,
			)
			if notificationErr != nil {
				return Comment{}, notificationErr
			}
			deliveries = append(deliveries, notificationDelivery{
				UserID:       post.AuthorID,
				Notification: createdNotification,
			})
		}
	} else {
		if replyTargetAuthorID != "" && replyTargetAuthorID != author.ID {
			createdNotification, notificationErr := s.insertNotification(
				ctx,
				tx,
				replyTargetAuthorID,
				"comment_reply",
				"New reply to your comment",
				fmt.Sprintf("%s replied to you on \"%s\".", actorName, post.Title),
				"post",
				postID,
			)
			if notificationErr != nil {
				return Comment{}, notificationErr
			}
			deliveries = append(deliveries, notificationDelivery{
				UserID:       replyTargetAuthorID,
				Notification: createdNotification,
			})
		}

		if post.AuthorID != author.ID && post.AuthorID != replyTargetAuthorID {
			createdNotification, notificationErr := s.insertNotification(
				ctx,
				tx,
				post.AuthorID,
				"post_comment",
				"New comment on your post",
				fmt.Sprintf("%s commented on \"%s\".", actorName, post.Title),
				"post",
				postID,
			)
			if notificationErr != nil {
				return Comment{}, notificationErr
			}
			deliveries = append(deliveries, notificationDelivery{
				UserID:       post.AuthorID,
				Notification: createdNotification,
			})
		}
	}

	if err = tx.Commit(); err != nil {
		return Comment{}, fmt.Errorf("commit create comment transaction: %w", err)
	}

	comment := Comment{
		ID:              commentID,
		Body:            normalizedInput.Body,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
		ParentCommentID: normalizedInput.ParentCommentID,
		Depth:           depth,
		Author: PostAuthor{
			ID:                author.ID,
			FirstName:         author.FirstName,
			LastName:          author.LastName,
			Nickname:          author.Nickname,
			ProfileVisibility: author.ProfileVisibility,
		},
		Replies: []Comment{},
	}

	eventType := "comment.created"
	if comment.Depth > 0 {
		eventType = "comment.reply.created"
	}

	s.publisher.PublishToPost(postID, eventType, CommentEvent{
		PostID:  postID,
		Comment: comment,
	})

	for _, delivery := range deliveries {
		if delivery.Notification.ID == "" {
			continue
		}

		s.publisher.PublishToUser(delivery.UserID, "notification.created", NotificationEvent{
			Notification: delivery.Notification,
		})
	}

	return comment, nil
}

func (s Service) Notifications(ctx context.Context, userID string) ([]Notification, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`
			SELECT id, type, title, body, entity_type, entity_id, is_read, created_at
			FROM notifications
			WHERE user_id = $1
			ORDER BY created_at DESC
			LIMIT 100
		`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query notifications: %w", err)
	}
	defer rows.Close()

	notifications := make([]Notification, 0)
	for rows.Next() {
		var item Notification
		var entityType sql.NullString
		var entityID sql.NullString
		if err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.Title,
			&item.Body,
			&entityType,
			&entityID,
			&item.IsRead,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}

		item.EntityType = nullStringValue(entityType)
		item.EntityID = nullStringValue(entityID)
		notifications = append(notifications, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notifications: %w", err)
	}

	return notifications, nil
}

func (s Service) MarkNotificationRead(ctx context.Context, userID, notificationID string) error {
	result, err := s.db.ExecContext(
		ctx,
		`
			UPDATE notifications
			SET is_read = TRUE
			WHERE id = $1 AND user_id = $2
		`,
		notificationID,
		userID,
	)
	if err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count notification rows: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	s.publisher.PublishToUser(userID, "notification.read", NotificationReadEvent{
		NotificationID: notificationID,
	})

	return nil
}

func (s Service) loadCommentCounts(ctx context.Context, postIDs []string) (map[string]int, error) {
	countsByPostID := make(map[string]int, len(postIDs))
	if len(postIDs) == 0 {
		return countsByPostID, nil
	}

	rows, err := s.db.QueryContext(
		ctx,
		`
			SELECT post_id, COUNT(*)::INT
			FROM comments
			WHERE post_id = ANY($1)
			GROUP BY post_id
		`,
		pq.Array(postIDs),
	)
	if err != nil {
		return nil, fmt.Errorf("query comment counts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var postID string
		var count int
		if err := rows.Scan(&postID, &count); err != nil {
			return nil, fmt.Errorf("scan comment count: %w", err)
		}

		countsByPostID[postID] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate comment counts: %w", err)
	}

	return countsByPostID, nil
}

func (s Service) loadVisiblePost(ctx context.Context, reader sqlReader, viewerID, postID string) (visiblePost, error) {
	var post visiblePost
	if err := reader.QueryRowContext(
		ctx,
		`
			SELECT
				p.id,
				p.title,
				p.author_id,
				u.profile_visibility,
				p.privacy,
				EXISTS (
					SELECT 1
					FROM followers f
					WHERE f.follower_id = $1 AND f.followee_id = p.author_id
				)
			FROM posts p
			INNER JOIN users u ON u.id = p.author_id
			WHERE p.id = $2
		`,
		viewerID,
		postID,
	).Scan(
		&post.ID,
		&post.Title,
		&post.AuthorID,
		&post.AuthorProfileVisibility,
		&post.Privacy,
		&post.ViewerFollowsAuthor,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return visiblePost{}, ErrNotFound
		}

		return visiblePost{}, fmt.Errorf("load post visibility: %w", err)
	}

	if viewerID == post.AuthorID {
		return post, nil
	}

	switch {
	case post.AuthorProfileVisibility == "public" && post.Privacy == "public":
		return post, nil
	case post.AuthorProfileVisibility == "public" && post.Privacy == "followers" && post.ViewerFollowsAuthor:
		return post, nil
	case post.AuthorProfileVisibility == "private" && post.ViewerFollowsAuthor:
		return post, nil
	default:
		return visiblePost{}, ErrForbidden
	}
}

func (s Service) loadUserIdentity(ctx context.Context, reader sqlReader, userID string) (userIdentity, error) {
	var user userIdentity
	var nickname sql.NullString
	if err := reader.QueryRowContext(
		ctx,
		`
			SELECT id, first_name, last_name, nickname, email
			FROM users
			WHERE id = $1
		`,
		userID,
	).Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&nickname,
		&user.Email,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return userIdentity{}, ErrNotFound
		}

		return userIdentity{}, fmt.Errorf("load user identity: %w", err)
	}

	user.Nickname = nullStringValue(nickname)
	return user, nil
}

func (s Service) insertNotification(ctx context.Context, executor sqlExecutor, userID, notificationType, title, body, entityType, entityID string) (Notification, error) {
	if strings.TrimSpace(userID) == "" {
		return Notification{}, nil
	}

	notificationID, err := newToken(16)
	if err != nil {
		return Notification{}, fmt.Errorf("generate notification id: %w", err)
	}

	var entityTypeValue any
	if trimmed := strings.TrimSpace(entityType); trimmed != "" {
		entityTypeValue = trimmed
	}

	var entityIDValue any
	if trimmed := strings.TrimSpace(entityID); trimmed != "" {
		entityIDValue = trimmed
	}

	var createdAt time.Time
	if err := executor.QueryRowContext(
		ctx,
		`
			INSERT INTO notifications (id, user_id, type, title, body, entity_type, entity_id, is_read, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, FALSE, NOW())
			RETURNING created_at
		`,
		notificationID,
		userID,
		notificationType,
		title,
		body,
		entityTypeValue,
		entityIDValue,
	).Scan(&createdAt); err != nil {
		return Notification{}, fmt.Errorf("insert notification: %w", err)
	}

	return Notification{
		ID:         notificationID,
		Type:       notificationType,
		Title:      title,
		Body:       body,
		EntityType: strings.TrimSpace(entityType),
		EntityID:   strings.TrimSpace(entityID),
		IsRead:     false,
		CreatedAt:  createdAt,
	}, nil
}

func normalizeCreateCommentInput(input CreateCommentInput) (CreateCommentInput, error) {
	normalized := CreateCommentInput{
		Body:            strings.TrimSpace(input.Body),
		ParentCommentID: strings.TrimSpace(input.ParentCommentID),
	}

	fieldErrors := make(map[string]string)
	if normalized.Body == "" {
		fieldErrors["body"] = "Comment text is required."
	} else if len(normalized.Body) > maxCommentBodyLength {
		fieldErrors["body"] = "Comments must be 1000 characters or fewer."
	}

	if len(fieldErrors) > 0 {
		return CreateCommentInput{}, &ValidationError{
			Message: "Please correct the comment details.",
			Fields:  fieldErrors,
		}
	}

	return normalized, nil
}

func displayName(firstName, lastName, nickname, fallback string) string {
	if trimmed := strings.TrimSpace(nickname); trimmed != "" {
		return trimmed
	}

	fullName := strings.TrimSpace(strings.TrimSpace(firstName) + " " + strings.TrimSpace(lastName))
	if fullName != "" {
		return fullName
	}

	return strings.TrimSpace(fallback)
}

func displayNameFromAuthUser(user auth.User) string {
	return displayName(user.FirstName, user.LastName, user.Nickname, user.Email)
}
