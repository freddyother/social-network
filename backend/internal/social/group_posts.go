package social

import (
	"context"
	"database/sql"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lib/pq"

	"social-network/backend/internal/auth"
	uploadmedia "social-network/backend/internal/media"
)

const maxGroupPostBodyLength = 4000

type CreateGroupPostInput struct {
	Body   string
	Images []*multipart.FileHeader
}

type GroupPostMedia struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	SortOrder int    `json:"sortOrder"`
}

type GroupPost struct {
	ID            string           `json:"id"`
	GroupID       string           `json:"groupId"`
	Body          string           `json:"body"`
	ImageURL      string           `json:"imageUrl,omitempty"`
	Media         []GroupPostMedia `json:"media"`
	CreatedAt     time.Time        `json:"createdAt"`
	UpdatedAt     time.Time        `json:"updatedAt"`
	CommentsCount int              `json:"commentsCount"`
	Author        GroupUser        `json:"author"`
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

	return s.loadGroupPostsFromRows(ctx, rows, "group timeline")
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

	savedMedia, err := s.saveGroupPostMedia(normalizedGroupID, groupPostID, normalizedInput.Images)
	if err != nil {
		return GroupPost{}, err
	}

	cleanupMedia := len(savedMedia) > 0
	defer func() {
		if cleanupMedia {
			_ = os.RemoveAll(filepath.Join(s.uploadsDir, "groups", normalizedGroupID, "posts", groupPostID))
		}
	}()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return GroupPost{}, fmt.Errorf("begin create group post transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var coverPath any
	if len(savedMedia) > 0 {
		coverPath = savedMedia[0].StoragePath
	}

	var createdAt time.Time
	var updatedAt time.Time
	if err = tx.QueryRowContext(
		ctx,
		`
			INSERT INTO group_posts (id, group_id, author_id, body, image_url)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING created_at, updated_at
		`,
		groupPostID,
		normalizedGroupID,
		author.ID,
		normalizedInput.Body,
		coverPath,
	).Scan(&createdAt, &updatedAt); err != nil {
		return GroupPost{}, fmt.Errorf("insert group post: %w", err)
	}

	for _, media := range savedMedia {
		if _, err = tx.ExecContext(
			ctx,
			`
				INSERT INTO group_post_media (id, group_post_id, storage_path, sort_order)
				VALUES ($1, $2, $3, $4)
			`,
			media.ID,
			groupPostID,
			media.StoragePath,
			media.SortOrder,
		); err != nil {
			return GroupPost{}, fmt.Errorf("insert group post media: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return GroupPost{}, fmt.Errorf("commit create group post transaction: %w", err)
	}

	cleanupMedia = false

	return GroupPost{
		ID:        groupPostID,
		GroupID:   normalizedGroupID,
		Body:      normalizedInput.Body,
		ImageURL:  firstGroupPostImageURL(s, savedMedia),
		Media:     s.buildGroupPostMedia(savedMedia),
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

func (s Service) loadGroupPostsFromRows(ctx context.Context, rows *sql.Rows, operation string) ([]GroupPost, error) {
	posts := make([]GroupPost, 0)
	postIDs := make([]string, 0)

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
		postIDs = append(postIDs, post.ID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s group posts: %w", operation, err)
	}

	mediaByPostID, err := s.loadGroupPostMedia(ctx, postIDs)
	if err != nil {
		return nil, err
	}

	for index := range posts {
		posts[index].Media = mediaByPostID[posts[index].ID]
		if len(posts[index].Media) == 0 && posts[index].ImageURL != "" {
			posts[index].Media = []GroupPostMedia{
				{
					ID:        posts[index].ID + "-cover",
					URL:       posts[index].ImageURL,
					SortOrder: 1,
				},
			}
		}
	}

	return posts, nil
}

func normalizeCreateGroupPostInput(input CreateGroupPostInput) (CreateGroupPostInput, error) {
	normalized := CreateGroupPostInput{
		Body:   strings.TrimSpace(input.Body),
		Images: input.Images,
	}

	fieldErrors := make(map[string]string)
	if normalized.Body == "" && len(normalized.Images) == 0 {
		fieldErrors["body"] = "Write something or add at least one image for your group."
	} else if len(normalized.Body) > maxGroupPostBodyLength {
		fieldErrors["body"] = "Group posts must be 4000 characters or fewer."
	}
	if len(normalized.Images) > maxPostImages {
		fieldErrors["images"] = "You can upload up to 6 images per group post."
	}

	if len(fieldErrors) > 0 {
		return CreateGroupPostInput{}, &ValidationError{
			Message: "Please correct the group post.",
			Fields:  fieldErrors,
		}
	}

	return normalized, nil
}

func (s Service) loadGroupPostMedia(ctx context.Context, postIDs []string) (map[string][]GroupPostMedia, error) {
	mediaByPostID := make(map[string][]GroupPostMedia, len(postIDs))
	if len(postIDs) == 0 {
		return mediaByPostID, nil
	}

	rows, err := s.db.QueryContext(
		ctx,
		`
			SELECT id, group_post_id, storage_path, sort_order
			FROM group_post_media
			WHERE group_post_id = ANY($1)
			ORDER BY group_post_id, sort_order ASC
		`,
		pq.Array(postIDs),
	)
	if err != nil {
		return nil, fmt.Errorf("query group post media: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var mediaID string
		var postID string
		var storagePath string
		var sortOrder int
		if err := rows.Scan(&mediaID, &postID, &storagePath, &sortOrder); err != nil {
			return nil, fmt.Errorf("scan group post media: %w", err)
		}

		mediaByPostID[postID] = append(mediaByPostID[postID], GroupPostMedia{
			ID:        mediaID,
			URL:       s.publicURL(storagePath),
			SortOrder: sortOrder,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate group post media: %w", err)
	}

	return mediaByPostID, nil
}

func (s Service) saveGroupPostMedia(groupID, postID string, images []*multipart.FileHeader) ([]savedMedia, error) {
	if len(images) == 0 {
		return nil, nil
	}

	postDir := filepath.Join(s.uploadsDir, "groups", groupID, "posts", postID)
	if err := os.MkdirAll(postDir, 0o755); err != nil {
		return nil, fmt.Errorf("create group post uploads directory: %w", err)
	}

	media := make([]savedMedia, 0, len(images))
	for index, header := range images {
		mediaID, err := newToken(16)
		if err != nil {
			return nil, fmt.Errorf("generate group post media id: %w", err)
		}

		fileBaseName := fmt.Sprintf("%02d-%s", index+1, mediaID[:12])
		relativePrefix := filepath.ToSlash(filepath.Join("groups", groupID, "posts", postID, fileBaseName))
		absolutePrefix := filepath.Join(s.uploadsDir, filepath.FromSlash(relativePrefix))

		result, err := uploadmedia.OptimizeAndSaveUpload(header, absolutePrefix, uploadmedia.OptimizeOptions{
			MaxBytes:    maxImageFileSize,
			MaxWidth:    maxPostImageDimension,
			MaxHeight:   maxPostImageDimension,
			JPEGQuality: jpegQuality,
		})
		if err != nil {
			return nil, wrapImageUploadError(err, "images")
		}

		relativePath := relativePrefix + result.Extension
		media = append(media, savedMedia{
			ID:          mediaID,
			StoragePath: relativePath,
			SortOrder:   index + 1,
		})
	}

	return media, nil
}

func (s Service) buildGroupPostMedia(saved []savedMedia) []GroupPostMedia {
	if len(saved) == 0 {
		return []GroupPostMedia{}
	}

	media := make([]GroupPostMedia, 0, len(saved))
	for _, item := range saved {
		media = append(media, GroupPostMedia{
			ID:        item.ID,
			URL:       s.publicURL(item.StoragePath),
			SortOrder: item.SortOrder,
		})
	}

	return media
}

func firstGroupPostImageURL(s Service, saved []savedMedia) string {
	if len(saved) == 0 {
		return ""
	}

	return s.publicURL(saved[0].StoragePath)
}
