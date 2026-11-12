package gui

// overridden reports whether the window manager gave the window a size
// other than the one ng asked for (reqW x reqH pixels): a tiling window
// manager, or one that clamped the window. Differences smaller than a cell
// come from rounding between window coordinates and pixels.
func overridden(reqW, reqH, gotW, gotH, cellW, cellH int) bool {
	return abs(gotW-reqW) >= cellW || abs(gotH-reqH) >= cellH
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
