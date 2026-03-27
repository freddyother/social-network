package social

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"

	"social-network/backend/internal/auth"
)

const (
	maxPostImages    = 6
	maxImageFileSize = 10 << 20
)

var (
	ErrForbidden       = errors.New("forbidden")
	ErrNotFound        = errors.New("not found")
	ErrFollowYourself  = errors.New("cannot follow yourself")
	ErrAlreadyHandled  = errors.New("follow request already handled")
	ErrInvalidResponse = errors.New("invalid response")
)

type ValidationError struct {
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func (e *ValidationError) Error() string {
	if e == nil || e.Message == "" {
		return "validation failed"
	}

	return e.Message
}

type CreatePostInput struct {
	Title   string
	Body    string
	Privacy string
	Images  []*multipart.FileHeader
}

type PostAuthor struct {
	ID                string `json:"id"`
	FirstName         string `json:"firstName"`
	LastName          string `json:"lastName"`
	Nickname          string `json:"nickname,omitempty"`
	ProfileVisibility string `json:"profileVisibility"`
}

type PostMedia struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	SortOrder int    `json:"sortOrder"`
}

type Post struct {
	ID        string      `json:"id"`
	Title     string      `json:"title"`
	Body      string      `json:"body"`
	Privacy   string      `json:"privacy"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
	Author    PostAuthor  `json:"author"`
	Media     []PostMedia `json:"media"`
}

type SuggestedUser struct {
	ID                 string `json:"id"`
	FirstName          string `json:"firstName"`
	LastName           string `json:"lastName"`
	Nickname           string `json:"nickname,omitempty"`
	AboutMe            string `json:"aboutMe,omitempty"`
	ProfileVisibility  string `json:"profileVisibility"`
	RelationshipStatus string `json:"relationshipStatus"`
}

type FollowRequest struct {
	ID        string        `json:"id"`
	CreatedAt time.Time     `json:"createdAt"`
	Sender    SuggestedUser `json:"sender"`
}

type FollowActionResult struct {
	Status string `json:"status"`
}

type Service struct {
	db            *sql.DB
	uploadsDir    string
	publicBaseURL string
}

func NewService(db *sql.DB, uploadsDir, publicBaseURL string) Service {
	trimmedBaseURL := strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	if trimmedBaseURL == "" {
		trimmedBaseURL = "http://localhost:8080"
	}

	trimmedUploadsDir := strings.TrimSpace(uploadsDir)
	if trimmedUploadsDir == "" {
		trimmedUploadsDir = "./uploads"
	}

	return Service{
		db:            db,
		uploadsDir:    trimmedUploadsDir,
		publicBaseURL: trimmedBaseURL,
	}
}

func (s Service) CreatePost(ctx context.Context, author auth.User, input CreatePostInput) (Post, error) {
	normalizedInput, err := normalizeCreatePostInput(input)
	if err != nil {
		return Post{}, err
	}

	postID, err := newToken(16)
	if err != nil {
		return Post{}, fmt.Errorf("generate post id: %w", err)
	}

	savedMedia, err := s.savePostMedia(postID, normalizedInput.Images)
	if err != nil {
		return Post{}, err
	}

	cleanupMedia := len(savedMedia) > 0
	defer func() {
		if cleanupMedia {
			_ = os.RemoveAll(filepath.Join(s.uploadsDir, "posts", postID))
		}
	}()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Post{}, fmt.Errorf("begin create post transaction: %w", err)
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
			INSERT INTO posts (id, author_id, title, body, image_url, privacy)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING created_at, updated_at
		`,
		postID,
		author.ID,
		normalizedInput.Title,
		normalizedInput.Body,
		coverPath,
		normalizedInput.Privacy,
	).Scan(&createdAt, &updatedAt); err != nil {
		return Post{}, fmt.Errorf("insert post: %w", err)
	}

	for _, media := range savedMedia {
		if _, err = tx.ExecContext(
			ctx,
			`
				INSERT INTO post_media (id, post_id, storage_path, sort_order)
				VALUES ($1, $2, $3, $4)
			`,
			media.ID,
			postID,
			media.StoragePath,
			media.SortOrder,
		); err != nil {
			return Post{}, fmt.Errorf("insert post media: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return Post{}, fmt.Errorf("commit create post transaction: %w", err)
	}

	cleanupMedia = false

	return Post{
		ID:        postID,
		Title:     normalizedInput.Title,
		Body:      normalizedInput.Body,
		Privacy:   normalizedInput.Privacy,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Author: PostAuthor{
			ID:                author.ID,
			FirstName:         author.FirstName,
			LastName:          author.LastName,
			Nickname:          author.Nickname,
			ProfileVisibility: author.ProfileVisibility,
		},
		Media: s.buildPostMedia(savedMedia),
	}, nil
}

func (s Service) Feed(ctx context.Context, viewerID string) ([]Post, error) {
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
			ORDER BY p.created_at DESC
			LIMIT 50
		`,
		viewerID,
	)
	if err != nil {
		return nil, fmt.Errorf("query feed: %w", err)
	}
	defer rows.Close()

	posts := make([]Post, 0)
	postIDs := make([]string, 0)

	for rows.Next() {
		var item Post
		var nickname sql.NullString
		if err := rows.Scan(
			&item.ID,
			&item.Title,
			&item.Body,
			&item.Privacy,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.Author.ID,
			&item.Author.FirstName,
			&item.Author.LastName,
			&nickname,
			&item.Author.ProfileVisibility,
		); err != nil {
			return nil, fmt.Errorf("scan feed post: %w", err)
		}

		item.Author.Nickname = nullStringValue(nickname)
		posts = append(posts, item)
		postIDs = append(postIDs, item.ID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate feed posts: %w", err)
	}

	mediaByPostID, err := s.loadPostMedia(ctx, postIDs)
	if err != nil {
		return nil, err
	}

	for index := range posts {
		posts[index].Media = mediaByPostID[posts[index].ID]
	}

	return posts, nil
}

func (s Service) DiscoverUsers(ctx context.Context, viewerID string) ([]SuggestedUser, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`
			SELECT
				u.id,
				u.first_name,
				u.last_name,
				u.nickname,
				u.about_me,
				u.profile_visibility,
				CASE
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
			WHERE u.id <> $1
			ORDER BY u.created_at DESC
			LIMIT 30
		`,
		viewerID,
	)
	if err != nil {
		return nil, fmt.Errorf("query discover users: %w", err)
	}
	defer rows.Close()

	users := make([]SuggestedUser, 0)
	for rows.Next() {
		var user SuggestedUser
		var nickname sql.NullString
		var aboutMe sql.NullString
		if err := rows.Scan(
			&user.ID,
			&user.FirstName,
			&user.LastName,
			&nickname,
			&aboutMe,
			&user.ProfileVisibility,
			&user.RelationshipStatus,
		); err != nil {
			return nil, fmt.Errorf("scan discover user: %w", err)
		}

		user.Nickname = nullStringValue(nickname)
		user.AboutMe = nullStringValue(aboutMe)
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate discover users: %w", err)
	}

	return users, nil
}

func (s Service) FollowUser(ctx context.Context, followerID, followeeID string) (FollowActionResult, error) {
	if followerID == followeeID {
		return FollowActionResult{}, ErrFollowYourself
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return FollowActionResult{}, fmt.Errorf("begin follow transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var profileVisibility string
	if err = tx.QueryRowContext(ctx, `SELECT profile_visibility FROM users WHERE id = $1`, followeeID).Scan(&profileVisibility); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return FollowActionResult{}, ErrNotFound
		}

		return FollowActionResult{}, fmt.Errorf("load followee: %w", err)
	}

	var alreadyFollowing bool
	if err = tx.QueryRowContext(
		ctx,
		`SELECT EXISTS (SELECT 1 FROM followers WHERE follower_id = $1 AND followee_id = $2)`,
		followerID,
		followeeID,
	).Scan(&alreadyFollowing); err != nil {
		return FollowActionResult{}, fmt.Errorf("check following relationship: %w", err)
	}

	if alreadyFollowing {
		if err = tx.Commit(); err != nil {
			return FollowActionResult{}, fmt.Errorf("commit follow transaction: %w", err)
		}

		return FollowActionResult{Status: "following"}, nil
	}

	if profileVisibility == "public" {
		if _, err = tx.ExecContext(
			ctx,
			`
				INSERT INTO followers (follower_id, followee_id)
				VALUES ($1, $2)
				ON CONFLICT (follower_id, followee_id) DO NOTHING
			`,
			followerID,
			followeeID,
		); err != nil {
			return FollowActionResult{}, fmt.Errorf("insert follower: %w", err)
		}

		if _, err = tx.ExecContext(
			ctx,
			`DELETE FROM follow_requests WHERE sender_id = $1 AND recipient_id = $2`,
			followerID,
			followeeID,
		); err != nil {
			return FollowActionResult{}, fmt.Errorf("delete obsolete follow request: %w", err)
		}

		if err = tx.Commit(); err != nil {
			return FollowActionResult{}, fmt.Errorf("commit follow transaction: %w", err)
		}

		return FollowActionResult{Status: "following"}, nil
	}

	requestID, err := newToken(16)
	if err != nil {
		return FollowActionResult{}, fmt.Errorf("generate follow request id: %w", err)
	}

	if _, err = tx.ExecContext(
		ctx,
		`
			INSERT INTO follow_requests (id, sender_id, recipient_id, status, created_at, responded_at)
			VALUES ($1, $2, $3, 'pending', NOW(), NULL)
			ON CONFLICT (sender_id, recipient_id)
			DO UPDATE SET
				status = 'pending',
				responded_at = NULL,
				created_at = NOW()
		`,
		requestID,
		followerID,
		followeeID,
	); err != nil {
		return FollowActionResult{}, fmt.Errorf("upsert follow request: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return FollowActionResult{}, fmt.Errorf("commit follow transaction: %w", err)
	}

	return FollowActionResult{Status: "requested"}, nil
}

func (s Service) IncomingFollowRequests(ctx context.Context, userID string) ([]FollowRequest, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`
			SELECT
				fr.id,
				fr.created_at,
				u.id,
				u.first_name,
				u.last_name,
				u.nickname,
				u.about_me,
				u.profile_visibility
			FROM follow_requests fr
			INNER JOIN users u ON u.id = fr.sender_id
			WHERE fr.recipient_id = $1 AND fr.status = 'pending'
			ORDER BY fr.created_at DESC
		`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query follow requests: %w", err)
	}
	defer rows.Close()

	requests := make([]FollowRequest, 0)
	for rows.Next() {
		var request FollowRequest
		var nickname sql.NullString
		var aboutMe sql.NullString
		if err := rows.Scan(
			&request.ID,
			&request.CreatedAt,
			&request.Sender.ID,
			&request.Sender.FirstName,
			&request.Sender.LastName,
			&nickname,
			&aboutMe,
			&request.Sender.ProfileVisibility,
		); err != nil {
			return nil, fmt.Errorf("scan follow request: %w", err)
		}

		request.Sender.Nickname = nullStringValue(nickname)
		request.Sender.AboutMe = nullStringValue(aboutMe)
		request.Sender.RelationshipStatus = "requested"
		requests = append(requests, request)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate follow requests: %w", err)
	}

	return requests, nil
}

func (s Service) RespondToFollowRequest(ctx context.Context, userID, requestID string, accept bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin follow response transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var senderID string
	var status string
	if err = tx.QueryRowContext(
		ctx,
		`
			SELECT sender_id, status
			FROM follow_requests
			WHERE id = $1 AND recipient_id = $2
		`,
		requestID,
		userID,
	).Scan(&senderID, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}

		return fmt.Errorf("load follow request: %w", err)
	}

	if status != "pending" {
		return ErrAlreadyHandled
	}

	newStatus := "declined"
	if accept {
		newStatus = "accepted"
		if _, err = tx.ExecContext(
			ctx,
			`
				INSERT INTO followers (follower_id, followee_id)
				VALUES ($1, $2)
				ON CONFLICT (follower_id, followee_id) DO NOTHING
			`,
			senderID,
			userID,
		); err != nil {
			return fmt.Errorf("insert accepted follower: %w", err)
		}
	}

	if _, err = tx.ExecContext(
		ctx,
		`
			UPDATE follow_requests
			SET status = $1, responded_at = NOW()
			WHERE id = $2
		`,
		newStatus,
		requestID,
	); err != nil {
		return fmt.Errorf("update follow request: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit follow response transaction: %w", err)
	}

	return nil
}

func (s Service) UpdateProfileVisibility(ctx context.Context, userID, visibility string) (auth.User, error) {
	normalizedVisibility, err := normalizeProfileVisibility(visibility)
	if err != nil {
		return auth.User{}, err
	}

	row := s.db.QueryRowContext(
		ctx,
		`
			UPDATE users
			SET profile_visibility = $1, updated_at = NOW()
			WHERE id = $2
			RETURNING
				id,
				email,
				first_name,
				last_name,
				date_of_birth,
				avatar_url,
				nickname,
				about_me,
				profile_visibility,
				created_at,
				updated_at
		`,
		normalizedVisibility,
		userID,
	)

	user, err := scanAuthUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.User{}, ErrNotFound
		}

		return auth.User{}, fmt.Errorf("update profile visibility: %w", err)
	}

	return user, nil
}

func (s Service) loadPostMedia(ctx context.Context, postIDs []string) (map[string][]PostMedia, error) {
	mediaByPostID := make(map[string][]PostMedia, len(postIDs))
	if len(postIDs) == 0 {
		return mediaByPostID, nil
	}

	rows, err := s.db.QueryContext(
		ctx,
		`
			SELECT id, post_id, storage_path, sort_order
			FROM post_media
			WHERE post_id = ANY($1)
			ORDER BY post_id, sort_order ASC
		`,
		pq.Array(postIDs),
	)
	if err != nil {
		return nil, fmt.Errorf("query post media: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var mediaID string
		var postID string
		var storagePath string
		var sortOrder int
		if err := rows.Scan(&mediaID, &postID, &storagePath, &sortOrder); err != nil {
			return nil, fmt.Errorf("scan post media: %w", err)
		}

		mediaByPostID[postID] = append(mediaByPostID[postID], PostMedia{
			ID:        mediaID,
			URL:       s.publicURL(storagePath),
			SortOrder: sortOrder,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate post media: %w", err)
	}

	return mediaByPostID, nil
}

func (s Service) savePostMedia(postID string, images []*multipart.FileHeader) ([]savedMedia, error) {
	if len(images) == 0 {
		return nil, nil
	}

	postDir := filepath.Join(s.uploadsDir, "posts", postID)
	if err := os.MkdirAll(postDir, 0o755); err != nil {
		return nil, fmt.Errorf("create post uploads directory: %w", err)
	}

	media := make([]savedMedia, 0, len(images))
	for index, header := range images {
		if header.Size > maxImageFileSize {
			return nil, &ValidationError{
				Message: "Each image must be 10 MB or smaller.",
				Fields: map[string]string{
					"images": "Each image must be 10 MB or smaller.",
				},
			}
		}

		file, err := header.Open()
		if err != nil {
			return nil, fmt.Errorf("open uploaded image: %w", err)
		}

		extension, err := imageExtension(file)
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if err != nil {
			return nil, err
		}

		mediaID, err := newToken(16)
		if err != nil {
			return nil, fmt.Errorf("generate media id: %w", err)
		}

		fileName := fmt.Sprintf("%02d-%s%s", index+1, mediaID[:12], extension)
		relativePath := filepath.ToSlash(filepath.Join("posts", postID, fileName))
		absolutePath := filepath.Join(s.uploadsDir, relativePath)

		sourceFile, err := header.Open()
		if err != nil {
			return nil, fmt.Errorf("reopen uploaded image: %w", err)
		}

		if err := copyFile(absolutePath, sourceFile); err != nil {
			_ = sourceFile.Close()
			return nil, err
		}

		if err := sourceFile.Close(); err != nil {
			return nil, fmt.Errorf("close uploaded image: %w", err)
		}

		media = append(media, savedMedia{
			ID:          mediaID,
			StoragePath: relativePath,
			SortOrder:   index + 1,
		})
	}

	sort.Slice(media, func(i, j int) bool {
		return media[i].SortOrder < media[j].SortOrder
	})

	return media, nil
}

func (s Service) buildPostMedia(saved []savedMedia) []PostMedia {
	if len(saved) == 0 {
		return []PostMedia{}
	}

	media := make([]PostMedia, 0, len(saved))
	for _, item := range saved {
		media = append(media, PostMedia{
			ID:        item.ID,
			URL:       s.publicURL(item.StoragePath),
			SortOrder: item.SortOrder,
		})
	}

	return media
}

func (s Service) publicURL(storagePath string) string {
	normalizedPath := strings.TrimLeft(filepath.ToSlash(storagePath), "/")
	return s.publicBaseURL + "/uploads/" + normalizedPath
}

type savedMedia struct {
	ID          string
	StoragePath string
	SortOrder   int
}

func normalizeCreatePostInput(input CreatePostInput) (CreatePostInput, error) {
	normalized := CreatePostInput{
		Title:   strings.TrimSpace(input.Title),
		Body:    strings.TrimSpace(input.Body),
		Privacy: strings.ToLower(strings.TrimSpace(input.Privacy)),
		Images:  input.Images,
	}

	fieldErrors := make(map[string]string)
	if normalized.Title == "" {
		fieldErrors["title"] = "Title is required."
	} else if len(normalized.Title) > 120 {
		fieldErrors["title"] = "Title must be 120 characters or fewer."
	}

	if normalized.Body == "" {
		fieldErrors["body"] = "Caption is required."
	} else if len(normalized.Body) > 3000 {
		fieldErrors["body"] = "Caption must be 3000 characters or fewer."
	}

	if normalized.Privacy == "" {
		normalized.Privacy = "public"
	}

	if normalized.Privacy != "public" && normalized.Privacy != "followers" {
		fieldErrors["privacy"] = "Privacy must be public or followers."
	}

	if len(normalized.Images) > maxPostImages {
		fieldErrors["images"] = "You can upload up to 6 images per post."
	}

	if len(fieldErrors) > 0 {
		return CreatePostInput{}, &ValidationError{
			Message: "Please correct the post details.",
			Fields:  fieldErrors,
		}
	}

	return normalized, nil
}

func normalizeProfileVisibility(visibility string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(visibility))
	if normalized != "public" && normalized != "private" {
		return "", &ValidationError{
			Message: "Profile visibility must be public or private.",
			Fields: map[string]string{
				"visibility": "Profile visibility must be public or private.",
			},
		}
	}

	return normalized, nil
}

func imageExtension(file multipart.File) (string, error) {
	sniffBuffer := make([]byte, 512)
	bytesRead, err := file.Read(sniffBuffer)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read uploaded image: %w", err)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewind uploaded image: %w", err)
	}

	contentType := http.DetectContentType(sniffBuffer[:bytesRead])
	switch contentType {
	case "image/jpeg":
		return ".jpg", nil
	case "image/png":
		return ".png", nil
	case "image/webp":
		return ".webp", nil
	case "image/gif":
		return ".gif", nil
	default:
		return "", &ValidationError{
			Message: "Only JPG, PNG, GIF, and WebP images are supported.",
			Fields: map[string]string{
				"images": "Only JPG, PNG, GIF, and WebP images are supported.",
			},
		}
	}
}

func copyFile(destinationPath string, source multipart.File) error {
	destination, err := os.Create(destinationPath)
	if err != nil {
		return fmt.Errorf("create destination image: %w", err)
	}
	defer destination.Close()

	if _, err := io.Copy(destination, source); err != nil {
		return fmt.Errorf("copy uploaded image: %w", err)
	}

	return nil
}

func newToken(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}

	return value.String
}

type authUserScanner interface {
	Scan(dest ...any) error
}

func scanAuthUser(row authUserScanner) (auth.User, error) {
	var user auth.User
	var dateOfBirth time.Time
	var avatarURL sql.NullString
	var nickname sql.NullString
	var aboutMe sql.NullString

	if err := row.Scan(
		&user.ID,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&dateOfBirth,
		&avatarURL,
		&nickname,
		&aboutMe,
		&user.ProfileVisibility,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		return auth.User{}, err
	}

	user.DateOfBirth = dateOfBirth.Format("2006-01-02")
	user.AvatarURL = nullStringValue(avatarURL)
	user.Nickname = nullStringValue(nickname)
	user.AboutMe = nullStringValue(aboutMe)
	return user, nil
}
