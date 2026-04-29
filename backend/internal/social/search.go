package social

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const (
	maxGlobalSearchQueryLength = 80
)

type SearchUser struct {
	ID                 string `json:"id"`
	FirstName          string `json:"firstName"`
	LastName           string `json:"lastName"`
	Nickname           string `json:"nickname,omitempty"`
	AboutMe            string `json:"aboutMe,omitempty"`
	AvatarURL          string `json:"avatarUrl,omitempty"`
	ProfileVisibility  string `json:"profileVisibility"`
	RelationshipStatus string `json:"relationshipStatus"`
}

type GlobalSearchResult struct {
	Query  string       `json:"query"`
	Users  []SearchUser `json:"users"`
	Posts  []Post       `json:"posts"`
	Groups []Group      `json:"groups"`
}

func (s Service) Search(ctx context.Context, viewerID, query string) (GlobalSearchResult, error) {
	normalizedQuery, err := normalizeGlobalSearchQuery(query)
	if err != nil {
		return GlobalSearchResult{}, err
	}

	users, err := s.searchUsers(ctx, viewerID, normalizedQuery)
	if err != nil {
		return GlobalSearchResult{}, err
	}

	posts, err := s.searchPosts(ctx, viewerID, normalizedQuery)
	if err != nil {
		return GlobalSearchResult{}, err
	}

	groups, err := s.searchGroups(ctx, viewerID, normalizedQuery)
	if err != nil {
		return GlobalSearchResult{}, err
	}

	return GlobalSearchResult{
		Query:  normalizedQuery,
		Users:  users,
		Posts:  posts,
		Groups: groups,
	}, nil
}

func (s Service) searchUsers(ctx context.Context, viewerID, query string) ([]SearchUser, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`
			SELECT
				u.id,
				u.first_name,
				u.last_name,
				u.nickname,
				u.about_me,
				u.avatar_url,
				u.profile_visibility,
				CASE
					WHEN u.id = $1 THEN 'self'
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
				LOWER(BTRIM(COALESCE(u.nickname, ''))) LIKE '%' || LOWER($2) || '%'
				OR LOWER(BTRIM(u.first_name || ' ' || u.last_name)) LIKE '%' || LOWER($2) || '%'
				OR LOWER(BTRIM(COALESCE(u.about_me, ''))) LIKE '%' || LOWER($2) || '%'
			ORDER BY
				CASE
					WHEN LOWER(BTRIM(COALESCE(u.nickname, ''))) = LOWER($2) THEN 0
					WHEN LOWER(BTRIM(COALESCE(u.nickname, ''))) LIKE LOWER($2) || '%' THEN 1
					WHEN LOWER(BTRIM(u.first_name || ' ' || u.last_name)) LIKE LOWER($2) || '%' THEN 2
					ELSE 3
				END,
				CASE WHEN u.id = $1 THEN 0 ELSE 1 END,
				u.created_at DESC
			LIMIT 8
		`,
		viewerID,
		query,
	)
	if err != nil {
		return nil, fmt.Errorf("query search users: %w", err)
	}
	defer rows.Close()

	users := make([]SearchUser, 0)
	for rows.Next() {
		var (
			user      SearchUser
			nickname  sql.NullString
			aboutMe   sql.NullString
			avatarURL sql.NullString
		)
		if err := rows.Scan(
			&user.ID,
			&user.FirstName,
			&user.LastName,
			&nickname,
			&aboutMe,
			&avatarURL,
			&user.ProfileVisibility,
			&user.RelationshipStatus,
		); err != nil {
			return nil, fmt.Errorf("scan search user: %w", err)
		}

		user.Nickname = nullStringValue(nickname)
		user.AboutMe = nullStringValue(aboutMe)
		if avatarURL.Valid {
			user.AvatarURL = s.publicURL(avatarURL.String)
		}

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search users: %w", err)
	}

	return users, nil
}

func (s Service) searchPosts(ctx context.Context, viewerID, query string) ([]Post, error) {
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
			LEFT JOIN followers f
				ON f.follower_id = $1
				AND f.followee_id = p.author_id
			WHERE
				(
					p.author_id = $1
					OR (
						u.profile_visibility = 'public'
						AND (
							p.privacy = 'public'
							OR (p.privacy = 'followers' AND f.follower_id IS NOT NULL)
						)
					)
					OR (
						u.profile_visibility = 'private'
						AND f.follower_id IS NOT NULL
					)
				)
				AND (
					LOWER(COALESCE(p.title, '')) LIKE '%' || LOWER($2) || '%'
					OR LOWER(COALESCE(p.body, '')) LIKE '%' || LOWER($2) || '%'
					OR LOWER(BTRIM(COALESCE(u.nickname, ''))) LIKE '%' || LOWER($2) || '%'
					OR LOWER(BTRIM(u.first_name || ' ' || u.last_name)) LIKE '%' || LOWER($2) || '%'
				)
			ORDER BY
				CASE
					WHEN LOWER(COALESCE(p.title, '')) LIKE LOWER($2) || '%' THEN 0
					WHEN LOWER(COALESCE(p.body, '')) LIKE LOWER($2) || '%' THEN 1
					ELSE 2
				END,
				p.created_at DESC,
				p.id DESC
			LIMIT 8
		`,
		viewerID,
		query,
	)
	if err != nil {
		return nil, fmt.Errorf("query search posts: %w", err)
	}
	defer rows.Close()

	return s.loadPostsFromRows(ctx, rows, "search")
}

func (s Service) searchGroups(ctx context.Context, viewerID, query string) ([]Group, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`
			SELECT
				g.id,
				g.title,
				g.description,
				g.created_at,
				g.updated_at,
				u.id,
				u.first_name,
				u.last_name,
				u.nickname,
				u.avatar_url,
				gm.role,
				viewer_request.id,
				viewer_request.status,
				COALESCE(member_counts.members_count, 0) AS members_count,
				COALESCE(post_counts.posts_count, 0) AS posts_count,
				COALESCE(event_counts.events_count, 0) AS events_count,
				COALESCE(request_counts.pending_requests_count, 0) AS pending_requests_count
			FROM groups g
			INNER JOIN users u ON u.id = g.creator_id
			LEFT JOIN group_memberships gm
				ON gm.group_id = g.id
				AND gm.user_id = $1
			LEFT JOIN LATERAL (
				SELECT gjr.id, gjr.status
				FROM group_join_requests gjr
				WHERE
					gjr.group_id = g.id
					AND gjr.requester_id = $1
					AND gjr.status = 'pending'
				ORDER BY gjr.created_at DESC, gjr.id DESC
				LIMIT 1
			) viewer_request ON TRUE
			LEFT JOIN (
				SELECT group_id, COUNT(*)::INT AS members_count
				FROM group_memberships
				GROUP BY group_id
			) member_counts ON member_counts.group_id = g.id
			LEFT JOIN (
				SELECT group_id, COUNT(*)::INT AS posts_count
				FROM group_posts
				GROUP BY group_id
			) post_counts ON post_counts.group_id = g.id
			LEFT JOIN (
				SELECT group_id, COUNT(*)::INT AS events_count
				FROM group_events
				GROUP BY group_id
			) event_counts ON event_counts.group_id = g.id
			LEFT JOIN (
				SELECT group_id, COUNT(*)::INT AS pending_requests_count
				FROM group_join_requests
				WHERE status = 'pending'
				GROUP BY group_id
			) request_counts ON request_counts.group_id = g.id
			WHERE
				LOWER(BTRIM(g.title)) LIKE '%' || LOWER($2) || '%'
				OR LOWER(BTRIM(g.description)) LIKE '%' || LOWER($2) || '%'
				OR LOWER(BTRIM(COALESCE(u.nickname, ''))) LIKE '%' || LOWER($2) || '%'
				OR LOWER(BTRIM(u.first_name || ' ' || u.last_name)) LIKE '%' || LOWER($2) || '%'
			ORDER BY
				CASE
					WHEN LOWER(BTRIM(g.title)) = LOWER($2) THEN 0
					WHEN LOWER(BTRIM(g.title)) LIKE LOWER($2) || '%' THEN 1
					ELSE 2
				END,
				CASE WHEN gm.user_id IS NULL THEN 1 ELSE 0 END,
				g.created_at DESC,
				g.id DESC
			LIMIT 8
		`,
		viewerID,
		query,
	)
	if err != nil {
		return nil, fmt.Errorf("query search groups: %w", err)
	}
	defer rows.Close()

	groups := make([]Group, 0)
	for rows.Next() {
		group, scanErr := s.scanGroup(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan search group: %w", scanErr)
		}

		groups = append(groups, group)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search groups: %w", err)
	}

	return groups, nil
}

func normalizeGlobalSearchQuery(query string) (string, error) {
	normalizedQuery := strings.TrimSpace(query)

	fieldErrors := make(map[string]string)
	if normalizedQuery == "" {
		fieldErrors["q"] = "Write something to search for."
	} else if len(normalizedQuery) < 2 {
		fieldErrors["q"] = "Search terms must be at least 2 characters."
	} else if len(normalizedQuery) > maxGlobalSearchQueryLength {
		fieldErrors["q"] = "Search terms must be 80 characters or fewer."
	}

	if len(fieldErrors) > 0 {
		return "", &ValidationError{
			Message: "Please correct the search query.",
			Fields:  fieldErrors,
		}
	}

	return normalizedQuery, nil
}
