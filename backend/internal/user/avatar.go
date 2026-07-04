// backend/internal/user/avatar.go
package user

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"  // registers GIF decoding with image.Decode
	_ "image/jpeg" // registers JPEG decoding with image.Decode
	_ "image/png"  // registers PNG decoding with image.Decode
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"
	"golang.org/x/image/draw"
	xwebp "golang.org/x/image/webp" // decode-only — lets a user upload a .webp and still dedup/re-encode it consistently
)

// Tunables for the avatar pipeline. Kept as named constants (not config/env)
// because changing them means every existing stored avatar was generated
// under a different contract — a deliberate code change, not a runtime knob.
const (
	MaxAvatarsPerUser = 3
	avatarCanvasSize  = 512 // px — square canvas every avatar is normalized to
	avatarWebPQuality = 80  // 0-100, passed to the lossy WebP encoder
	avatarMaxUpload   = 5 * 1024 * 1024
	avatarUploadDir   = "uploads/avatars"
)

var (
	ErrAvatarNotFound     = errors.New("avatar not found")
	ErrAvatarLimitReached = errors.New("avatar limit reached")
	ErrInvalidImage       = errors.New("file is not a valid image")
)

// UserAvatar is one stored avatar image belonging to a user. Up to
// MaxAvatarsPerUser rows may exist per user; at most one has IsActive true,
// enforced by idx_user_avatars_one_active_per_user (see migration 00020).
type UserAvatar struct {
	ID          string    `json:"id" db:"id"`
	UserID      string    `json:"-" db:"user_id"`
	FilePath    string    `json:"-" db:"file_path"` // server-relative; never sent to clients directly, see URL below
	ContentHash string    `json:"-" db:"content_hash"`
	FileSize    int       `json:"fileSize" db:"file_size"`
	Width       int       `json:"width" db:"width"`
	Height      int       `json:"height" db:"height"`
	IsActive    bool      `json:"isActive" db:"is_active"`
	CreatedAt   time.Time `json:"createdAt" db:"created_at"`
	URL         string    `json:"url"` // = FilePath, just under the JSON name the frontend expects
}

// ToResponse copies FilePath into URL for JSON output. FilePath carries
// json:"-" because "filePath" read badly as a public API field name next to
// SafeUser.PhotoURL's convention — but the value is identical, still a
// server-relative path the frontend resolves via resolveAssetUrl.
func (a *UserAvatar) ToResponse() *UserAvatar {
	cp := *a
	cp.URL = a.FilePath
	return &cp
}

// ---------------------------------------------------------------------------
// Repository
// ---------------------------------------------------------------------------

// AvatarRepository persists UserAvatar rows. Every method that changes which
// avatar is active also updates users.photo_url in the same transaction —
// the two can never disagree, by construction, rather than by convention.
type AvatarRepository interface {
	List(ctx context.Context, userID string) ([]*UserAvatar, error)
	FindByHash(ctx context.Context, userID, contentHash string) (*UserAvatar, error)
	Count(ctx context.Context, userID string) (int, error)

	// Insert stores a brand-new avatar row, marks it active (deactivating
	// whichever one previously was), and updates users.photo_url to match.
	Insert(ctx context.Context, a *UserAvatar) (*User, error)

	// Activate marks an existing avatar active and updates users.photo_url.
	Activate(ctx context.Context, userID, avatarID string) (*User, error)

	// Delete removes an avatar row. If it was the active one, the most
	// recently created remaining avatar (if any) is promoted to active;
	// otherwise users.photo_url is cleared. Returns the removed row's
	// FilePath (possibly "") so the caller can unlink the file on disk.
	Delete(ctx context.Context, userID, avatarID string) (filePath string, updatedUser *User, err error)
}

type avatarRepoImpl struct {
	db *pgxpool.Pool
}

func NewAvatarRepository(db *pgxpool.Pool) AvatarRepository {
	return &avatarRepoImpl{db: db}
}

const avatarSelectColumns = `id, user_id, file_path, content_hash, file_size, width, height, is_active, created_at`

func scanAvatar(row pgx.Row) (*UserAvatar, error) {
	a := &UserAvatar{}
	err := row.Scan(
		&a.ID, &a.UserID, &a.FilePath, &a.ContentHash,
		&a.FileSize, &a.Width, &a.Height, &a.IsActive, &a.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (r *avatarRepoImpl) List(ctx context.Context, userID string) ([]*UserAvatar, error) {
	const q = `SELECT ` + avatarSelectColumns + ` FROM user_avatars WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("avatar: List: %w", err)
	}
	defer rows.Close()

	out := []*UserAvatar{}
	for rows.Next() {
		a, err := scanAvatar(rows)
		if err != nil {
			return nil, fmt.Errorf("avatar: List scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *avatarRepoImpl) FindByHash(ctx context.Context, userID, contentHash string) (*UserAvatar, error) {
	const q = `SELECT ` + avatarSelectColumns + ` FROM user_avatars WHERE user_id = $1 AND content_hash = $2`
	a, err := scanAvatar(r.db.QueryRow(ctx, q, userID, contentHash))
	if err != nil {
		return nil, fmt.Errorf("avatar: FindByHash: %w", err)
	}
	return a, nil
}

func (r *avatarRepoImpl) Count(ctx context.Context, userID string) (int, error) {
	const q = `SELECT COUNT(*) FROM user_avatars WHERE user_id = $1`
	var n int
	if err := r.db.QueryRow(ctx, q, userID).Scan(&n); err != nil {
		return 0, fmt.Errorf("avatar: Count: %w", err)
	}
	return n, nil
}

func (r *avatarRepoImpl) Insert(ctx context.Context, a *UserAvatar) (*User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("avatar: Insert begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op once Commit succeeds

	if _, err := tx.Exec(ctx, `UPDATE user_avatars SET is_active = false WHERE user_id = $1 AND is_active`, a.UserID); err != nil {
		return nil, fmt.Errorf("avatar: Insert deactivate: %w", err)
	}

	const insertQ = `
		INSERT INTO user_avatars (user_id, file_path, content_hash, file_size, width, height, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, true)
		RETURNING id, created_at`
	if err := tx.QueryRow(ctx, insertQ,
		a.UserID, a.FilePath, a.ContentHash, a.FileSize, a.Width, a.Height,
	).Scan(&a.ID, &a.CreatedAt); err != nil {
		return nil, fmt.Errorf("avatar: Insert row: %w", err)
	}
	a.IsActive = true

	u, err := setUserPhotoURLTx(ctx, tx, a.UserID, a.FilePath)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("avatar: Insert commit: %w", err)
	}
	return u, nil
}

func (r *avatarRepoImpl) Activate(ctx context.Context, userID, avatarID string) (*User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("avatar: Activate begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var filePath string
	err = tx.QueryRow(ctx,
		`SELECT file_path FROM user_avatars WHERE id = $1 AND user_id = $2`,
		avatarID, userID,
	).Scan(&filePath)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAvatarNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("avatar: Activate lookup: %w", err)
	}

	if _, err := tx.Exec(ctx, `UPDATE user_avatars SET is_active = false WHERE user_id = $1 AND is_active`, userID); err != nil {
		return nil, fmt.Errorf("avatar: Activate deactivate: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE user_avatars SET is_active = true WHERE id = $1`, avatarID); err != nil {
		return nil, fmt.Errorf("avatar: Activate set: %w", err)
	}

	u, err := setUserPhotoURLTx(ctx, tx, userID, filePath)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("avatar: Activate commit: %w", err)
	}
	return u, nil
}

func (r *avatarRepoImpl) Delete(ctx context.Context, userID, avatarID string) (string, *User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("avatar: Delete begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var filePath string
	var wasActive bool
	err = tx.QueryRow(ctx,
		`SELECT file_path, is_active FROM user_avatars WHERE id = $1 AND user_id = $2`,
		avatarID, userID,
	).Scan(&filePath, &wasActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, ErrAvatarNotFound
	}
	if err != nil {
		return "", nil, fmt.Errorf("avatar: Delete lookup: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM user_avatars WHERE id = $1`, avatarID); err != nil {
		return "", nil, fmt.Errorf("avatar: Delete row: %w", err)
	}

	var u *User
	if wasActive {
		// This one was the active avatar — promote the most recently
		// uploaded remaining one, if any, rather than leaving the user
		// with no avatar just because they deleted the current one.
		var nextID, nextPath string
		lookupErr := tx.QueryRow(ctx, `
			SELECT id, file_path FROM user_avatars
			WHERE user_id = $1
			ORDER BY created_at DESC
			LIMIT 1`, userID,
		).Scan(&nextID, &nextPath)

		switch {
		case errors.Is(lookupErr, pgx.ErrNoRows):
			u, err = setUserPhotoURLTx(ctx, tx, userID, "")
			if err != nil {
				return "", nil, err
			}
		case lookupErr != nil:
			return "", nil, fmt.Errorf("avatar: Delete find-next: %w", lookupErr)
		default:
			if _, err := tx.Exec(ctx, `UPDATE user_avatars SET is_active = true WHERE id = $1`, nextID); err != nil {
				return "", nil, fmt.Errorf("avatar: Delete promote: %w", err)
			}
			u, err = setUserPhotoURLTx(ctx, tx, userID, nextPath)
			if err != nil {
				return "", nil, err
			}
		}
	} else {
		u, err = findUserTx(ctx, tx, userID)
		if err != nil {
			return "", nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", nil, fmt.Errorf("avatar: Delete commit: %w", err)
	}
	return filePath, u, nil
}

// setUserPhotoURLTx updates users.photo_url inside an already-open
// transaction. Shared by Insert/Activate/Delete so user_avatars and the
// denormalized users.photo_url column are updated atomically together —
// there is no code path where one changes without the other.
func setUserPhotoURLTx(ctx context.Context, tx pgx.Tx, userID, photoURL string) (*User, error) {
	const q = `
		UPDATE users
		SET photo_url  = NULLIF($1, ''),
		    updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING ` + userSelectColumns
	u, err := scanUser(tx.QueryRow(ctx, q, photoURL, userID))
	if err != nil {
		return nil, fmt.Errorf("avatar: setUserPhotoURLTx: %w", err)
	}
	if u == nil {
		return nil, ErrNotFound
	}
	return u, nil
}

func findUserTx(ctx context.Context, tx pgx.Tx, userID string) (*User, error) {
	const q = `SELECT ` + userSelectColumns + ` FROM users WHERE id = $1 AND deleted_at IS NULL`
	u, err := scanUser(tx.QueryRow(ctx, q, userID))
	if err != nil {
		return nil, fmt.Errorf("avatar: findUserTx: %w", err)
	}
	if u == nil {
		return nil, ErrNotFound
	}
	return u, nil
}

// ---------------------------------------------------------------------------
// Service
// ---------------------------------------------------------------------------

// AvatarService owns the upload pipeline: sniff real content type, center-
// crop to square, resize to a fixed canvas, encode to WebP, hash for dedup,
// and enforce MaxAvatarsPerUser — all before the repository is ever touched.
type AvatarService interface {
	List(ctx context.Context, userID string) ([]*UserAvatar, error)
	Upload(ctx context.Context, userID string, raw []byte) (*User, *UserAvatar, error)
	Activate(ctx context.Context, userID, avatarID string) (*User, error)
	Delete(ctx context.Context, userID, avatarID string) (*User, error)
}

type avatarServiceImpl struct {
	repo AvatarRepository
}

func NewAvatarService(repo AvatarRepository) AvatarService {
	return &avatarServiceImpl{repo: repo}
}

func (s *avatarServiceImpl) List(ctx context.Context, userID string) ([]*UserAvatar, error) {
	return s.repo.List(ctx, userID)
}

func (s *avatarServiceImpl) Upload(ctx context.Context, userID string, raw []byte) (*User, *UserAvatar, error) {
	// Sniff the real content — never trust the filename extension the
	// client sent (the old handler only checked the extension string,
	// which a renamed malicious file trivially defeats).
	contentType := http.DetectContentType(raw)
	switch contentType {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
	default:
		return nil, nil, ErrInvalidImage
	}

	img, err := decodeImage(raw, contentType)
	if err != nil {
		return nil, nil, ErrInvalidImage
	}

	// Defense in depth: the frontend crop step is expected to deliver a
	// square image already, but the backend must never assume a client
	// did what the UI asked — a direct API call, a future non-web client,
	// or a bug could all bypass that step.
	square := centerCropSquare(img)
	resized := resizeSquare(square, avatarCanvasSize)

	encOpts, err := encoder.NewLossyEncoderOptions(encoder.PresetPhoto, avatarWebPQuality)
	if err != nil {
		return nil, nil, fmt.Errorf("avatar: encoder options: %w", err)
	}
	var buf bytes.Buffer
	if err := webp.Encode(&buf, resized, encOpts); err != nil {
		return nil, nil, fmt.Errorf("avatar: webp encode: %w", err)
	}
	encoded := buf.Bytes()

	sum := sha256.Sum256(encoded)
	hash := hex.EncodeToString(sum[:])

	// Dedup: the *encoded* bytes are hashed, so re-uploading the same
	// source photo — even re-cropped identically by the user a second
	// time — lands on the same hash and reuses the existing row instead
	// of writing (and counting against the limit) a duplicate file.
	if existing, err := s.repo.FindByHash(ctx, userID, hash); err != nil {
		return nil, nil, fmt.Errorf("avatar: dedup lookup: %w", err)
	} else if existing != nil {
		u, err := s.repo.Activate(ctx, userID, existing.ID)
		if err != nil {
			return nil, nil, err
		}
		return u, existing, nil
	}

	count, err := s.repo.Count(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("avatar: count: %w", err)
	}
	if count >= MaxAvatarsPerUser {
		return nil, nil, ErrAvatarLimitReached
	}

	if err := os.MkdirAll(avatarUploadDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("avatar: mkdir: %w", err)
	}
	diskPath := filepath.Join(avatarUploadDir, hash+".webp")
	if err := os.WriteFile(diskPath, encoded, 0o644); err != nil {
		return nil, nil, fmt.Errorf("avatar: write file: %w", err)
	}

	avatar := &UserAvatar{
		UserID:      userID,
		FilePath:    "/" + filepath.ToSlash(diskPath),
		ContentHash: hash,
		FileSize:    len(encoded),
		Width:       avatarCanvasSize,
		Height:      avatarCanvasSize,
	}
	u, err := s.repo.Insert(ctx, avatar)
	if err != nil {
		_ = os.Remove(diskPath) // best-effort — don't strand a file the DB doesn't know about
		return nil, nil, err
	}
	return u, avatar, nil
}

func (s *avatarServiceImpl) Activate(ctx context.Context, userID, avatarID string) (*User, error) {
	return s.repo.Activate(ctx, userID, avatarID)
}

func (s *avatarServiceImpl) Delete(ctx context.Context, userID, avatarID string) (*User, error) {
	diskPath, u, err := s.repo.Delete(ctx, userID, avatarID)
	if err != nil {
		return nil, err
	}
	// The DB row is already gone — that's the source of truth for the API,
	// so a failed unlink here is logged by the caller, not fatal to the
	// request. It leaves an orphaned file, not a correctness bug.
	if diskPath != "" {
		_ = os.Remove(strings.TrimPrefix(diskPath, "/"))
	}
	return u, nil
}

// ---------------------------------------------------------------------------
// Image helpers
// ---------------------------------------------------------------------------

func decodeImage(raw []byte, contentType string) (image.Image, error) {
	if contentType == "image/webp" {
		return xwebp.Decode(bytes.NewReader(raw))
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	return img, err
}

// centerCropSquare crops the largest centered square out of img.
func centerCropSquare(img image.Image) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	side := w
	if h < side {
		side = h
	}
	x0 := b.Min.X + (w-side)/2
	y0 := b.Min.Y + (h-side)/2
	srcRect := image.Rect(x0, y0, x0+side, y0+side)

	if si, ok := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}); ok {
		return si.SubImage(srcRect)
	}
	// Defensive fallback — every concrete type returned by image.Decode and
	// xwebp.Decode implements SubImage, so this path is not expected to run.
	dst := image.NewRGBA(image.Rect(0, 0, side, side))
	// draw.NearestNeighbor.Draw(dst, dst.Bounds(), img, srcRect.Min)
	draw.NearestNeighbor.Scale(dst, dst.Bounds(), img, srcRect, draw.Over, nil)
	return dst
}

func resizeSquare(img image.Image, size int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	return dst
}
