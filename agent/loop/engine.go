package loop

// Engine drives task execution and emits events.
type Engine struct{}

// executor is a pluggable runner so the loop package can stay cycle-free.
var executor = struct {
	Run func(task Task) string
}{
	Run: func(task Task) string {
		return "Executed: " + task.Description
	},
}

func SetExecutorRun(run func(task Task) string) {
	if run == nil {
		return
	}
	executor.Run = run
}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) Run(task Task) ([]Event, error) {
	result := executor.Run(task)
	return []Event{{Type: "result", Message: result}}, nil
}
