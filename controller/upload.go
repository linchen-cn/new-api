package controller

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PlaygroundUpload handles file uploads from the Creation Center.
// Files are stored in Alibaba Cloud OSS and a public URL is returned.
//
// Supported file types: images (jpg, png, gif, webp) and videos (mp4, mov, webm).
// Max file size: 100 MB.
const maxUploadSize = 100 << 20 // 100 MB

var allowedImageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
}
var allowedVideoExts = map[string]bool{
	".mp4": true, ".mov": true, ".webm": true,
}

func PlaygroundUpload(c *gin.Context) {
	if !common.IsOSSEnabled() {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "OSS storage is not configured. Please set OSS_ENDPOINT, OSS_ACCESS_KEY_ID, OSS_ACCESS_KEY_SECRET, OSS_BUCKET_NAME environment variables.",
		})
		return
	}

	// Limit request body size
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("failed to read uploaded file: %v", err),
		})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	fileType := "images"
	if allowedImageExts[ext] {
		fileType = "images"
	} else if allowedVideoExts[ext] {
		fileType = "videos"
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("unsupported file type: %s. Allowed: jpg, png, gif, webp, mp4, mov, webm", ext),
		})
		return
	}

	// Build object key: playground/{images|videos}/{date}/{uuid}.{ext}
	dateStr := time.Now().Format("20060102")
	objectKey := fmt.Sprintf("%s%s/%s/%s%s",
		common.OSSPathPrefix(), fileType, dateStr, uuid.New().String(), ext)

	url, err := common.UploadToOSS(file, objectKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"url":      url,
			"fileType": fileType,
			"fileName": header.Filename,
		},
	})
}
