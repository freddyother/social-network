package social

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

const reactionLike = "like"

var (
	postReactionTarget = reactionTargetConfig{
		TargetType:   "post",
		Table:        "post_reactions",
		TargetColumn: "post_id",
	}
	commentReactionTarget = reactionTargetConfig{
		TargetType:   "comment",
		Table:        "comment_reactions",
		TargetColumn: "comment_id",
	}
	groupPostReactionTarget = reactionTargetConfig{
		TargetType:   "group_post",
		Table:        "group_post_reactions",
		TargetColumn: "group_post_id",
	}
	groupCommentReactionTarget = reactionTargetConfig{
		TargetType:   "group_comment",
		Table:        "group_comment_reactions",
		TargetColumn: "group_comment_id",
	}
)

type ReactionResult struct {
	TargetType     string         `json:"targetType"`
	TargetID       string         `json:"targetId"`
	ReactionsCount int            `json:"reactionsCount"`
	ViewerReaction string         `json:"viewerReaction,omitempty"`
	ReactionUsers  []ReactionUser `json:"reactionUsers"`
}

type ReactionUser struct {
	ID        string `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Nickname  string `json:"nickname,omitempty"`
	AvatarURL string `json:"avatarUrl,omitempty"`
}

type reactionSummary struct {
	Count          int
	ViewerReaction string
	Users          []ReactionUser
}

type reactionTargetConfig struct {
	TargetType   string
	Table        string
	TargetColumn string
}

type reactionNotification struct {
	RecipientID string
	Type        string
	Title       string
	Body        string
	EntityType  string
	EntityID    string
	Metadata    map[string]string
}

type commentReactionTargetState struct {
	Post      visiblePost
	CommentID string
	AuthorID  string
}

type groupCommentReactionTargetState struct {
	Post      groupPostState
	CommentID string
	AuthorID  string
}

func (s Service) SetPostReaction(ctx context.Context, userID, postID, reactionType string) (ReactionResult, error) {
	normalizedReaction, err := normalizeReactionType(reactionType)
	if err != nil {
		return ReactionResult{}, err
	}

	post, err := s.loadVisiblePost(ctx, s.db, userID, strings.TrimSpace(postID))
	if err != nil {
		return ReactionResult{}, err
	}

	createdNotification, err := s.createReactionWithNotification(
		ctx,
		postReactionTarget,
		post.ID,
		userID,
		normalizedReaction,
		reactionNotification{
			RecipientID: post.AuthorID,
			Type:        "post_reaction",
			Title:       "New reaction on your post",
			Body:        reactionPostBody(ctx, s, userID, post.Title),
			EntityType:  "post",
			EntityID:    post.ID,
			Metadata:    map[string]string{"postId": post.ID},
		},
	)
	if err != nil {
		return ReactionResult{}, err
	}

	s.publishReactionNotification(createdNotification)
	return s.loadReactionResult(ctx, postReactionTarget, userID, post.ID)
}

func (s Service) ClearPostReaction(ctx context.Context, userID, postID string) (ReactionResult, error) {
	post, err := s.loadVisiblePost(ctx, s.db, userID, strings.TrimSpace(postID))
	if err != nil {
		return ReactionResult{}, err
	}

	if err := s.deleteReaction(ctx, postReactionTarget, post.ID, userID); err != nil {
		return ReactionResult{}, err
	}

	return s.loadReactionResult(ctx, postReactionTarget, userID, post.ID)
}

func (s Service) SetCommentReaction(ctx context.Context, userID, postID, commentID, reactionType string) (ReactionResult, error) {
	normalizedReaction, err := normalizeReactionType(reactionType)
	if err != nil {
		return ReactionResult{}, err
	}

	target, err := s.loadCommentReactionTarget(ctx, userID, postID, commentID)
	if err != nil {
		return ReactionResult{}, err
	}

	createdNotification, err := s.createReactionWithNotification(
		ctx,
		commentReactionTarget,
		target.CommentID,
		userID,
		normalizedReaction,
		reactionNotification{
			RecipientID: target.AuthorID,
			Type:        "comment_reaction",
			Title:       "New reaction on your comment",
			Body:        reactionCommentBody(ctx, s, userID, target.Post.Title),
			EntityType:  "comment",
			EntityID:    target.CommentID,
			Metadata:    map[string]string{"postId": target.Post.ID, "commentId": target.CommentID},
		},
	)
	if err != nil {
		return ReactionResult{}, err
	}

	s.publishReactionNotification(createdNotification)
	return s.loadReactionResult(ctx, commentReactionTarget, userID, target.CommentID)
}

func (s Service) ClearCommentReaction(ctx context.Context, userID, postID, commentID string) (ReactionResult, error) {
	normalizedCommentID, err := s.requireCommentAccess(ctx, userID, postID, commentID)
	if err != nil {
		return ReactionResult{}, err
	}

	if err := s.deleteReaction(ctx, commentReactionTarget, normalizedCommentID, userID); err != nil {
		return ReactionResult{}, err
	}

	return s.loadReactionResult(ctx, commentReactionTarget, userID, normalizedCommentID)
}

func (s Service) SetGroupPostReaction(ctx context.Context, userID, groupID, groupPostID, reactionType string) (ReactionResult, error) {
	normalizedReaction, err := normalizeReactionType(reactionType)
	if err != nil {
		return ReactionResult{}, err
	}

	post, err := s.loadGroupPostState(ctx, s.db, userID, groupID, groupPostID)
	if err != nil {
		return ReactionResult{}, err
	}

	createdNotification, err := s.createReactionWithNotification(
		ctx,
		groupPostReactionTarget,
		post.ID,
		userID,
		normalizedReaction,
		reactionNotification{
			RecipientID: post.AuthorID,
			Type:        "group_post_reaction",
			Title:       "New reaction on your group post",
			Body:        reactionGroupPostBody(ctx, s, userID),
			EntityType:  "group_post",
			EntityID:    post.ID,
			Metadata:    map[string]string{"groupId": post.GroupID, "groupPostId": post.ID},
		},
	)
	if err != nil {
		return ReactionResult{}, err
	}

	s.publishReactionNotification(createdNotification)
	return s.loadReactionResult(ctx, groupPostReactionTarget, userID, post.ID)
}

func (s Service) ClearGroupPostReaction(ctx context.Context, userID, groupID, groupPostID string) (ReactionResult, error) {
	post, err := s.loadGroupPostState(ctx, s.db, userID, groupID, groupPostID)
	if err != nil {
		return ReactionResult{}, err
	}

	if err := s.deleteReaction(ctx, groupPostReactionTarget, post.ID, userID); err != nil {
		return ReactionResult{}, err
	}

	return s.loadReactionResult(ctx, groupPostReactionTarget, userID, post.ID)
}

func (s Service) SetGroupCommentReaction(ctx context.Context, userID, groupID, groupPostID, commentID, reactionType string) (ReactionResult, error) {
	normalizedReaction, err := normalizeReactionType(reactionType)
	if err != nil {
		return ReactionResult{}, err
	}

	target, err := s.loadGroupCommentReactionTarget(ctx, userID, groupID, groupPostID, commentID)
	if err != nil {
		return ReactionResult{}, err
	}

	createdNotification, err := s.createReactionWithNotification(
		ctx,
		groupCommentReactionTarget,
		target.CommentID,
		userID,
		normalizedReaction,
		reactionNotification{
			RecipientID: target.AuthorID,
			Type:        "group_comment_reaction",
			Title:       "New reaction on your group comment",
			Body:        reactionGroupCommentBody(ctx, s, userID),
			EntityType:  "group_comment",
			EntityID:    target.CommentID,
			Metadata:    map[string]string{"groupId": target.Post.GroupID, "groupPostId": target.Post.ID, "commentId": target.CommentID},
		},
	)
	if err != nil {
		return ReactionResult{}, err
	}

	s.publishReactionNotification(createdNotification)
	return s.loadReactionResult(ctx, groupCommentReactionTarget, userID, target.CommentID)
}

func (s Service) ClearGroupCommentReaction(ctx context.Context, userID, groupID, groupPostID, commentID string) (ReactionResult, error) {
	normalizedCommentID, err := s.requireGroupCommentAccess(ctx, userID, groupID, groupPostID, commentID)
	if err != nil {
		return ReactionResult{}, err
	}

	if err := s.deleteReaction(ctx, groupCommentReactionTarget, normalizedCommentID, userID); err != nil {
		return ReactionResult{}, err
	}

	return s.loadReactionResult(ctx, groupCommentReactionTarget, userID, normalizedCommentID)
}

func (s Service) requireCommentAccess(ctx context.Context, userID, postID, commentID string) (string, error) {
	target, err := s.loadCommentReactionTarget(ctx, userID, postID, commentID)
	if err != nil {
		return "", err
	}

	return target.CommentID, nil
}

func (s Service) loadCommentReactionTarget(ctx context.Context, userID, postID, commentID string) (commentReactionTargetState, error) {
	post, err := s.loadVisiblePost(ctx, s.db, userID, strings.TrimSpace(postID))
	if err != nil {
		return commentReactionTargetState{}, err
	}

	normalizedCommentID := strings.TrimSpace(commentID)
	if normalizedCommentID == "" {
		return commentReactionTargetState{}, &ValidationError{
			Message: "Choose a comment first.",
			Fields: map[string]string{
				"commentId": "Choose a comment first.",
			},
		}
	}

	target := commentReactionTargetState{
		Post:      post,
		CommentID: normalizedCommentID,
	}
	if err := s.db.QueryRowContext(
		ctx,
		`SELECT author_id FROM comments WHERE id = $1 AND post_id = $2`,
		normalizedCommentID,
		post.ID,
	).Scan(&target.AuthorID); err != nil {
		if err == sql.ErrNoRows {
			return commentReactionTargetState{}, ErrNotFound
		}

		return commentReactionTargetState{}, fmt.Errorf("load comment reaction target: %w", err)
	}

	return target, nil
}

func (s Service) requireGroupCommentAccess(ctx context.Context, userID, groupID, groupPostID, commentID string) (string, error) {
	target, err := s.loadGroupCommentReactionTarget(ctx, userID, groupID, groupPostID, commentID)
	if err != nil {
		return "", err
	}

	return target.CommentID, nil
}

func (s Service) loadGroupCommentReactionTarget(ctx context.Context, userID, groupID, groupPostID, commentID string) (groupCommentReactionTargetState, error) {
	post, err := s.loadGroupPostState(ctx, s.db, userID, groupID, groupPostID)
	if err != nil {
		return groupCommentReactionTargetState{}, err
	}

	normalizedCommentID := strings.TrimSpace(commentID)
	if normalizedCommentID == "" {
		return groupCommentReactionTargetState{}, &ValidationError{
			Message: "Choose a group comment first.",
			Fields: map[string]string{
				"commentId": "Choose a group comment first.",
			},
		}
	}

	target := groupCommentReactionTargetState{
		Post:      post,
		CommentID: normalizedCommentID,
	}
	if err := s.db.QueryRowContext(
		ctx,
		`SELECT author_id FROM group_comments WHERE id = $1 AND group_post_id = $2`,
		normalizedCommentID,
		post.ID,
	).Scan(&target.AuthorID); err != nil {
		if err == sql.ErrNoRows {
			return groupCommentReactionTargetState{}, ErrNotFound
		}

		return groupCommentReactionTargetState{}, fmt.Errorf("load group comment reaction target: %w", err)
	}

	return target, nil
}

func (s Service) createReactionWithNotification(ctx context.Context, target reactionTargetConfig, targetID, userID, reactionType string, notification reactionNotification) (notificationDelivery, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return notificationDelivery{}, fmt.Errorf("begin %s reaction transaction: %w", target.TargetType, err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	inserted, err := s.upsertReaction(ctx, tx, target, targetID, userID, reactionType)
	if err != nil {
		return notificationDelivery{}, err
	}

	var createdNotification Notification
	if inserted && strings.TrimSpace(notification.RecipientID) != "" && notification.RecipientID != userID {
		createdNotification, err = s.insertNotification(
			ctx,
			tx,
			notification.RecipientID,
			notification.Type,
			notification.Title,
			notification.Body,
			notification.EntityType,
			notification.EntityID,
			notification.Metadata,
		)
		if err != nil {
			return notificationDelivery{}, err
		}
	}

	if err = tx.Commit(); err != nil {
		return notificationDelivery{}, fmt.Errorf("commit %s reaction transaction: %w", target.TargetType, err)
	}

	return notificationDelivery{
		UserID:       notification.RecipientID,
		Notification: createdNotification,
	}, nil
}

func (s Service) publishReactionNotification(delivery notificationDelivery) {
	if delivery.Notification.ID == "" {
		return
	}

	s.publisher.PublishToUser(delivery.UserID, "notification.created", NotificationEvent{
		Notification: delivery.Notification,
	})
}

func (s Service) upsertReaction(ctx context.Context, executor sqlExecutor, target reactionTargetConfig, targetID, userID, reactionType string) (bool, error) {
	query := fmt.Sprintf(
		`
			INSERT INTO %s (%s, user_id, reaction_type, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), NOW())
			ON CONFLICT (%s, user_id)
			DO NOTHING
		`,
		target.Table,
		target.TargetColumn,
		target.TargetColumn,
	)

	result, err := executor.ExecContext(ctx, query, targetID, userID, reactionType)
	if err != nil {
		return false, fmt.Errorf("upsert %s reaction: %w", target.TargetType, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("count %s reaction rows: %w", target.TargetType, err)
	}

	return rowsAffected > 0, nil
}

func (s Service) deleteReaction(ctx context.Context, target reactionTargetConfig, targetID, userID string) error {
	query := fmt.Sprintf(
		`DELETE FROM %s WHERE %s = $1 AND user_id = $2`,
		target.Table,
		target.TargetColumn,
	)

	if _, err := s.db.ExecContext(ctx, query, targetID, userID); err != nil {
		return fmt.Errorf("delete %s reaction: %w", target.TargetType, err)
	}

	return nil
}

func (s Service) loadReactionResult(ctx context.Context, target reactionTargetConfig, viewerID, targetID string) (ReactionResult, error) {
	summaries, err := s.loadReactionSummaries(ctx, target, viewerID, []string{targetID})
	if err != nil {
		return ReactionResult{}, err
	}

	summary := summaries[targetID]
	return ReactionResult{
		TargetType:     target.TargetType,
		TargetID:       targetID,
		ReactionsCount: summary.Count,
		ViewerReaction: summary.ViewerReaction,
		ReactionUsers:  ensureReactionUsers(summary.Users),
	}, nil
}

func reactionPostBody(ctx context.Context, s Service, actorID, postTitle string) string {
	actorName := reactionActorName(ctx, s, actorID)
	if strings.TrimSpace(postTitle) == "" {
		return fmt.Sprintf("%s reacted to your post.", actorName)
	}

	return fmt.Sprintf("%s reacted to \"%s\".", actorName, postTitle)
}

func reactionCommentBody(ctx context.Context, s Service, actorID, postTitle string) string {
	actorName := reactionActorName(ctx, s, actorID)
	if strings.TrimSpace(postTitle) == "" {
		return fmt.Sprintf("%s reacted to your comment.", actorName)
	}

	return fmt.Sprintf("%s reacted to your comment on \"%s\".", actorName, postTitle)
}

func reactionGroupPostBody(ctx context.Context, s Service, actorID string) string {
	return fmt.Sprintf("%s reacted to your group post.", reactionActorName(ctx, s, actorID))
}

func reactionGroupCommentBody(ctx context.Context, s Service, actorID string) string {
	return fmt.Sprintf("%s reacted to your group comment.", reactionActorName(ctx, s, actorID))
}

func reactionActorName(ctx context.Context, s Service, actorID string) string {
	actor, err := s.loadUserIdentity(ctx, s.db, actorID)
	if err != nil {
		return "Someone"
	}

	return actor.DisplayName()
}

func (s Service) loadReactionSummaries(ctx context.Context, target reactionTargetConfig, viewerID string, targetIDs []string) (map[string]reactionSummary, error) {
	summariesByID := make(map[string]reactionSummary, len(targetIDs))
	if len(targetIDs) == 0 {
		return summariesByID, nil
	}

	countsQuery := fmt.Sprintf(
		`
			SELECT %s, COUNT(*)::INT
			FROM %s
			WHERE %s = ANY($1)
			GROUP BY %s
		`,
		target.TargetColumn,
		target.Table,
		target.TargetColumn,
		target.TargetColumn,
	)

	rows, err := s.db.QueryContext(ctx, countsQuery, pq.Array(targetIDs))
	if err != nil {
		return nil, fmt.Errorf("query %s reaction counts: %w", target.TargetType, err)
	}
	defer rows.Close()

	for rows.Next() {
		var targetID string
		var count int
		if err := rows.Scan(&targetID, &count); err != nil {
			return nil, fmt.Errorf("scan %s reaction count: %w", target.TargetType, err)
		}

		summary := summariesByID[targetID]
		summary.Count = count
		summariesByID[targetID] = summary
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s reaction counts: %w", target.TargetType, err)
	}

	if strings.TrimSpace(viewerID) == "" {
		return s.loadReactionUsers(ctx, target, targetIDs, summariesByID)
	}

	viewerQuery := fmt.Sprintf(
		`
			SELECT %s, reaction_type
			FROM %s
			WHERE user_id = $1 AND %s = ANY($2)
		`,
		target.TargetColumn,
		target.Table,
		target.TargetColumn,
	)

	viewerRows, err := s.db.QueryContext(ctx, viewerQuery, viewerID, pq.Array(targetIDs))
	if err != nil {
		return nil, fmt.Errorf("query %s viewer reactions: %w", target.TargetType, err)
	}
	defer viewerRows.Close()

	for viewerRows.Next() {
		var targetID string
		var reactionType string
		if err := viewerRows.Scan(&targetID, &reactionType); err != nil {
			return nil, fmt.Errorf("scan %s viewer reaction: %w", target.TargetType, err)
		}

		summary := summariesByID[targetID]
		summary.ViewerReaction = reactionType
		summariesByID[targetID] = summary
	}

	if err := viewerRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s viewer reactions: %w", target.TargetType, err)
	}

	return s.loadReactionUsers(ctx, target, targetIDs, summariesByID)
}

func (s Service) loadReactionUsers(ctx context.Context, target reactionTargetConfig, targetIDs []string, summariesByID map[string]reactionSummary) (map[string]reactionSummary, error) {
	usersQuery := fmt.Sprintf(
		`
			SELECT
				reactions.target_id,
				u.id,
				u.first_name,
				u.last_name,
				u.nickname,
				u.avatar_url
			FROM (
				SELECT
					%s AS target_id,
					user_id,
					ROW_NUMBER() OVER (
						PARTITION BY %s
						ORDER BY updated_at DESC, user_id DESC
					) AS row_number
				FROM %s
				WHERE %s = ANY($1)
			) reactions
			INNER JOIN users u ON u.id = reactions.user_id
			WHERE reactions.row_number <= 8
			ORDER BY reactions.target_id, reactions.row_number
		`,
		target.TargetColumn,
		target.TargetColumn,
		target.Table,
		target.TargetColumn,
	)

	rows, err := s.db.QueryContext(ctx, usersQuery, pq.Array(targetIDs))
	if err != nil {
		return nil, fmt.Errorf("query %s reaction users: %w", target.TargetType, err)
	}
	defer rows.Close()

	for rows.Next() {
		var targetID string
		var user ReactionUser
		var nickname sql.NullString
		var avatarURL sql.NullString
		if err := rows.Scan(
			&targetID,
			&user.ID,
			&user.FirstName,
			&user.LastName,
			&nickname,
			&avatarURL,
		); err != nil {
			return nil, fmt.Errorf("scan %s reaction user: %w", target.TargetType, err)
		}

		user.Nickname = nullStringValue(nickname)
		if avatarURL.Valid {
			user.AvatarURL = s.publicURL(avatarURL.String)
		}

		summary := summariesByID[targetID]
		summary.Users = append(summary.Users, user)
		summariesByID[targetID] = summary
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s reaction users: %w", target.TargetType, err)
	}

	return summariesByID, nil
}

func normalizeReactionType(reactionType string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(reactionType))
	if normalized == "" {
		normalized = reactionLike
	}

	if normalized != reactionLike {
		return "", &ValidationError{
			Message: "Reaction type is invalid.",
			Fields: map[string]string{
				"reaction": "Reaction type is invalid.",
			},
		}
	}

	return normalized, nil
}

func ensureReactionUsers(users []ReactionUser) []ReactionUser {
	if users == nil {
		return []ReactionUser{}
	}

	return users
}
