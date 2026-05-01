package social

import (
	"context"
	"fmt"
	"strings"
)

func (s Service) PostByID(ctx context.Context, viewerID, postID string) (Post, error) {
	normalizedPostID := strings.TrimSpace(postID)
	if normalizedPostID == "" {
		return Post{}, &ValidationError{
			Message: "Choose a post to open.",
			Fields: map[string]string{
				"postId": "Choose a post to open.",
			},
		}
	}

	if _, err := s.loadVisiblePost(ctx, s.db, viewerID, normalizedPostID); err != nil {
		return Post{}, err
	}

	rows, err := s.db.QueryContext(
		ctx,
		`
			SELECT
				p.id,
				p.title,
				p.body,
				p.privacy,
				p.created_at,
				p.updated_at,
				u.id,
				u.first_name,
				u.last_name,
				u.nickname,
				u.profile_visibility
			FROM posts p
			INNER JOIN users u ON u.id = p.author_id
			WHERE p.id = $1
			LIMIT 1
		`,
		normalizedPostID,
	)
	if err != nil {
		return Post{}, fmt.Errorf("query post by id: %w", err)
	}
	defer rows.Close()

	posts, err := s.loadPostsFromRows(ctx, rows, "single post", viewerID)
	if err != nil {
		return Post{}, err
	}

	if len(posts) == 0 {
		return Post{}, ErrNotFound
	}

	return posts[0], nil
}
