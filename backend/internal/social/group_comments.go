package social

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"social-network/backend/internal/auth"
)

const maxGroupCommentBodyLength = 1000

type GroupComment struct {
	ID             string    `json:"id"`
	GroupID        string    `json:"groupId"`
	GroupPostID    string    `json:"groupPostId"`
	Body           string    `json:"body"`
	CreatedAt      time.Time `json:"createdAt"`
	ReactionsCount int       `json:"reactionsCount"`
	ViewerReaction string    `json:"viewerReaction,omitempty"`
	Author         GroupUser `json:"author"`
}

type CreateGroupCommentInput struct {
	Body string
}

type groupPostState struct {
	ID       string
	GroupID  string
	AuthorID string
}

func (s Service) GroupComments(ctx context.Context, viewerID, groupID, groupPostID string) ([]GroupComment, error) {
	post, err := s.loadGroupPostState(ctx, s.db, viewerID, groupID, groupPostID)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(
		ctx,
		`
			SELECT
				gc.id,
				gp.group_id,
				gc.group_post_id,
				gc.body,
				gc.created_at,
				u.id,
				u.first_name,
				u.last_name,
				u.nickname,
				u.avatar_url
			FROM group_comments gc
			INNER JOIN group_posts gp ON gp.id = gc.group_post_id
			INNER JOIN users u ON u.id = gc.author_id
			WHERE gc.group_post_id = $1
			ORDER BY gc.created_at ASC, gc.id ASC
		`,
		post.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("query group comments: %w", err)
	}
	defer rows.Close()

	return s.loadGroupCommentsFromRows(ctx, rows, "group comments", viewerID)
}

func (s Service) CreateGroupComment(ctx context.Context, author auth.User, groupID, groupPostID string, input CreateGroupCommentInput) (GroupComment, error) {
	normalizedInput, err := normalizeCreateGroupCommentInput(input)
	if err != nil {
		return GroupComment{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return GroupComment{}, fmt.Errorf("begin group comment transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	post, err := s.loadGroupPostState(ctx, tx, author.ID, groupID, groupPostID)
	if err != nil {
		return GroupComment{}, err
	}

	commentID, err := newToken(16)
	if err != nil {
		return GroupComment{}, fmt.Errorf("generate group comment id: %w", err)
	}

	var createdAt time.Time
	if err = tx.QueryRowContext(
		ctx,
		`
			INSERT INTO group_comments (id, group_post_id, author_id, body)
			VALUES ($1, $2, $3, $4)
			RETURNING created_at
		`,
		commentID,
		post.ID,
		author.ID,
		normalizedInput.Body,
	).Scan(&createdAt); err != nil {
		return GroupComment{}, fmt.Errorf("insert group comment: %w", err)
	}

	var createdNotification Notification
	if post.AuthorID != author.ID {
		createdNotification, err = s.insertNotification(
			ctx,
			tx,
			post.AuthorID,
			"group_post_comment",
			"New comment in your group post",
			fmt.Sprintf("%s commented inside one of your group discussions.", displayNameFromAuthUser(author)),
			"group",
			post.GroupID,
		)
		if err != nil {
			return GroupComment{}, err
		}
	}

	if err = tx.Commit(); err != nil {
		return GroupComment{}, fmt.Errorf("commit group comment transaction: %w", err)
	}

	if createdNotification.ID != "" {
		s.publisher.PublishToUser(post.AuthorID, "notification.created", NotificationEvent{
			Notification: createdNotification,
		})
	}

	return GroupComment{
		ID:          commentID,
		GroupID:     post.GroupID,
		GroupPostID: post.ID,
		Body:        normalizedInput.Body,
		CreatedAt:   createdAt,
		Author: GroupUser{
			ID:        author.ID,
			FirstName: author.FirstName,
			LastName:  author.LastName,
			Nickname:  author.Nickname,
			AvatarURL: author.AvatarURL,
		},
	}, nil
}

func (s Service) loadGroupCommentsFromRows(ctx context.Context, rows *sql.Rows, operation, viewerID string) ([]GroupComment, error) {
	comments := make([]GroupComment, 0)
	commentIDs := make([]string, 0)

	for rows.Next() {
		var (
			comment         GroupComment
			authorNickname  sql.NullString
			authorAvatarURL sql.NullString
		)
		if err := rows.Scan(
			&comment.ID,
			&comment.GroupID,
			&comment.GroupPostID,
			&comment.Body,
			&comment.CreatedAt,
			&comment.Author.ID,
			&comment.Author.FirstName,
			&comment.Author.LastName,
			&authorNickname,
			&authorAvatarURL,
		); err != nil {
			return nil, fmt.Errorf("scan %s group comment: %w", operation, err)
		}

		comment.Author.Nickname = nullStringValue(authorNickname)
		if authorAvatarURL.Valid {
			comment.Author.AvatarURL = s.publicURL(authorAvatarURL.String)
		}

		comments = append(comments, comment)
		commentIDs = append(commentIDs, comment.ID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s group comments: %w", operation, err)
	}

	reactionsByCommentID, err := s.loadReactionSummaries(ctx, groupCommentReactionTarget, viewerID, commentIDs)
	if err != nil {
		return nil, err
	}

	for index := range comments {
		reactionSummary := reactionsByCommentID[comments[index].ID]
		comments[index].ReactionsCount = reactionSummary.Count
		comments[index].ViewerReaction = reactionSummary.ViewerReaction
	}

	return comments, nil
}

func (s Service) loadGroupPostState(ctx context.Context, reader sqlReader, viewerID, groupID, groupPostID string) (groupPostState, error) {
	normalizedGroupID, err := s.requireGroupMembership(ctx, reader, viewerID, groupID)
	if err != nil {
		return groupPostState{}, err
	}

	normalizedGroupPostID := strings.TrimSpace(groupPostID)
	if normalizedGroupPostID == "" {
		return groupPostState{}, &ValidationError{
			Message: "Choose a group post first.",
			Fields: map[string]string{
				"groupPostId": "Choose a group post first.",
			},
		}
	}

	var post groupPostState
	if err := reader.QueryRowContext(
		ctx,
		`
			SELECT id, group_id, author_id
			FROM group_posts
			WHERE id = $1 AND group_id = $2
		`,
		normalizedGroupPostID,
		normalizedGroupID,
	).Scan(&post.ID, &post.GroupID, &post.AuthorID); err != nil {
		if err == sql.ErrNoRows {
			return groupPostState{}, ErrNotFound
		}

		return groupPostState{}, fmt.Errorf("load group post: %w", err)
	}

	return post, nil
}

func normalizeCreateGroupCommentInput(input CreateGroupCommentInput) (CreateGroupCommentInput, error) {
	body := strings.TrimSpace(input.Body)

	fieldErrors := make(map[string]string)
	if body == "" {
		fieldErrors["body"] = "Comment text is required."
	} else if len(body) > maxGroupCommentBodyLength {
		fieldErrors["body"] = "Comments must be 1000 characters or fewer."
	}

	if len(fieldErrors) > 0 {
		return CreateGroupCommentInput{}, &ValidationError{
			Message: "Please correct the comment details.",
			Fields:  fieldErrors,
		}
	}

	return CreateGroupCommentInput{Body: body}, nil
}
