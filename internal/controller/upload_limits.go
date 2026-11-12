package controller

import (
    "errors"
    "net/http"

    "github.com/gin-gonic/gin"
)

// limitMultipartUpload bounds the entire multipart request, including headers and form fields.
func limitMultipartUpload(c *gin.Context, maxBytes int64) {
    c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
}

func multipartUploadTooLarge(err error) bool {
    var maxErr *http.MaxBytesError
    return errors.As(err, &maxErr)
}
