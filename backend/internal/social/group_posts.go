package social

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"social-network/backend/internal/auth"
)

const maxGroupPostBodyLength = 4000

type CreateGroupPostInput struct {
	Body string
}

type GroupPost struct {
	ID            string    `json:"id"`
	GroupID       string    `json:"groupId"`
	Body          string    `json:"body"`
	ImageURL      string    `json:"imageUrl,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	CommentsCount int       `json:"commentsCount"`
	Author        GroupUser `json:"author"`
}

func (s Service) GroupPosts(ctx context.Context, viewerID, groupID string) ([]GroupPost, error) {
	normalizedGroupID, err := s.requireGroupMembership(ctx, s.db, viewerID, groupID)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(
		ctx,
		`
			SELECT
				gp.id,
				gp.group_id,
				gp.body,
				gp.image_url,
				gp.created_at,
				gp.updated_at,
				u.id,
				u.first_name,
				u.last_name,
				u.nickname,
				u.avatar_url,
				COALESCE(comment_counts.comments_count, 0) AS comments_count
			FROM group_posts gp
			INNER JOIN users u ON u.id = gp.author_id
			LEFT JOIN (
				SELECT group_post_id, COUNT(*)::INT AS comments_count
				FROM group_comments
				GROUP BY group_post_id
			) comment_counts ON comment_counts.group_post_id = gp.id
			WHERE gp.group_id = $1
			ORDER BY gp.created_at DESC, gp.id DESC
			LIMIT 50
		`,
		normalizedGroupID,
	)
	if err != nil {
		return nil, fmt.Errorf("query group posts: %w", err)
	}
	defer rows.Close()

	return s.loadGroupPostsFromRows(rows, "group timeline")
}

func (s Service) CreateGroupPost(ctx context.Context, author auth.User, groupID string, input CreateGroupPostInput) (GroupPost, error) {
	normalizedGroupID, err := s.requireGroupMembership(ctx, s.db, author.ID, groupID)
	if err != nil {
		return GroupPost{}, err
	}

	normalizedInput, err := normalizeCreateGroupPostInput(input)
	if err != nil {
		return GroupPost{}, err
	}

	groupPostID, err := newToken(16)
	if err != nil {
		return GroupPost{}, fmt.Errorf("generate group post id: %w", err)
	}

	var createdAt time.Time
	var updatedAt time.Time
	if err := s.db.QueryRowContext(
		ctx,
		`
			INSERT INTO group_posts (id, group_id, author_id, body)
			VALUES ($1, $2, $3, $4)
			RETURNING created_at, updated_at
		`,
		groupPostID,
		normalizedGroupID,
		author.ID,
		normalizedInput.Body,
	).Scan(&createdAt, &updatedAt); err != nil {
		return GroupPost{}, fmt.Errorf("insert group post: %w", err)
	}

	return GroupPost{
		ID:        groupPostID,
		GroupID:   normalizedGroupID,
		Body:      normalizedInput.Body,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Author: GroupUser{
			ID:        author.ID,
			FirstName: author.FirstName,
			LastName:  author.LastName,
			Nickname:  author.Nickname,
			AvatarURL: author.AvatarURL,
		},
	}, nil
}

func (s Service) requireGroupMembership(ctx context.Context, reader sqlReader, userID, groupID string) (string, error) {
	normalizedGroupID := strings.TrimSpace(groupID)
	if normalizedGroupID == "" {
		return "", &ValidationError{
			Message: "Choose a group first.",
			Fields: map[string]string{
				"groupId": "Choose a group first.",
			},
		}
	}

	var exists bool
	var role sql.NullString
	if err := reader.QueryRowContext(
		ctx,
		`
			SELECT
				EXISTS (SELECT 1 FROM groups WHERE id = $1) AS group_exists,
				(
					SELECT gm.role
					FROM group_memberships gm
					WHERE gm.group_id = $1 AND gm.user_id = $2
					LIMIT 1
				) AS membership_role
		`,
		normalizedGroupID,
		userID,
	).Scan(&exists, &role); err != nil {
		return "", fmt.Errorf("load group membership: %w", err)
	}

	if !exists {
		return "", ErrNotFound
	}

	if !role.Valid || strings.TrimSpace(role.String) == "" {
		return "", ErrForbidden
	}

	return normalizedGroupID, nil
}

func (s Service) loadGroupPostsFromRows(rows *sql.Rows, operation string) ([]GroupPost, error) {
	posts := make([]GroupPost, 0)

	for rows.Next() {
		var (
			post            GroupPost
			authorNickname  sql.NullString
			authorAvatarURL sql.NullString
			imageURL        sql.NullString
		)
		if err := rows.Scan(
			&post.ID,
			&post.GroupID,
			&post.Body,
			&imageURL,
			&post.CreatedAt,
			&post.UpdatedAt,
			&post.Author.ID,
			&post.Author.FirstName,
			&post.Author.LastName,
			&authorNickname,
			&authorAvatarURL,
			&post.CommentsCount,
		); err != nil {
			return nil, fmt.Errorf("scan %s group post: %w", operation, err)
		}

		post.Author.Nickname = nullStringValue(authorNickname)
		if authorAvatarURL.Valid {
			post.Author.AvatarURL = s.publicURL(authorAvatarURL.String)
		}
		if imageURL.Valid {
			post.ImageURL = s.publicURL(imageURL.String)
		}

		posts = append(posts, post)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s group posts: %w", operation, err)
	}

	return posts, nil
}

func normalizeCreateGroupPostInput(input CreateGroupPostInput) (CreateGroupPostInput, error) {
	normalized := CreateGroupPostInput{
		Body: strings.TrimSpace(input.Body),
	}

	fieldErrors := make(map[string]string)
	if normalized.Body == "" {
		fieldErrors["body"] = "Write something for your group."
	} else if len(normalized.Body) > maxGroupPostBodyLength {
		fieldErrors["body"] = "Group posts must be 4000 characters or fewer."
	}

	if len(fieldErrors) > 0 {
		return CreateGroupPostInput{}, &ValidationError{
			Message: "Please correct the group post.",
			Fields:  fieldErrors,
		}
	}

	return normalized, nil
}
