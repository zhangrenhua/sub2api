package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ImageWorkbenchHandler 画图工作台用户态接口(fork 功能,JWT 鉴权)。
type ImageWorkbenchHandler struct {
	service       *service.ImageWorkbenchService
	apiKeyService *service.APIKeyService
}

// NewImageWorkbenchHandler 构造画图工作台 handler。
func NewImageWorkbenchHandler(svc *service.ImageWorkbenchService, apiKeyService *service.APIKeyService) *ImageWorkbenchHandler {
	return &ImageWorkbenchHandler{service: svc, apiKeyService: apiKeyService}
}

type imageWorkbenchGenerateRequest struct {
	APIKeyID    int64  `json:"api_key_id"`
	Prompt      string `json:"prompt"`
	Model       string `json:"model"`
	Size        string `json:"size"`
	Quality     string `json:"quality"`
	N            int    `json:"n"`
	SessionID    string `json:"session_id"`
	BaseImageID   int64    `json:"base_image_id"`
	BaseImagesB64 []string `json:"base_images_b64"`
}

type imageWorkbenchImageDTO struct {
	ID            int64  `json:"id"`
	Prompt        string `json:"prompt"`
	RevisedPrompt string `json:"revised_prompt"`
	Model         string `json:"model"`
	Size          string `json:"size"`
	Quality       string `json:"quality"`
	SessionID     string `json:"session_id"`
	URL           string `json:"url"`
	Mime          string `json:"mime"`
	Bytes         int64  `json:"bytes"`
	CreatedAt     string `json:"created_at"`
	ExpiresAt     string `json:"expires_at"`
}

func imageWorkbenchToDTO(rec *service.ImageWorkbenchRecord) imageWorkbenchImageDTO {
	return imageWorkbenchImageDTO{
		ID:            rec.ID,
		Prompt:        rec.Prompt,
		RevisedPrompt: rec.RevisedPrompt,
		Model:         rec.Model,
		Size:          rec.Size,
		Quality:       rec.Quality,
		SessionID:     rec.SessionID,
		URL:           "/api/v1/image-workbench/files/" + rec.Token,
		Mime:          rec.Mime,
		Bytes:         rec.Bytes,
		CreatedAt:     rec.CreatedAt.Format(time.RFC3339),
		ExpiresAt:     rec.ExpiresAt.Format(time.RFC3339),
	}
}

type imageWorkbenchTaskDTO struct {
	ID        int64                    `json:"id"`
	Status    string                   `json:"status"`
	Prompt    string                   `json:"prompt"`
	Model     string                   `json:"model"`
	Size      string                   `json:"size"`
	N         int                      `json:"n"`
	Error     string                   `json:"error"`
	Images    []imageWorkbenchImageDTO `json:"images"`
	CreatedAt string                   `json:"created_at"`
	UpdatedAt string                   `json:"updated_at"`
}

func imageWorkbenchTaskToDTO(t *service.ImageWorkbenchTask) imageWorkbenchTaskDTO {
	imgs := make([]imageWorkbenchImageDTO, 0, len(t.ResultImages))
	for _, r := range t.ResultImages {
		imgs = append(imgs, imageWorkbenchToDTO(r))
	}
	return imageWorkbenchTaskDTO{
		ID:        t.ID,
		Status:    t.Status,
		Prompt:    t.Prompt,
		Model:     t.Model,
		Size:      t.Size,
		N:         t.N,
		Error:     t.Error,
		Images:    imgs,
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
		UpdatedAt: t.UpdatedAt.Format(time.RFC3339),
	}
}

// Generate 提交一个生图/多轮编辑任务(异步)：校验所选 key 归属后入队，worker 在服务端
// 执行(刷新页面不影响)，前端轮询 /tasks 获取状态与结果。返回新建任务。
func (h *ImageWorkbenchHandler) Generate(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	// 限制请求体大小：最多 4 张 × 20MB（/help），base64+JSON 留余量 120MB，防内存压力。
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 120<<20)
	var req imageWorkbenchGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		response.BadRequest(c, "prompt is required")
		return
	}
	if req.APIKeyID <= 0 {
		response.BadRequest(c, "api_key_id is required")
		return
	}

	// 提交时校验所选 key 归属，给用户即时反馈(执行时 worker 会再次解析取明文计费)。
	key, err := h.apiKeyService.GetByID(c.Request.Context(), req.APIKeyID)
	if err != nil || key == nil || key.UserID != subject.UserID {
		response.NotFound(c, "API key not found")
		return
	}

	task, err := h.service.CreateTask(c.Request.Context(), subject.UserID, req.APIKeyID, service.ImageWorkbenchGenerateRequest{
		Prompt:        req.Prompt,
		Model:         req.Model,
		Size:          req.Size,
		Quality:       req.Quality,
		N:             req.N,
		SessionID:     req.SessionID,
		BaseImageID:   req.BaseImageID,
		BaseImagesB64: req.BaseImagesB64,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"task": imageWorkbenchTaskToDTO(task)})
}

// Tasks 列出当前用户的生图任务(可按 status 过滤)，供前端轮询与任务队列页查询。
func (h *ImageWorkbenchHandler) Tasks(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	status := strings.TrimSpace(c.Query("status"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	tasks, err := h.service.ListTasks(c.Request.Context(), subject.UserID, status, limit, offset)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	dtos := make([]imageWorkbenchTaskDTO, 0, len(tasks))
	for _, t := range tasks {
		dtos = append(dtos, imageWorkbenchTaskToDTO(t))
	}
	response.Success(c, gin.H{"tasks": dtos})
}

// History 当前用户 7 天内未过期的生图记录。
func (h *ImageWorkbenchHandler) History(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	records, err := h.service.List(c.Request.Context(), subject.UserID, limit, offset)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	dtos := make([]imageWorkbenchImageDTO, 0, len(records))
	for _, r := range records {
		dtos = append(dtos, imageWorkbenchToDTO(r))
	}
	response.Success(c, gin.H{"images": dtos})
}

// ServeFile 用不可猜测 token 串流图片文件(免 JWT,供 <img src> 直接访问;校验未过期)。
func (h *ImageWorkbenchHandler) ServeFile(c *gin.Context) {
	token := c.Param("token")
	full, mime, err := h.service.ResolveFileByToken(c.Request.Context(), token)
	if err != nil {
		response.NotFound(c, "Image not found")
		return
	}
	c.Header("Content-Type", mime)
	c.Header("Cache-Control", "private, max-age=86400")
	c.File(full)
}

// Delete 删除一条生图记录及其文件。
func (h *ImageWorkbenchHandler) Delete(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid id")
		return
	}
	if err := h.service.Delete(c.Request.Context(), id, subject.UserID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}
