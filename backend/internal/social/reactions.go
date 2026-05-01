package social

import (
	"context"
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
	TargetType     string `json:"targetType"`
	TargetID       string `json:"targetId"`
	ReactionsCount int    `json:"reactionsCount"`
	ViewerReaction string `json:"viewerReaction,omitempty"`
}

type reactionSummary struct {
	Count          int
	ViewerReaction string
}

type reactionTargetConfig struct {
	TargetType   string
	Table        string
	TargetColumn string
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

	if err := s.upsertReaction(ctx, postReactionTarget, post.ID, userID, normalizedReaction); err != nil {
		return ReactionResult{}, err
	}

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

	normalizedCommentID, err := s.requireCommentAccess(ctx, userID, postID, commentID)
	if err != nil {
		return ReactionResult{}, err
	}

	if err := s.upsertReaction(ctx, commentReactionTarget, normalizedCommentID, userID, normalizedReaction); err != nil {
		return ReactionResult{}, err
	}

	return s.loadReactionResult(ctx, commentReactionTarget, userID, normalizedCommentID)
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

	if err := s.upsertReaction(ctx, groupPostReactionTarget, post.ID, userID, normalizedReaction); err != nil {
		return ReactionResult{}, err
	}

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

	normalizedCommentID, err := s.requireGroupCommentAccess(ctx, userID, groupID, groupPostID, commentID)
	if err != nil {
		return ReactionResult{}, err
	}

	if err := s.upsertReaction(ctx, groupCommentReactionTarget, normalizedCommentID, userID, normalizedReaction); err != nil {
		return ReactionResult{}, err
	}

	return s.loadReactionResult(ctx, groupCommentReactionTarget, userID, normalizedCommentID)
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
	post, err := s.loadVisiblePost(ctx, s.db, userID, strings.TrimSpace(postID))
	if err != nil {
		return "", err
	}

	normalizedCommentID := strings.TrimSpace(commentID)
	if normalizedCommentID == "" {
		return "", &ValidationError{
			Message: "Choose a comment first.",
			Fields: map[string]string{
				"commentId": "Choose a comment first.",
			},
		}
	}

	var exists bool
	if err := s.db.QueryRowContext(
		ctx,
		`SELECT EXISTS (SELECT 1 FROM comments WHERE id = $1 AND post_id = $2)`,
		normalizedCommentID,
		post.ID,
	).Scan(&exists); err != nil {
		return "", fmt.Errorf("check comment access: %w", err)
	}

	if !exists {
		return "", ErrNotFound
	}

	return normalizedCommentID, nil
}

func (s Service) requireGroupCommentAccess(ctx context.Context, userID, groupID, groupPostID, commentID string) (string, error) {
	post, err := s.loadGroupPostState(ctx, s.db, userID, groupID, groupPostID)
	if err != nil {
		return "", err
	}

	normalizedCommentID := strings.TrimSpace(commentID)
	if normalizedCommentID == "" {
		return "", &ValidationError{
			Message: "Choose a group comment first.",
			Fields: map[string]string{
				"commentId": "Choose a group comment first.",
			},
		}
	}

	var exists bool
	if err := s.db.QueryRowContext(
		ctx,
		`SELECT EXISTS (SELECT 1 FROM group_comments WHERE id = $1 AND group_post_id = $2)`,
		normalizedCommentID,
		post.ID,
	).Scan(&exists); err != nil {
		return "", fmt.Errorf("check group comment access: %w", err)
	}

	if !exists {
		return "", ErrNotFound
	}

	return normalizedCommentID, nil
}

func (s Service) upsertReaction(ctx context.Context, target reactionTargetConfig, targetID, userID, reactionType string) error {
	query := fmt.Sprintf(
		`
			INSERT INTO %s (%s, user_id, reaction_type, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), NOW())
			ON CONFLICT (%s, user_id)
			DO UPDATE SET reaction_type = EXCLUDED.reaction_type, updated_at = NOW()
		`,
		target.Table,
		target.TargetColumn,
		target.TargetColumn,
	)

	if _, err := s.db.ExecContext(ctx, query, targetID, userID, reactionType); err != nil {
		return fmt.Errorf("upsert %s reaction: %w", target.TargetType, err)
	}

	return nil
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
	}, nil
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
		return summariesByID, nil
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
