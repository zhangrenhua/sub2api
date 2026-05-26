package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// CryptoWalletHandler exposes admin endpoints for the self-custodied TRC20 HD
// wallet: balance overview, address listing, and the TOTP-gated sensitive
// operations (initialization, collection address, one-click sweep).
type CryptoWalletHandler struct {
	walletService *service.CryptoWalletService
	totpService   *service.TotpService
}

func NewCryptoWalletHandler(walletService *service.CryptoWalletService, totpService *service.TotpService) *CryptoWalletHandler {
	return &CryptoWalletHandler{walletService: walletService, totpService: totpService}
}

// requireTOTP verifies the caller's TOTP code for a sensitive money-moving
// operation. Returns the admin user id on success.
func (h *CryptoWalletHandler) requireTOTP(c *gin.Context, code string) (int64, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return 0, false
	}
	if err := h.totpService.VerifyCode(c.Request.Context(), subject.UserID, code); err != nil {
		response.ErrorFrom(c, err)
		return 0, false
	}
	return subject.UserID, true
}

// GetOverview returns wallet balances for the admin dashboard.
// GET /api/v1/admin/payment/crypto/overview
func (h *CryptoWalletHandler) GetOverview(c *gin.Context) {
	ov, err := h.walletService.Overview(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, ov)
}

// ListAddresses returns per-user deposit addresses with cached balances.
// GET /api/v1/admin/payment/crypto/addresses
func (h *CryptoWalletHandler) ListAddresses(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	network := c.Query("network") // "" = all networks
	items, total, err := h.walletService.ListAddresses(c.Request.Context(), network, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "total": total})
}

// RefreshBalances queries the chain and updates cached balances.
// POST /api/v1/admin/payment/crypto/refresh-balances
func (h *CryptoWalletHandler) RefreshBalances(c *gin.Context) {
	n, err := h.walletService.RefreshBalances(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"refreshed": n})
}

type initWalletRequest struct {
	Mnemonic string `json:"mnemonic"`
	TotpCode string `json:"totp_code" binding:"required"`
}

// InitWallet initializes or imports the master mnemonic (TOTP-gated, one-time).
// POST /api/v1/admin/payment/crypto/wallet/init
func (h *CryptoWalletHandler) InitWallet(c *gin.Context) {
	var req initWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	if _, ok := h.requireTOTP(c, req.TotpCode); !ok {
		return
	}
	res, err := h.walletService.InitWallet(c.Request.Context(), req.Mnemonic)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	// res.Mnemonic is present only when freshly generated — shown once for backup.
	response.Success(c, res)
}

type collectionAddressRequest struct {
	Address  string `json:"address" binding:"required"`
	TotpCode string `json:"totp_code" binding:"required"`
}

// SetCollectionAddress updates the sweep destination (TOTP-gated).
// PUT /api/v1/admin/payment/crypto/wallet/collection-address
func (h *CryptoWalletHandler) SetCollectionAddress(c *gin.Context) {
	var req collectionAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	if _, ok := h.requireTOTP(c, req.TotpCode); !ok {
		return
	}
	if err := h.walletService.SetCollectionAddress(c.Request.Context(), req.Address); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

type sweepRequest struct {
	TotpCode string `json:"totp_code" binding:"required"`
}

// StartSweep triggers a one-click TRC20 consolidation (TOTP-gated).
// POST /api/v1/admin/payment/crypto/sweep
func (h *CryptoWalletHandler) StartSweep(c *gin.Context) {
	var req sweepRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	uid, ok := h.requireTOTP(c, req.TotpCode)
	if !ok {
		return
	}
	job, err := h.walletService.StartSweep(c.Request.Context(), "user:"+strconv.FormatInt(uid, 10))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, job)
}

// SetEthCollectionAddress updates the ERC20 sweep destination (TOTP-gated).
// PUT /api/v1/admin/payment/crypto/wallet/eth-collection-address
func (h *CryptoWalletHandler) SetEthCollectionAddress(c *gin.Context) {
	var req collectionAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	if _, ok := h.requireTOTP(c, req.TotpCode); !ok {
		return
	}
	if err := h.walletService.SetEthCollectionAddress(c.Request.Context(), req.Address); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// StartSweepEth triggers a one-click ERC20 consolidation (TOTP-gated).
// POST /api/v1/admin/payment/crypto/eth-sweep
func (h *CryptoWalletHandler) StartSweepEth(c *gin.Context) {
	var req sweepRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	uid, ok := h.requireTOTP(c, req.TotpCode)
	if !ok {
		return
	}
	job, err := h.walletService.StartSweepEth(c.Request.Context(), "user:"+strconv.FormatInt(uid, 10))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, job)
}

// GetSweepJob returns a sweep job with its per-address tasks.
// GET /api/v1/admin/payment/crypto/sweep/:jobId
func (h *CryptoWalletHandler) GetSweepJob(c *gin.Context) {
	jobID, err := strconv.ParseInt(c.Param("jobId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid job id")
		return
	}
	job, tasks, err := h.walletService.GetSweepJob(c.Request.Context(), jobID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"job": job, "tasks": tasks})
}

// ListSweepJobs returns recent sweep jobs.
// GET /api/v1/admin/payment/crypto/sweeps
func (h *CryptoWalletHandler) ListSweepJobs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	jobs, err := h.walletService.ListSweepJobs(c.Request.Context(), limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": jobs})
}
