package executor

import "github.com/vigo999/ms-cli/agent/loop"

func Run(task loop.Task) string {
	return "Executed: " + task.Description
}
