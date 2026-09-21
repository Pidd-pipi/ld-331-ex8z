package handler

import (
	"errors"
	"github.com/gbsched/hospital-scheduler/internal/constants"
	"github.com/gbsched/hospital-scheduler/internal/repository"
	"github.com/gbsched/hospital-scheduler/pkg/response"
	"github.com/gin-gonic/gin"
	"net/http"
)

func handleError(c *gin.Context, e error) {
	if errors.Is(e, repository.ErrNotFound) {
		response.Error(c, http.StatusNotFound, constants.CodeNotFound, "资源不存在")
		return
	}
	response.Error(c, http.StatusBadRequest, constants.CodeBadRequest, e.Error())
}
func parseID(c *gin.Context) (uint, bool) {
	var id uint
	if e := c.ShouldBindUri(&struct {
		ID *uint `uri:"id" binding:"required"`
	}{ID: &id}); e != nil {
		response.Error(c, 400, constants.CodeBadRequest, "无效资源编号")
		return 0, false
	}
	return id, true
}
