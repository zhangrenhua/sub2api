package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// imageWorkbenchRepository 画图工作台记录仓储(raw SQL,fork 功能)。
type imageWorkbenchRepository struct {
	db *sql.DB
}

// NewImageWorkbenchRepository 构造画图工作台仓储。
func NewImageWorkbenchRepository(db *sql.DB) service.ImageWorkbenchRepository {
	return &imageWorkbenchRepository{db: db}
}

const imageWorkbenchColumns = `id, user_id, session_id, prompt, revised_prompt, model, size, quality, storage, object_key, token, mime, bytes, width, height, created_at, expires_at`

func scanImageWorkbenchRecord(scanner interface {
	Scan(dest ...any) error
}) (*service.ImageWorkbenchRecord, error) {
	rec := &service.ImageWorkbenchRecord{}
	if err := scanner.Scan(
		&rec.ID, &rec.UserID, &rec.SessionID, &rec.Prompt, &rec.RevisedPrompt,
		&rec.Model, &rec.Size, &rec.Quality, &rec.Storage, &rec.ObjectKey, &rec.Token,
		&rec.Mime, &rec.Bytes, &rec.Width, &rec.Height, &rec.CreatedAt, &rec.ExpiresAt,
	); err != nil {
		return nil, err
	}
	return rec, nil
}

func (r *imageWorkbenchRepository) Create(ctx context.Context, rec *service.ImageWorkbenchRecord) (int64, error) {
	const q = `
		INSERT INTO image_workbench_images
			(user_id, session_id, prompt, revised_prompt, model, size, quality, storage, object_key, token, mime, bytes, width, height, created_at, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		RETURNING id`
	var id int64
	err := r.db.QueryRowContext(ctx, q,
		rec.UserID, rec.SessionID, rec.Prompt, rec.RevisedPrompt, rec.Model,
		rec.Size, rec.Quality, rec.Storage, rec.ObjectKey, rec.Token, rec.Mime,
		rec.Bytes, rec.Width, rec.Height, rec.CreatedAt, rec.ExpiresAt,
	).Scan(&id)
	return id, err
}

func (r *imageWorkbenchRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*service.ImageWorkbenchRecord, error) {
	const q = `SELECT ` + imageWorkbenchColumns + `
		FROM image_workbench_images
		WHERE user_id = $1 AND expires_at > now()
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.db.QueryContext(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]*service.ImageWorkbenchRecord, 0, limit)
	for rows.Next() {
		rec, err := scanImageWorkbenchRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *imageWorkbenchRepository) GetByID(ctx context.Context, id int64) (*service.ImageWorkbenchRecord, error) {
	const q = `SELECT ` + imageWorkbenchColumns + ` FROM image_workbench_images WHERE id = $1`
	return scanImageWorkbenchRecord(r.db.QueryRowContext(ctx, q, id))
}

func (r *imageWorkbenchRepository) GetByToken(ctx context.Context, token string) (*service.ImageWorkbenchRecord, error) {
	const q = `SELECT ` + imageWorkbenchColumns + ` FROM image_workbench_images WHERE token = $1 LIMIT 1`
	return scanImageWorkbenchRecord(r.db.QueryRowContext(ctx, q, token))
}

func (r *imageWorkbenchRepository) Delete(ctx context.Context, id, userID int64) (string, error) {
	const q = `DELETE FROM image_workbench_images WHERE id = $1 AND user_id = $2 RETURNING object_key`
	var objectKey string
	err := r.db.QueryRowContext(ctx, q, id, userID).Scan(&objectKey)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return objectKey, err
}

func (r *imageWorkbenchRepository) DeleteOverLimit(ctx context.Context, userID int64, keep int) ([]string, error) {
	const q = `
		DELETE FROM image_workbench_images
		WHERE id IN (
			SELECT id FROM image_workbench_images
			WHERE user_id = $1
			ORDER BY created_at DESC, id DESC
			OFFSET $2
		)
		RETURNING object_key`
	rows, err := r.db.QueryContext(ctx, q, userID, keep)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (r *imageWorkbenchRepository) DeleteExpired(ctx context.Context, now time.Time, limit int) ([]string, error) {
	const q = `
		DELETE FROM image_workbench_images
		WHERE id IN (
			SELECT id FROM image_workbench_images WHERE expires_at < $1 ORDER BY expires_at LIMIT $2
		)
		RETURNING object_key`
	rows, err := r.db.QueryContext(ctx, q, now, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (r *imageWorkbenchRepository) GetByIDs(ctx context.Context, ids []int64) ([]*service.ImageWorkbenchRecord, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	q := `SELECT ` + imageWorkbenchColumns + ` FROM image_workbench_images WHERE id = ANY($1)`
	rows, err := r.db.QueryContext(ctx, q, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*service.ImageWorkbenchRecord
	for rows.Next() {
		rec, err := scanImageWorkbenchRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// ---- 任务 ----

const imageWorkbenchTaskColumns = `id, user_id, api_key_id, status, prompt, model, size, n, base_image_id, base_object_keys, result_image_ids, error, created_at, updated_at`

func scanImageWorkbenchTask(scanner interface {
	Scan(dest ...any) error
}) (*service.ImageWorkbenchTask, error) {
	t := &service.ImageWorkbenchTask{}
	var baseKeys, resultIDs []byte
	if err := scanner.Scan(
		&t.ID, &t.UserID, &t.APIKeyID, &t.Status, &t.Prompt, &t.Model, &t.Size, &t.N,
		&t.BaseImageID, &baseKeys, &resultIDs, &t.Error, &t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return nil, err
	}
	_ = json.Unmarshal(baseKeys, &t.BaseObjectKeys)
	_ = json.Unmarshal(resultIDs, &t.ResultImageIDs)
	return t, nil
}

func jsonStrings(s []string) string {
	if s == nil {
		s = []string{}
	}
	b, _ := json.Marshal(s)
	return string(b)
}
func jsonInts(s []int64) string {
	if s == nil {
		s = []int64{}
	}
	b, _ := json.Marshal(s)
	return string(b)
}

func (r *imageWorkbenchRepository) CreateTask(ctx context.Context, t *service.ImageWorkbenchTask) (int64, error) {
	const q = `
		INSERT INTO image_workbench_tasks
			(user_id, api_key_id, status, prompt, model, size, n, base_image_id, base_object_keys, result_image_ids, error, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING id`
	var id int64
	err := r.db.QueryRowContext(ctx, q,
		t.UserID, t.APIKeyID, t.Status, t.Prompt, t.Model, t.Size, t.N, t.BaseImageID,
		jsonStrings(t.BaseObjectKeys), jsonInts(t.ResultImageIDs), t.Error, t.CreatedAt, t.UpdatedAt,
	).Scan(&id)
	return id, err
}

func (r *imageWorkbenchRepository) ClaimNextTask(ctx context.Context) (*service.ImageWorkbenchTask, error) {
	const q = `
		UPDATE image_workbench_tasks SET status='running', updated_at=now()
		WHERE id = (
			SELECT id FROM image_workbench_tasks WHERE status='queued' ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1
		)
		RETURNING ` + imageWorkbenchTaskColumns
	t, err := scanImageWorkbenchTask(r.db.QueryRowContext(ctx, q))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}

func (r *imageWorkbenchRepository) FinishTask(ctx context.Context, id int64, status string, resultIDs []int64, errMsg string) error {
	const q = `UPDATE image_workbench_tasks SET status=$2, result_image_ids=$3, error=$4, updated_at=now() WHERE id=$1`
	_, err := r.db.ExecContext(ctx, q, id, status, jsonInts(resultIDs), errMsg)
	return err
}

func (r *imageWorkbenchRepository) ListTasksByUser(ctx context.Context, userID int64, status string, limit, offset int) ([]*service.ImageWorkbenchTask, error) {
	q := `SELECT ` + imageWorkbenchTaskColumns + ` FROM image_workbench_tasks WHERE user_id=$1`
	args := []any{userID}
	if status != "" {
		q += ` AND status=$2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`
		args = append(args, status, limit, offset)
	} else {
		q += ` ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		args = append(args, limit, offset)
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*service.ImageWorkbenchTask
	for rows.Next() {
		t, err := scanImageWorkbenchTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *imageWorkbenchRepository) CountActiveTasks(ctx context.Context, userID int64) (int, error) {
	const q = `SELECT count(*) FROM image_workbench_tasks WHERE user_id=$1 AND status IN ('queued','running')`
	var n int
	err := r.db.QueryRowContext(ctx, q, userID).Scan(&n)
	return n, err
}

func (r *imageWorkbenchRepository) RequeueStaleRunning(ctx context.Context) error {
	const q = `UPDATE image_workbench_tasks SET status='queued', updated_at=now() WHERE status='running'`
	_, err := r.db.ExecContext(ctx, q)
	return err
}
