package tui

// The page is derived from the cursor alone instead of a stored scroll
// offset, because every place that resets the cursor would also have to
// reset such an offset.
// The clamp aligns the last page to the end of the list, so the frame keeps a
// constant height and the status line below it does not jump.
func pageBounds(cursor, total, capacity int) (start, end int) {
	return windowBounds((cursor/capacity)*capacity, total, capacity)
}

// The offset is clamped instead of trusted, because a resize can shrink the
// largest valid offset after it was set.
func windowBounds(offset, total, capacity int) (start, end int) {
	if total <= capacity {
		return 0, total
	}
	start = min(max(offset, 0), total-capacity)
	return start, start + capacity
}
