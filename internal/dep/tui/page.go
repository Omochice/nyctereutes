package tui

// Chooses the half-open range [start, end) of the at most capacity rows
// that make up the page holding cursor in a list of total rows.
// The page is derived from the cursor alone instead of a stored scroll
// offset, because every place that resets the cursor would also have to
// reset such an offset.
// The last page is aligned to the end of the list so the frame keeps a
// constant height and the status line below it does not jump.
//
// Returns 0 and total when every row fits in capacity.
func pageBounds(cursor, total, capacity int) (int, int) {
	if total <= capacity {
		return 0, total
	}
	start := min((cursor/capacity)*capacity, total-capacity)
	return start, start + capacity
}
