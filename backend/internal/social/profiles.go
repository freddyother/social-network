package social

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type PublicProfile struct {
	ID                 string `json:"id"`
	FirstName          string `json:"firstName"`
	LastName           string `json:"lastName"`
	Nickname           string `json:"nickname,omitempty"`
	AboutMe            string `json:"aboutMe,omitempty"`
	AvatarURL          string `json:"avatarUrl,omitempty"`
	ProfileVisibility  string `json:"profileVisibility"`
	RelationshipStatus string `json:"relationshipStatus"`
	FollowersCount     int    `json:"followersCount"`
	FollowingCount     int    `json:"followingCount"`
	PostsCount         int    `json:"postsCount"`
	IsViewer           bool   `json:"isViewer"`
	CanViewPosts       bool   `json:"canViewPosts"`
	CanMessage         bool   `json:"canMessage"`
}

type PublicProfilePage struct {
	Profile PublicProfile
	Posts   []Post
}

type profileState struct {
	PublicProfile
	ViewerFollowsTarget bool
	ViewerRequested     bool
}

func (s Service) ProfileByHandle(ctx context.Context, viewerID, handle string) (PublicProfilePage, error) {
	profile, err := s.loadProfileByHandle(ctx, viewerID, handle)
	if err != nil {
		return PublicProfilePage{}, err
	}

	posts, err := s.loadProfilePosts(ctx, profile)
	if err != nil {
		return PublicProfilePage{}, err
	}

	return PublicProfilePage{
		Profile: profile.PublicProfile,
		Posts:   posts,
	}, nil
}

func (s Service) loadProfileByHandle(ctx context.Context, viewerID, handle string) (profileState, error) {
	normalizedViewerID := strings.TrimSpace(viewerID)
	normalizedHandle := strings.TrimSpace(handle)
	if normalizedHandle == "" {
		return profileState{}, &ValidationError{
			Message: "Choose a profile to open.",
			Fields: map[string]string{
				"handle": "Choose a profile to open.",
			},
		}
	}

	row := s.db.QueryRowContext(
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
				EXISTS (
					SELECT 1
					FROM followers f
					WHERE f.follower_id = $1 AND f.followee_id = u.id
				) AS viewer_follows_target,
				EXISTS (
					SELECT 1
					FROM follow_requests fr
					WHERE fr.sender_id = $1
					  AND fr.recipient_id = u.id
					  AND fr.status = 'pending'
				) AS viewer_requested_target,
				COALESCE((
					SELECT COUNT(*)::INT
					FROM followers f
					WHERE f.followee_id = u.id
				), 0) AS followers_count,
				COALESCE((
					SELECT COUNT(*)::INT
					FROM followers f
					WHERE f.follower_id = u.id
				), 0) AS following_count,
				COALESCE((
					SELECT COUNT(*)::INT
					FROM posts p
					WHERE p.author_id = u.id
				), 0) AS posts_count
			FROM users u
			WHERE LOWER(BTRIM(u.nickname)) = LOWER(BTRIM($2))
			  AND BTRIM(COALESCE(u.nickname, '')) <> ''
		`,
		normalizedViewerID,
		normalizedHandle,
	)

	var profile profileState
	var nickname sql.NullString
	var aboutMe sql.NullString
	var avatarURL sql.NullString
	if err := row.Scan(
		&profile.ID,
		&profile.FirstName,
		&profile.LastName,
		&nickname,
		&aboutMe,
		&avatarURL,
		&profile.ProfileVisibility,
		&profile.ViewerFollowsTarget,
		&profile.ViewerRequested,
		&profile.FollowersCount,
		&profile.FollowingCount,
		&profile.PostsCount,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return profileState{}, ErrNotFound
		}

		return profileState{}, fmt.Errorf("load profile by handle: %w", err)
	}

	profile.Nickname = nullStringValue(nickname)
	profile.AboutMe = nullStringValue(aboutMe)
	if avatarURL.Valid {
		profile.AvatarURL = s.publicURL(avatarURL.String)
	}

	profile.IsViewer = normalizedViewerID != "" && normalizedViewerID == profile.ID
	profile.CanMessage = normalizedViewerID != "" &&
		!profile.IsViewer &&
		(profile.ProfileVisibility == "public" || profile.ViewerFollowsTarget)

	switch {
	case profile.IsViewer:
		profile.RelationshipStatus = "self"
	case profile.ViewerFollowsTarget:
		profile.RelationshipStatus = "following"
	case profile.ViewerRequested:
		profile.RelationshipStatus = "requested"
	default:
		profile.RelationshipStatus = "not_following"
	}

	switch {
	case profile.IsViewer:
		profile.CanViewPosts = true
	case profile.ProfileVisibility == "private":
		profile.CanViewPosts = profile.ViewerFollowsTarget
	default:
		profile.CanViewPosts = true
	}

	return profile, nil
}

func (s Service) loadProfilePosts(ctx context.Context, profile profileState) ([]Post, error) {
	if !profile.CanViewPosts {
		return []Post{}, nil
	}

	query := `
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
		WHERE p.author_id = $1
	`

	switch {
	case profile.IsViewer:
		// Owners can always see their own posts.
	case profile.ProfileVisibility == "private":
		// Approved followers can see the whole private profile timeline.
	case profile.RelationshipStatus == "following":
		query += "\nAND p.privacy IN ('public', 'followers')"
	default:
		query += "\nAND p.privacy = 'public'"
	}

	query += "\nORDER BY p.created_at DESC LIMIT 50"

	rows, err := s.db.QueryContext(ctx, query, profile.ID)
	if err != nil {
		return nil, fmt.Errorf("query profile posts: %w", err)
	}
	defer rows.Close()

	return s.loadPostsFromRows(ctx, rows, "profile")
}
