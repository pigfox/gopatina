package a

// Count is reached from main; it calls tally.
func Count(s string) int { return tally(s) }

func tally(s string) int {
	seen := map[float64]int{}
	seen[float64(len(s))]++
	return seen[1]
}

// Fine is reached too and is inside the subset.
func Fine() int { return 1 }

// Never is not reached, so its map is no reason to reject the program.
func Never() map[int]int { return nil }
