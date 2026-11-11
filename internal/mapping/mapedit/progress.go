package mapedit

// progressStride is how many steps pass between two Progress calls.
const progressStride = 4096

// progressOver returns a function to call once per step of work with total
// steps; it reports to Progress every progressStride steps and at the end.
func (e *Editor) progressOver(total int) func() {
	report := e.Progress
	if report == nil {
		return func() {}
	}
	done := 0
	return func() {
		done++
		if done%progressStride == 0 || done == total {
			report(done, total)
		}
	}
}
