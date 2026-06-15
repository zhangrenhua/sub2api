package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// Fork feature: 画图工作台 (Image Workbench) —— 后端异步任务模式。
//
// 提交即建任务(image_workbench_tasks，queued)；服务端 worker 池领取并执行：按所选 key 对
// 网关 /v1/images/{generations,edits} 发 loopback 请求(复用全部鉴权/计费/限流/转发)，拿回
// base64 落本地盘、写 image_workbench_images(带 token + 3 天过期)，更新任务为 done/error。
// 任务在服务端运行，刷新页面不影响；前端轮询 /tasks 展示状态与结果。

const (
	imageWorkbenchTTL            = 3 * 24 * time.Hour
	imageWorkbenchMaxN           = 4
	imageWorkbenchMaxPerUser     = 50              // 每用户最多保留图片数(超出删最旧)
	imageWorkbenchHTTPTimeout    = 10 * time.Minute // 生图同步阻塞，给到 600s
	imageWorkbenchMaxImageBytes  = 32 << 20         // 32MB 单图上限(落盘保护)
	imageWorkbenchMaxEditImages  = 4                // 图改图单次最多输入张数(/help)
	imageWorkbenchMaxInputBytes  = 20 << 20         // 单张输入图上限 20MB(/help)
	imageWorkbenchMaxPromptLen   = 12000            // 后端 prompt 字符上限(前端 UX 限 10000)
	imageWorkbenchMaxActiveTasks = 20               // 每用户排队+执行中任务上限
	imageWorkbenchWorkers        = 20               // 并发 worker 数
	imageWorkbenchBaseSubdir     = "_base"          // 上传底图临时落盘子目录
)

// apiKeyResolver 解析 api_key_id → APIKey(明文 + 归属)，供 worker 执行时取 bearer。
type apiKeyResolver interface {
	GetByID(ctx context.Context, id int64) (*APIKey, error)
}

// ImageWorkbenchRecord 对应 image_workbench_images 表一行。
type ImageWorkbenchRecord struct {
	ID            int64
	UserID        int64
	SessionID     string
	Prompt        string
	RevisedPrompt string
	Model         string
	Size          string
	Quality       string
	Storage       string
	ObjectKey     string
	Token         string
	Mime          string
	Bytes         int64
	Width         int
	Height        int
	CreatedAt     time.Time
	ExpiresAt     time.Time
}

// ImageWorkbenchTask 对应 image_workbench_tasks 表一行(+ 运行期填充的结果图)。
type ImageWorkbenchTask struct {
	ID             int64
	UserID         int64
	APIKeyID       int64
	Status         string
	Prompt         string
	Model          string
	Size           string
	N              int
	BaseImageID    int64
	BaseObjectKeys []string
	ResultImageIDs []int64
	Error          string
	CreatedAt      time.Time
	UpdatedAt      time.Time

	ResultImages []*ImageWorkbenchRecord // 运行期填充，不入库
}

// ImageWorkbenchRepository 画图工作台仓储(raw SQL,实现见 repository 包)。
type ImageWorkbenchRepository interface {
	// 图片
	Create(ctx context.Context, rec *ImageWorkbenchRecord) (int64, error)
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*ImageWorkbenchRecord, error)
	GetByID(ctx context.Context, id int64) (*ImageWorkbenchRecord, error)
	GetByIDs(ctx context.Context, ids []int64) ([]*ImageWorkbenchRecord, error)
	GetByToken(ctx context.Context, token string) (*ImageWorkbenchRecord, error)
	Delete(ctx context.Context, id, userID int64) (string, error)
	DeleteExpired(ctx context.Context, now time.Time, limit int) ([]string, error)
	DeleteOverLimit(ctx context.Context, userID int64, keep int) ([]string, error)
	// 任务
	CreateTask(ctx context.Context, task *ImageWorkbenchTask) (int64, error)
	ClaimNextTask(ctx context.Context) (*ImageWorkbenchTask, error)
	FinishTask(ctx context.Context, id int64, status string, resultIDs []int64, errMsg string) error
	ListTasksByUser(ctx context.Context, userID int64, status string, limit, offset int) ([]*ImageWorkbenchTask, error)
	CountActiveTasks(ctx context.Context, userID int64) (int, error)
	RequeueStaleRunning(ctx context.Context) error
}

// ImageWorkbenchGenerateRequest 生成/编辑入参。
type ImageWorkbenchGenerateRequest struct {
	Prompt        string
	Model         string
	Size          string
	Quality       string
	N             int
	SessionID     string
	BaseImageID   int64
	BaseImagesB64 []string
}

// ImageWorkbenchService 画图工作台业务。
type ImageWorkbenchService struct {
	repo       ImageWorkbenchRepository
	cfg        *config.Config
	keys       apiKeyResolver
	storageDir string
	httpClient *http.Client
}

// NewImageWorkbenchService 构造画图工作台 service。
func NewImageWorkbenchService(repo ImageWorkbenchRepository, cfg *config.Config, keys apiKeyResolver) *ImageWorkbenchService {
	dir := strings.TrimSpace(os.Getenv("IMAGE_WORKBENCH_DIR"))
	if dir == "" {
		dir = filepath.Join("data", "image-workbench")
	}
	// loopback 全部打到本机网关(同一 host)，调大单主机空闲连接复用，避免 worker 增多时连接抖动。
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = imageWorkbenchWorkers * 2
	transport.MaxIdleConnsPerHost = imageWorkbenchWorkers
	return &ImageWorkbenchService{
		repo:       repo,
		cfg:        cfg,
		keys:       keys,
		storageDir: dir,
		httpClient: &http.Client{Timeout: imageWorkbenchHTTPTimeout, Transport: transport},
	}
}

// ProvideImageWorkbenchService 构造 service 并启动清理 + worker 池(供 google/wire 装配)。
func ProvideImageWorkbenchService(repo ImageWorkbenchRepository, cfg *config.Config, keys *APIKeyService) *ImageWorkbenchService {
	svc := NewImageWorkbenchService(repo, cfg, keys)
	// 启动时把上次进程残留的 running 任务重新置为 queued(进程退出未完成的任务可被重领)
	if err := svc.repo.RequeueStaleRunning(context.Background()); err != nil {
		slog.Warn("image_workbench.requeue_stale_failed", "err", err)
	}
	svc.StartCleanupLoop()
	svc.StartWorkers(imageWorkbenchWorkers)
	return svc
}

// StorageDir 暴露存储根目录。
func (s *ImageWorkbenchService) StorageDir() string { return s.storageDir }

func (s *ImageWorkbenchService) loopbackPort() int {
	if s.cfg != nil && s.cfg.Server.Port > 0 {
		return s.cfg.Server.Port
	}
	return 8080
}

// ---- 任务：创建 / 查询 ----

// CreateTask 校验并入队一个生图任务(不阻塞，worker 异步执行)。
func (s *ImageWorkbenchService) CreateTask(ctx context.Context, userID, apiKeyID int64, req ImageWorkbenchGenerateRequest) (*ImageWorkbenchTask, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	if len(req.Prompt) > imageWorkbenchMaxPromptLen {
		return nil, fmt.Errorf("prompt too long")
	}
	if strings.TrimSpace(req.Model) == "" {
		return nil, fmt.Errorf("model is required")
	}
	if req.N < 1 {
		req.N = 1
	}
	if req.N > imageWorkbenchMaxN {
		req.N = imageWorkbenchMaxN
	}

	active, err := s.repo.CountActiveTasks(ctx, userID)
	if err != nil {
		return nil, err
	}
	if active >= imageWorkbenchMaxActiveTasks {
		return nil, fmt.Errorf("task queue is full (max %d)", imageWorkbenchMaxActiveTasks)
	}

	var baseKeys []string
	switch {
	case len(req.BaseImagesB64) > 0:
		if len(req.BaseImagesB64) > imageWorkbenchMaxEditImages {
			return nil, fmt.Errorf("too many images (max %d)", imageWorkbenchMaxEditImages)
		}
		for _, b64 := range req.BaseImagesB64 {
			data, dErr := decodeImagePayload(b64)
			if dErr != nil || len(data) == 0 {
				return nil, fmt.Errorf("invalid uploaded image")
			}
			if len(data) > imageWorkbenchMaxInputBytes {
				return nil, fmt.Errorf("uploaded image too large (max 20MB)")
			}
			key, wErr := s.writeBaseFile(userID, data)
			if wErr != nil {
				return nil, fmt.Errorf("save base image failed: %w", wErr)
			}
			baseKeys = append(baseKeys, key)
		}
	case req.BaseImageID > 0:
		base, gErr := s.repo.GetByID(ctx, req.BaseImageID)
		if gErr != nil || base == nil || base.UserID != userID {
			return nil, fmt.Errorf("base image not found")
		}
	}

	now := time.Now()
	task := &ImageWorkbenchTask{
		UserID:         userID,
		APIKeyID:       apiKeyID,
		Status:         "queued",
		Prompt:         req.Prompt,
		Model:          req.Model,
		Size:           req.Size,
		N:              req.N,
		BaseImageID:    req.BaseImageID,
		BaseObjectKeys: baseKeys,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	id, err := s.repo.CreateTask(ctx, task)
	if err != nil {
		s.cleanupBaseFiles(baseKeys)
		return nil, err
	}
	task.ID = id
	return task, nil
}

// ListTasks 列出用户任务(可按 status 过滤)，并填充结果图。
func (s *ImageWorkbenchService) ListTasks(ctx context.Context, userID int64, status string, limit, offset int) ([]*ImageWorkbenchTask, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	tasks, err := s.repo.ListTasksByUser(ctx, userID, status, limit, offset)
	if err != nil {
		return nil, err
	}
	// 收集所有结果图 id 批量取
	var allIDs []int64
	for _, t := range tasks {
		allIDs = append(allIDs, t.ResultImageIDs...)
	}
	if len(allIDs) > 0 {
		imgs, iErr := s.repo.GetByIDs(ctx, allIDs)
		if iErr == nil {
			byID := make(map[int64]*ImageWorkbenchRecord, len(imgs))
			for _, im := range imgs {
				byID[im.ID] = im
			}
			for _, t := range tasks {
				for _, rid := range t.ResultImageIDs {
					if im := byID[rid]; im != nil {
						t.ResultImages = append(t.ResultImages, im)
					}
				}
			}
		}
	}
	return tasks, nil
}

// ---- worker ----

// StartWorkers 启动 n 个后台 worker，轮询领取并执行任务。
func (s *ImageWorkbenchService) StartWorkers(n int) {
	for i := 0; i < n; i++ {
		go s.workerLoop()
	}
}

func (s *ImageWorkbenchService) workerLoop() {
	for {
		task, err := s.repo.ClaimNextTask(context.Background())
		if err != nil {
			slog.Warn("image_workbench.claim_failed", "err", err)
			time.Sleep(2 * time.Second)
			continue
		}
		if task == nil {
			time.Sleep(time.Second)
			continue
		}
		s.runTask(task)
	}
}

func (s *ImageWorkbenchService) runTask(task *ImageWorkbenchTask) {
	ctx, cancel := context.WithTimeout(context.Background(), imageWorkbenchHTTPTimeout+time.Minute)
	defer cancel()
	defer s.cleanupBaseFiles(task.BaseObjectKeys)

	key, err := s.keys.GetByID(ctx, task.APIKeyID)
	if err != nil || key == nil || key.UserID != task.UserID {
		_ = s.repo.FinishTask(ctx, task.ID, "error", nil, "API key not found")
		return
	}
	baseImages, lErr := s.loadBaseImages(ctx, task)
	if lErr != nil {
		_ = s.repo.FinishTask(ctx, task.ID, "error", nil, lErr.Error())
		return
	}
	req := ImageWorkbenchGenerateRequest{Prompt: task.Prompt, Model: task.Model, Size: task.Size, N: task.N}
	records, gErr := s.executeGeneration(ctx, task.UserID, key.Key, req, baseImages)
	if gErr != nil {
		_ = s.repo.FinishTask(ctx, task.ID, "error", nil, truncateString(sanitizeUpstreamErrorMessage(gErr.Error()), 500))
		return
	}
	ids := make([]int64, 0, len(records))
	for _, r := range records {
		ids = append(ids, r.ID)
	}
	if err := s.repo.FinishTask(ctx, task.ID, "done", ids, ""); err != nil {
		slog.Warn("image_workbench.finish_failed", "task_id", task.ID, "err", err)
	}
}

func (s *ImageWorkbenchService) loadBaseImages(ctx context.Context, task *ImageWorkbenchTask) ([][]byte, error) {
	if len(task.BaseObjectKeys) > 0 {
		imgs := make([][]byte, 0, len(task.BaseObjectKeys))
		for _, k := range task.BaseObjectKeys {
			data, err := os.ReadFile(filepath.Join(s.storageDir, filepath.Clean(k)))
			if err != nil {
				return nil, fmt.Errorf("base image file unavailable")
			}
			imgs = append(imgs, data)
		}
		return imgs, nil
	}
	if task.BaseImageID > 0 {
		base, err := s.repo.GetByID(ctx, task.BaseImageID)
		if err != nil || base == nil || base.UserID != task.UserID {
			return nil, fmt.Errorf("base image not found")
		}
		data, rErr := os.ReadFile(filepath.Join(s.storageDir, filepath.Clean(base.ObjectKey)))
		if rErr != nil {
			return nil, fmt.Errorf("base image file unavailable")
		}
		return [][]byte{data}, nil
	}
	return nil, nil
}

// executeGeneration loopback 生成/编辑 + 落盘落库 + 超额清理(worker 调用)。
func (s *ImageWorkbenchService) executeGeneration(ctx context.Context, userID int64, bearerKey string, req ImageWorkbenchGenerateRequest, baseImages [][]byte) ([]*ImageWorkbenchRecord, error) {
	var (
		images []imageWorkbenchUpstreamImage
		err    error
	)
	if len(baseImages) > 0 {
		images, err = s.loopbackEdit(ctx, bearerKey, req, baseImages)
	} else {
		images, err = s.loopbackGenerate(ctx, bearerKey, req)
	}
	if err != nil {
		return nil, err
	}
	if len(images) == 0 {
		return nil, fmt.Errorf("upstream returned no image")
	}

	now := time.Now()
	out := make([]*ImageWorkbenchRecord, 0, len(images))
	for _, img := range images {
		data, dErr := decodeImagePayload(img.b64)
		if dErr != nil || len(data) == 0 || len(data) > imageWorkbenchMaxImageBytes {
			continue
		}
		objectKey, mimeType, wErr := s.writeImageFile(userID, data)
		if wErr != nil {
			return nil, fmt.Errorf("save image failed: %w", wErr)
		}
		rec := &ImageWorkbenchRecord{
			UserID:        userID,
			SessionID:     truncateString(req.SessionID, 64),
			Prompt:        req.Prompt,
			RevisedPrompt: img.revisedPrompt,
			Model:         req.Model,
			Size:          req.Size,
			Quality:       req.Quality,
			Storage:       "local",
			ObjectKey:     objectKey,
			Token:         randHex16(),
			Mime:          mimeType,
			Bytes:         int64(len(data)),
			CreatedAt:     now,
			ExpiresAt:     now.Add(imageWorkbenchTTL),
		}
		id, cErr := s.repo.Create(ctx, rec)
		if cErr != nil {
			_ = os.Remove(filepath.Join(s.storageDir, objectKey))
			return nil, fmt.Errorf("persist image failed: %w", cErr)
		}
		rec.ID = id
		out = append(out, rec)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid image produced")
	}
	s.enforceUserLimit(ctx, userID)
	return out, nil
}

// enforceUserLimit 删除该用户最近 imageWorkbenchMaxPerUser 张之外的旧图(best-effort)。
func (s *ImageWorkbenchService) enforceUserLimit(ctx context.Context, userID int64) {
	keys, err := s.repo.DeleteOverLimit(ctx, userID, imageWorkbenchMaxPerUser)
	if err != nil {
		slog.Warn("image_workbench.enforce_limit_failed", "user_id", userID, "err", err)
		return
	}
	for _, k := range keys {
		if k != "" {
			_ = os.Remove(filepath.Join(s.storageDir, filepath.Clean(k)))
		}
	}
}

// ---- 图片：查询 / 删除 / 文件 / 清理 ----

func (s *ImageWorkbenchService) List(ctx context.Context, userID int64, limit, offset int) ([]*ImageWorkbenchRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListByUser(ctx, userID, limit, offset)
}

func (s *ImageWorkbenchService) ResolveFileByToken(ctx context.Context, token string) (string, string, error) {
	if strings.TrimSpace(token) == "" {
		return "", "", fmt.Errorf("image not found")
	}
	rec, err := s.repo.GetByToken(ctx, token)
	if err != nil || rec == nil {
		return "", "", fmt.Errorf("image not found")
	}
	if time.Now().After(rec.ExpiresAt) {
		return "", "", fmt.Errorf("image expired")
	}
	full := filepath.Join(s.storageDir, filepath.Clean(rec.ObjectKey))
	if _, err := os.Stat(full); err != nil {
		return "", "", fmt.Errorf("image file unavailable")
	}
	return full, rec.Mime, nil
}

func (s *ImageWorkbenchService) Delete(ctx context.Context, id, userID int64) error {
	objectKey, err := s.repo.Delete(ctx, id, userID)
	if err != nil {
		return err
	}
	if objectKey != "" {
		_ = os.Remove(filepath.Join(s.storageDir, filepath.Clean(objectKey)))
	}
	return nil
}

func (s *ImageWorkbenchService) CleanupExpired(ctx context.Context) (int, error) {
	keys, err := s.repo.DeleteExpired(ctx, time.Now(), 1000)
	if err != nil {
		return 0, err
	}
	for _, k := range keys {
		if k != "" {
			_ = os.Remove(filepath.Join(s.storageDir, filepath.Clean(k)))
		}
	}
	return len(keys), nil
}

// StartCleanupLoop 每天凌晨 3:00(本地时区)删除已过期(默认 3 天)的图片。
func (s *ImageWorkbenchService) StartCleanupLoop() {
	go func() {
		s.runCleanupOnce()
		for {
			timer := time.NewTimer(durationUntilHour(3))
			<-timer.C
			s.runCleanupOnce()
		}
	}()
}

func (s *ImageWorkbenchService) runCleanupOnce() {
	if n, err := s.CleanupExpired(context.Background()); err != nil {
		slog.Warn("image_workbench.cleanup_failed", "err", err)
	} else if n > 0 {
		slog.Info("image_workbench.cleanup", "deleted", n)
	}
}

func durationUntilHour(hour int) time.Duration {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next.Sub(now)
}

// ---- loopback ----

type imageWorkbenchUpstreamImage struct {
	b64           string
	revisedPrompt string
}

func (s *ImageWorkbenchService) loopbackGenerate(ctx context.Context, bearerKey string, req ImageWorkbenchGenerateRequest) ([]imageWorkbenchUpstreamImage, error) {
	payload := map[string]any{"model": req.Model, "prompt": req.Prompt, "n": req.N}
	if req.Size != "" {
		payload["size"] = req.Size
	}
	if req.Quality != "" {
		payload["quality"] = req.Quality
	}
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("http://127.0.0.1:%d/v1/images/generations", s.loopbackPort())
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+bearerKey)
	return s.doLoopback(httpReq)
}

func (s *ImageWorkbenchService) loopbackEdit(ctx context.Context, bearerKey string, req ImageWorkbenchGenerateRequest, baseImages [][]byte) ([]imageWorkbenchUpstreamImage, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for i, img := range baseImages {
		ext := extFromMime(http.DetectContentType(img))
		fw, err := mw.CreateFormFile("image[]", fmt.Sprintf("img%d%s", i, ext))
		if err != nil {
			return nil, err
		}
		if _, err := fw.Write(img); err != nil {
			return nil, err
		}
	}
	_ = mw.WriteField("model", req.Model)
	_ = mw.WriteField("prompt", req.Prompt)
	_ = mw.WriteField("n", strconv.Itoa(req.N))
	if req.Size != "" {
		_ = mw.WriteField("size", req.Size)
	}
	_ = mw.Close()

	url := fmt.Sprintf("http://127.0.0.1:%d/v1/images/edits", s.loopbackPort())
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", mw.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer "+bearerKey)
	return s.doLoopback(httpReq)
}

func (s *ImageWorkbenchService) doLoopback(httpReq *http.Request) ([]imageWorkbenchUpstreamImage, error) {
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("upstream request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(sanitizeUpstreamErrorMessage(extractUpstreamErrorMessage(respBody)))
		if msg == "" {
			msg = "HTTP " + strconv.Itoa(resp.StatusCode)
		}
		return nil, fmt.Errorf("%s", msg)
	}
	var parsed struct {
		Data []struct {
			B64JSON       string `json:"b64_json"`
			URL           string `json:"url"`
			RevisedPrompt string `json:"revised_prompt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("parse upstream response failed")
	}
	out := make([]imageWorkbenchUpstreamImage, 0, len(parsed.Data))
	for _, d := range parsed.Data {
		payload := d.B64JSON
		if payload == "" {
			payload = d.URL
		}
		if payload == "" {
			continue
		}
		out = append(out, imageWorkbenchUpstreamImage{b64: payload, revisedPrompt: d.RevisedPrompt})
	}
	return out, nil
}

// ---- storage helpers ----

func (s *ImageWorkbenchService) writeImageFile(userID int64, data []byte) (objectKey, mimeType string, err error) {
	return s.writeFileUnder(strconv.FormatInt(userID, 10), data)
}

func (s *ImageWorkbenchService) writeBaseFile(userID int64, data []byte) (objectKey string, err error) {
	k, _, e := s.writeFileUnder(filepath.Join(imageWorkbenchBaseSubdir, strconv.FormatInt(userID, 10)), data)
	return k, e
}

func (s *ImageWorkbenchService) writeFileUnder(subdir string, data []byte) (objectKey, mimeType string, err error) {
	mimeType = http.DetectContentType(data)
	if !strings.HasPrefix(mimeType, "image/") {
		mimeType = "image/png"
	}
	name := randHex16() + extFromMime(mimeType)
	objectKey = filepath.ToSlash(filepath.Join(subdir, name))
	full := filepath.Join(s.storageDir, filepath.FromSlash(objectKey))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return "", "", err
	}
	return objectKey, mimeType, nil
}

func (s *ImageWorkbenchService) cleanupBaseFiles(keys []string) {
	for _, k := range keys {
		if k != "" {
			_ = os.Remove(filepath.Join(s.storageDir, filepath.Clean(k)))
		}
	}
}

func decodeImagePayload(payload string) ([]byte, error) {
	payload = strings.TrimSpace(payload)
	if strings.HasPrefix(payload, "data:") {
		if idx := strings.Index(payload, ","); idx >= 0 {
			payload = payload[idx+1:]
		}
	}
	payload = strings.TrimSpace(payload)
	if data, err := base64.StdEncoding.DecodeString(payload); err == nil {
		return data, nil
	}
	return base64.RawStdEncoding.DecodeString(payload)
}

func extFromMime(mimeType string) string {
	switch {
	case strings.Contains(mimeType, "jpeg"), strings.Contains(mimeType, "jpg"):
		return ".jpg"
	case strings.Contains(mimeType, "webp"):
		return ".webp"
	default:
		return ".png"
	}
}

func randHex16() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}
