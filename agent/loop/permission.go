package loop

// PermissionService controls tool-call permissions.
type PermissionService interface {
	Request(tool, action, path string) (bool, error)
}
