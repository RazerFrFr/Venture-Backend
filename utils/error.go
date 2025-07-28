package utils

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

func CreateError(c *gin.Context, errorCode, errorMessage string, messageVars []string, numericErrorCode int, err string, statusCode int) {
	c.Header("X-Epic-Error-Name", errorCode)
	c.Header("X-Epic-Error-Code", fmt.Sprintf("%d", numericErrorCode))

	c.JSON(statusCode, gin.H{
		"errorCode":          errorCode,
		"errorMessage":       errorMessage,
		"messageVars":        messageVars,
		"numericErrorCode":   numericErrorCode,
		"originatingService": "any",
		"intent":             "prod",
		"error_description":  errorMessage,
		"error":              err,
	})
}

type HTTPError struct {
	Header  string
	Message string
}

func MethodNotAllowedError(c *gin.Context) *HTTPError {
	return &HTTPError{
		Header:  "Allow",
		Message: "Method Not Allowed",
	}
}
