package http

import (
	"errors"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/itering/subscan/internal/agentkeys"
)

const defaultAgentKeysAuditWorkerURL = "https://audit.litentry.org"

var agentkeysEnvelopeCache = agentkeys.NewEnvelopeCache()

func agentkeysAuditEnvelopeHandle(c *gin.Context) {
	hash := c.Param("hash")
	workerURL := os.Getenv("AGENTKEYS_AUDIT_WORKER_URL")
	if workerURL == "" {
		workerURL = defaultAgentKeysAuditWorkerURL
	}

	body, _, err := agentkeysEnvelopeCache.FetchAndDecode(c.Request.Context(), workerURL, hash)
	if errors.Is(err, agentkeys.ErrEnvelopeNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/octet-stream", body)
}
