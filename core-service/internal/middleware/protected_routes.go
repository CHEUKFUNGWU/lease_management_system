package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

// Permission names the authorization required to cross a protected HTTP seam.
type Permission struct {
	Resource string
	Action   string
}

// ProtectedRouter makes permission declaration mandatory for every registered route.
type ProtectedRouter struct {
	group *gin.RouterGroup
}

func NewProtectedRouter(group *gin.RouterGroup) *ProtectedRouter {
	return &ProtectedRouter{group: group}
}

func (r *ProtectedRouter) Handle(method, path string, permission Permission, handlers ...gin.HandlerFunc) {
	resource := strings.TrimSpace(permission.Resource)
	action := strings.TrimSpace(permission.Action)
	if resource == "" || action == "" {
		panic(fmt.Sprintf("protected route %s %s requires a permission", method, path))
	}

	chain := make([]gin.HandlerFunc, 0, len(handlers)+1)
	chain = append(chain, RBACMiddleware(resource, action))
	chain = append(chain, handlers...)
	r.group.Handle(method, path, chain...)
}
